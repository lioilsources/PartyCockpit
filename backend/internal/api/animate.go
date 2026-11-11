package api

// M4: living pictures and guest dances (DECISIONS.md D13).
//
//	live  — LivePortrait loop of an approved photo. It only moves the face of a
//	        photo the operator already approved, so it inherits that approval and
//	        plays in place of the still (stage_queue keeps the photo, Asset.Live
//	        carries the clip).
//	dance — Wan image-to-video of a guest's own photo. Generative, so it always
//	        waits for the operator and then joins the show queue like a photo.
//	breeze — Wan hair-in-the-breeze of an approved photo, ordered by the
//	        operator (D23). Generative, so it waits for the operator like a
//	        dance, then plays in place of the photo like a living picture.
//
// Animations get their own worker so a 1–3 min dance never delays restyles.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

type animTask struct {
	job    store.Job
	party  string
	motion providers.Motion
	image  []byte
}

func (s *Server) animWorker() {
	for t := range s.anims {
		s.runAnim(t)
	}
}

// submitAnimation creates the live/dance asset for src and queues the GPU job.
func (s *Server) submitAnimation(ctx context.Context, p store.Party, src store.Asset, m providers.Motion) (store.Asset, error) {
	if src.File == "" {
		return store.Asset{}, errors.New("fotka ještě není hotová")
	}
	img, err := os.ReadFile(filepath.Join(s.partyDir(p.Code), src.File))
	if err != nil {
		return store.Asset{}, err
	}
	var srcMeta map[string]any
	json.Unmarshal(src.Meta, &srcMeta)
	kind := "dance"
	if m.Engine == "live" || m.Engine == "breeze" {
		kind = m.Engine
	}
	meta, _ := json.Marshal(map[string]any{
		"motion": m.ID, "motionLabel": m.Label, "nickname": srcMeta["nickname"],
		"styleLabel": srcMeta["styleLabel"], "photo": src.File,
	})
	a, err := s.Store.CreateAsset(ctx, store.Asset{PartyCode: p.Code, GuestID: src.GuestID, Kind: kind, SrcID: src.ID, Meta: meta, Status: "processing"})
	if err != nil {
		return a, err
	}
	job, err := s.Store.CreateJob(ctx, store.Job{PartyCode: p.Code, AssetID: a.ID, Intent: "animate:" + m.ID, Provider: s.Animator.Name()})
	if err != nil {
		return a, err
	}
	select {
	case s.anims <- animTask{job: job, party: p.Code, motion: m, image: img}:
	default:
		s.Store.UpdateJob(ctx, job.ID, "failed", "", "queue full")
		s.failAnimation(ctx, a.ID, "fronta animací je plná")
	}
	s.pushCockpit(ctx, p)
	return a, nil
}

