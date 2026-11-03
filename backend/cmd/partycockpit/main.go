// Command partycockpit runs the whole party: API, WebSocket hub, GPU job
// workers and the embedded Stage/Cockpit/Guest web app, in one binary.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/api"
	"github.com/lioilsources/PartyCockpit/backend/internal/config"
	"github.com/lioilsources/PartyCockpit/backend/internal/djfy"
	"github.com/lioilsources/PartyCockpit/backend/internal/gallery"
	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
	"github.com/lioilsources/PartyCockpit/backend/internal/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(cfg.DataDir, "partycockpit.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	cat, err := providers.LoadCatalog()
	if err != nil {
		return err
	}

	cf := providers.Auth{CFClientID: cfg.CFClientID, CFClientSecret: cfg.CFClientSecret}
	aistack := cf
	aistack.Extra = map[string]string{"X-Project": cfg.AiStackProject}

	var restyler providers.Restyler
	switch cfg.RestyleProvider {
	case "restyler":
		a := cf
		a.Extra = map[string]string{"Authorization": "app " + cfg.RestylerToken, "X-User": cfg.RestylerUser}
		restyler = &providers.RestylerBackend{URL: cfg.RestylerURL, Auth: a}
	case "kontext":
		restyler = &providers.Kontext{URL: cfg.AiStackURL, Auth: aistack}
	default:
		restyler = &providers.Mock{Delay: 3 * time.Second}
	}
	var classifier providers.Classifier = providers.NoClassifier{}
	if cfg.Moderation == "aistack" {
		classifier = &providers.WD14{URL: cfg.AiStackURL, Auth: aistack}
	}

	srv := &api.Server{
		Cfg: cfg, Store: st, Hub: hub.New(),
		Gallery:  gallery.New(cfg.GalleryURL, filepath.Join(cfg.DataDir, "gallery-cache"), cfg.GalleryAllowUnclassified, cf.Headers()),
		DJfy:     djfy.New(cfg.DJfyURL),
		Catalog:  cat,
		Restyler: restyler, Classifier: classifier,
		Web: web.FS(),
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := srv.Init(ctx); err != nil {
		return err
	}
	if cfg.AdminToken == "" {
		slog.Warn("PC_ADMIN_TOKEN is empty: anyone on the network can create parties")
	}

	hs := &http.Server{Addr: cfg.Addr, Handler: srv.Routes(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	slog.Info("partycockpit listening", "addr", cfg.Addr, "public", cfg.PublicURL, "restyle", restyler.Name(), "moderation", cfg.Moderation, "web", srv.Web != nil)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
