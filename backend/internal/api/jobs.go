package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

type jobTask struct {
	job   store.Job
	party string
	req   providers.RestyleRequest
}

// submitRestyle stores the original (status "source", never on stage), creates
// the restyle asset in "processing" and queues the GPU job.
func (s *Server) submitRestyle(ctx context.Context, p store.Party, g store.Guest, raw []byte, style providers.Style, medium string) (store.Asset, error) {
	photo, w, h, err := providers.Normalise(raw)
	if err != nil {
		return store.Asset{}, fmt.Errorf("fotku se nepodařilo načíst: %w", err)
	}
	dir := s.partyDir(p.Code)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return store.Asset{}, err
	}
	srcID := store.NewID(10)
	srcFile := srcID + ".jpg"
	if err := os.WriteFile(filepath.Join(dir, srcFile), photo, 0o644); err != nil {
		return store.Asset{}, err
	}
	medium = providers.NormMedium(medium)
	meta, _ := json.Marshal(map[string]any{"style": style.ID, "styleLabel": style.Label, "medium": medium,
		"nickname": g.Nickname, "original": srcFile, "w": w, "h": h})
	if _, err := s.Store.CreateAsset(ctx, store.Asset{ID: srcID, PartyCode: p.Code, GuestID: g.ID, Kind: "upload", File: srcFile, Meta: meta, Status: "source"}); err != nil {
		return store.Asset{}, err
	}
	a, err := s.Store.CreateAsset(ctx, store.Asset{PartyCode: p.Code, GuestID: g.ID, Kind: "restyle", SrcID: srcID, Meta: meta, Status: "processing"})
	if err != nil {
		return a, err
	}
	job, err := s.Store.CreateJob(ctx, store.Job{PartyCode: p.Code, AssetID: a.ID, Intent: "restyle", Provider: s.Restyler.Name()})
	if err != nil {
		return a, err
	}
	task := jobTask{job: job, party: p.Code, req: providers.RestyleRequest{
		Photo: photo, Style: style, Prompt: s.Catalog.Prompt(style, medium), Negative: s.Catalog.NegativePrompt(medium)}}
	select {
	case s.jobs <- task:
	default:
		s.Store.UpdateJob(ctx, job.ID, "failed", "", "queue full")
		s.fallbackToOriginal(ctx, a.ID, "fronta je plná")
	}
	s.pushCockpit(ctx, p)
	return a, nil
}

func (s *Server) worker() {
	for t := range s.jobs {
		s.runJob(t)
	}
}

func (s *Server) runJob(t jobTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	s.Store.UpdateJob(ctx, t.job.ID, "running", "", "")
	start := time.Now()
	out, ext, err := s.Restyler.Restyle(ctx, t.req)
	if err != nil {
		slog.Warn("restyle failed", "job", t.job.ID, "provider", s.Restyler.Name(), "err", err)
		s.Store.UpdateJob(ctx, t.job.ID, "failed", ext, err.Error())
		s.fallbackToOriginal(ctx, t.job.AssetID, err.Error())
		return
	}
	file := t.job.AssetID + ".png"
	if detect(out) == "image/jpeg" {
		file = t.job.AssetID + ".jpg"
	}
	if err := os.WriteFile(filepath.Join(s.partyDir(t.party), file), out, 0o644); err != nil {
		s.Store.UpdateJob(ctx, t.job.ID, "failed", ext, err.Error())
		s.fallbackToOriginal(ctx, t.job.AssetID, err.Error())
		return
	}
	s.Store.UpdateJob(ctx, t.job.ID, "done", ext, "")
	slog.Info("restyle done", "job", t.job.ID, "provider", s.Restyler.Name(), "ms", time.Since(start).Milliseconds())
	s.finishAsset(ctx, t.party, t.job.AssetID, file, out, nil, true)
}

// finishAsset classifies the result and moves it to pending — or straight to the
// show queue in relaxed mode when the classifier is confident (D5).
// The original photo (fallback) never auto-approves.
func (s *Server) finishAsset(ctx context.Context, code, assetID, file string, img []byte, meta json.RawMessage, allowAuto bool) {
	score, err := s.Classifier.NSFW(ctx, img)
	if err != nil {
		slog.Warn("classifier", "asset", assetID, "err", err)
		score = -1
	}
	if err := s.Store.SetAssetResult(ctx, assetID, file, "pending", score, meta); err != nil {
		slog.Error("asset result", "asset", assetID, "err", err)
		return
	}
	p, err := s.Store.Party(ctx, code)
	if err != nil {
		return
	}
	if allowAuto && p.Mode == "relaxed" && score >= 0 && score <= s.Cfg.AutoApproveMax {
		if err := s.Store.SetAssetStatus(ctx, code, assetID, "approved"); err == nil {
			s.pushQueue(ctx, code)
		}
	}
	s.pushCockpit(ctx, p)
	s.Hub.Broadcast(code, hub.Msg{Type: "asset.updated", Data: map[string]string{"id": assetID}}, hub.Guest)
}

// fallbackToOriginal: the GPU failed, so offer the original photo for approval
// (the operator decides; a guest selfie is still better than nothing).
func (s *Server) fallbackToOriginal(ctx context.Context, assetID, reason string) {
	a, err := s.Store.AssetByID(ctx, assetID)
	if err != nil {
		return
	}
	var m map[string]any
	json.Unmarshal(a.Meta, &m)
	if m == nil {
		m = map[string]any{}
	}
	m["fallback"], m["error"] = true, reason
	meta, _ := json.Marshal(m)
	orig, _ := m["original"].(string)
	img, _ := os.ReadFile(filepath.Join(s.partyDir(a.PartyCode), orig))
	if orig == "" || img == nil {
		s.Store.SetAssetResult(ctx, a.ID, "", "failed", -1, meta)
		if p, err := s.Store.Party(ctx, a.PartyCode); err == nil {
			s.pushCockpit(ctx, p)
		}
		return
	}
	s.finishAsset(ctx, a.PartyCode, a.ID, orig, img, meta, false)
}

func detect(b []byte) string {
	if len(b) > 3 && b[0] == 0xFF && b[1] == 0xD8 {
		return "image/jpeg"
	}
	return "image/png"
}