func (s *Server) runAnim(t animTask) {
	timeout := 5 * time.Minute
	if t.motion.Engine != "live" {
		timeout = 20 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	s.Store.UpdateJob(ctx, t.job.ID, "running", "", "")
	start := time.Now()
	clip, ext, extID, err := s.Animator.Animate(ctx, providers.AnimateRequest{Image: t.image, Motion: t.motion})
	if err == nil && len(clip) == 0 {
		err = errors.New("empty clip")
	}
	file := t.job.AssetID + "." + ext
	if err == nil {
		err = os.WriteFile(filepath.Join(s.partyDir(t.party), file), clip, 0o644)
	}
	if err != nil {
		slog.Warn("animation failed", "job", t.job.ID, "motion", t.motion.ID, "provider", s.Animator.Name(), "err", err)
		s.Store.UpdateJob(ctx, t.job.ID, "failed", extID, err.Error())
		s.failAnimation(ctx, t.job.AssetID, err.Error())
		return
	}
	s.Store.UpdateJob(ctx, t.job.ID, "done", extID, "")
	slog.Info("animation done", "job", t.job.ID, "motion", t.motion.ID, "provider", s.Animator.Name(), "ms", time.Since(start).Milliseconds())

	a, err := s.Store.AssetByID(ctx, t.job.AssetID)
	if err != nil {
		return
	}
	status := "pending"
	if a.Kind == "live" {
		if src, err := s.Store.AssetByID(ctx, a.SrcID); err == nil && src.Status == "approved" {
			status = "approved" // inherits the photo's approval (D13)
		}
	}
	if err := s.Store.SetAssetResult(ctx, a.ID, file, status, -1, nil); err != nil {
		slog.Error("animation result", "asset", a.ID, "err", err)
		return
	}
	s.afterAssetChange(ctx, t.party, a.ID)
}

func (s *Server) failAnimation(ctx context.Context, assetID, reason string) {
	a, err := s.Store.AssetByID(ctx, assetID)
	if err != nil {
		return
	}
	var m map[string]any
	json.Unmarshal(a.Meta, &m)
	if m == nil {
		m = map[string]any{}
	}
	m["error"] = reason
	meta, _ := json.Marshal(m)
	s.Store.SetAssetResult(ctx, a.ID, "", "failed", -1, meta)
	s.afterAssetChange(ctx, a.PartyCode, a.ID)
}

func (s *Server) afterAssetChange(ctx context.Context, code, assetID string) {
	s.pushQueue(ctx, code)
	if p, err := s.Store.Party(ctx, code); err == nil {
		s.pushCockpit(ctx, p)
	}
	s.Hub.Broadcast(code, hub.Msg{Type: "asset.updated", Data: map[string]string{"id": assetID}}, hub.Guest)
}

// onApproved runs after a photo is approved: with auto-live on, its living
// loop is ordered right away (once per photo).
func (s *Server) onApproved(ctx context.Context, p store.Party, assetID string) {
	if !p.AutoLive {
		return
	}
	a, err := s.Store.Asset(ctx, p.Code, assetID)
	if err != nil || a.Kind != "restyle" {
		return
	}
	if _, err := s.Store.AnimationOf(ctx, a.ID, "live"); err == nil {
		return
	}
	m, _ := providers.MotionByID("alive")
	if _, err := s.submitAnimation(ctx, p, a, m); err != nil {
		slog.Warn("auto live", "asset", a.ID, "err", err)
	}
}

// ── HTTP ───────────────────────────────────────────────────────────────────

func (s *Server) listMotions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, providers.Motions)
}

// animateGallery (cockpit) orders living loops ("alive", default) or hair in
// the breeze ("breeze") for the given photos, or for every photo in the show
// queue that has none yet.
func (s *Server) animateGallery(w http.ResponseWriter, r *http.Request, p store.Party) {
	var in struct {
		IDs    []string `json:"ids"`
		Motion string   `json:"motion"`
	}
	_ = readJSON(r, &in)
	if in.Motion == "" {
		in.Motion = "alive"
	}
	m, ok := providers.MotionByID(in.Motion)
	if !ok || (m.Engine != "live" && m.Engine != "breeze") {
		writeErr(w, 400, "neznámý pohyb")
		return
	}
	ctx := r.Context()
	var photos []store.Asset
	if len(in.IDs) > 0 {
		for _, id := range in.IDs {
			if a, err := s.Store.Asset(ctx, p.Code, id); err == nil {
				photos = append(photos, a)
			}
		}
	} else {
		q, err := s.Store.StageQueue(ctx, p.Code)
		if err != nil {
			serverErr(w, r, err)
			return
		}
		photos = q
	}
	n := 0
	for _, a := range photos {
		if a.Kind != "restyle" || a.File == "" {
			continue
		}
		if _, err := s.Store.AnimationOf(ctx, a.ID, m.Engine); err == nil {
			continue
		}
		if _, err := s.submitAnimation(ctx, p, a, m); err != nil {
			serverErr(w, r, err)
			return
		}
		n++
	}
	writeJSON(w, 200, map[string]int{"queued": n})
}

// dance (guest): one of my finished photos + a dance → a clip for the operator.
func (s *Server) dance(w http.ResponseWriter, r *http.Request, p store.Party, g store.Guest) {
	var in struct {
		AssetID string `json:"assetId"`
		Move    string `json:"move"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	m, ok := providers.MotionByID(in.Move)
	if !ok || m.Engine != "dance" {
		writeErr(w, 400, "neznámý tanec")
		return
	}
	ctx := r.Context()
	src, err := s.Store.Asset(ctx, p.Code, in.AssetID)
	if err != nil || src.GuestID != g.ID || src.Kind != "restyle" || src.File == "" {
		writeErr(w, 404, "tuhle fotku nemáš")
		return
	}
	if busy, _ := s.Store.GuestBusy(ctx, p.Code, g.ID); busy {
		writeErr(w, 429, "něco se ti ještě maluje, chvilku počkej")
		return
	}
	a, err := s.submitAnimation(ctx, p, src, m)
	if err != nil {
		writeErr(w, 500, fmt.Sprint(err))
		return
	}
	writeJSON(w, 202, a)
}
