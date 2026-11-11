package api

// M7 preflight: the evening runbook as live checks in the Cockpit (DECISIONS D14).
// Each check says what is wrong and what to do; the manual items (audio cable,
// tablet charged, GPU window) live in the Cockpit next to these.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/providers"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

type check struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"` // ok | warn | fail | info
	Detail string `json:"detail"`
}

func (s *Server) preflight(w http.ResponseWriter, r *http.Request, p store.Party) {
	ctx := r.Context()
	var out []check
	add := func(id, label, status, detail string) {
		out = append(out, check{id, label, status, detail})
	}

	// network: what the QR on the TV points at
	join := s.joinURL(r, p.Code)
	u, _ := url.Parse(join)
	switch host := u.Hostname(); {
	case host == "localhost" || host == "127.0.0.1" || host == "::1":
		add("join", "Adresa v QR", "fail", join+" — telefony se nepřipojí. Nastav PC_PUBLIC_URL na LAN adresu, nebo otevři Cockpit přes ni.")
	default:
		add("join", "Adresa v QR", "ok", join)
	}
	if u.Scheme == "https" || s.Cfg.TLSCert != "" {
		add("https", "HTTPS", "ok", "kamera v telefonu (živé streamy) poběží")
	} else {
		add("https", "HTTPS", "warn", "bez https telefony nespustí kameru, živé streamy půjdou jen z MirrorBooth. Nastav PC_TLS_CERT/PC_TLS_KEY.")
	}

	counts := s.Hub.Counts(p.Code)
	if counts[hub.Stage] > 0 {
		add("stage", "Plátno", "ok", fmt.Sprintf("připojeno (%d)", counts[hub.Stage]))
	} else {
		add("stage", "Plátno", "fail", "žádné plátno není připojené. Na TV ho otevři tlačítkem „Otevřít plátno“ (odkaz nese klíč plátna).")
	}

	// GPU providers, probed in parallel (each at most a few seconds)
	type probe struct {
		id, label, name string
		ping            providers.Pinger
	}
	probes := []probe{{id: "restyle", label: "Restyle fotek", name: s.Restyler.Name()}, {id: "animate", label: "Animace (živé portréty, tance)", name: s.Animator.Name()}}
	if pg, ok := s.Restyler.(providers.Pinger); ok {
		probes[0].ping = pg
	}
	if pg, ok := s.Animator.(providers.Pinger); ok {
		probes[1].ping = pg
	}
	errs := make([]error, len(probes))
	var wg sync.WaitGroup
	for i, pr := range probes {
		if pr.ping == nil {
			continue
		}
		wg.Go(func() {
			pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			errs[i] = pr.ping.Ping(pctx)
		})
	}
	wg.Wait()
	for i, pr := range probes {
		switch {
		case pr.name == "mock":
			add(pr.id, pr.label, "warn", "mock — bez GPU, jen náhradní efekt. Pro party nastav "+map[string]string{"restyle": "RESTYLE_PROVIDER", "animate": "ANIMATE_PROVIDER"}[pr.id]+".")
		case pr.ping == nil:
			add(pr.id, pr.label, "info", pr.name+" — dostupnost ověří první úloha")
		case errs[i] != nil:
			hint := ""
			if pr.name == "comfy" {
				hint = " ComfyUI na L40S běží jen v domluveném okně (profil D) a přes VPN + tunel."
			}
			add(pr.id, pr.label, "fail", pr.name+" neodpovídá: "+errs[i].Error()+"."+hint)
		default:
			add(pr.id, pr.label, "ok", pr.name+" odpovídá")
		}
	}
	if jobs, err := s.Store.RecentJobs(ctx, p.Code, "restyle", 10); err == nil && len(jobs) >= 3 {
		failed := 0
		for _, j := range jobs {
			if j.Status == "failed" {
				failed++
			}
		}
		if failed*2 >= len(jobs) {
			add("restyle-jobs", "Poslední restyly", "fail", fmt.Sprintf("%d z %d posledních selhalo, hosté vidí originál ke schválení", failed, len(jobs)))
		}
	}

	// subtitles
	if s.Subtitles == nil {
		add("subtitles", "Titulky", "info", "bez Translatoru: jen ručně psané titulky z Cockpitu (TRANSLATOR_URL)")
	} else {
		pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := s.Subtitles.Health(pctx)
		cancel()
		if err != nil {
			add("subtitles", "Titulky", "warn", "Translator neodpovídá: "+err.Error())
		} else {
			add("subtitles", "Titulky", "ok", "Translator odpovídá")
		}
	}

	// music and beat
	switch {
	case p.DJfyCode == "":
		add("djfy", "DJfy", "warn", "není propojené: hosté nenavrhují hudbu, tempo jen z tapu nebo mikrofonu plátna")
	case s.DJfy.Snapshot(p.Code) == nil:
		add("djfy", "DJfy", "warn", "kód "+p.DJfyCode+" zadán, ale hostitelský telefon DJfy zatím nic neposlal")
	default:
		add("djfy", "DJfy", "ok", "propojeno ("+p.DJfyCode+")")
	}
	if t := s.currentTempo(p.Code); t.BPM > 0 {
		add("tempo", "Tempo", "ok", fmt.Sprintf("%.0f bpm (%s)", t.BPM, t.Source))
	} else {
		add("tempo", "Tempo", "info", "server tempo nezná: ťukni tap, nebo otevři plátno s ?audio=1 (beat z mikrofonu)")
	}

	// moderation and the queue
	if s.Cfg.Moderation == "none" {
		add("moderation", "Moderace", "info", "bez klasifikátoru: každou fotku schvaluješ ručně")
	} else {
		add("moderation", "Moderace", "ok", s.Cfg.Moderation+", režim "+p.Mode)
	}
	if pending, err := s.Store.Assets(ctx, p.Code, "pending"); err == nil && len(pending) > 0 {
		add("pending", "Ke schválení", "warn", fmt.Sprintf("%d čeká", len(pending)))
	}

	// content on disk (D23): what plays without the uplink
	if n := len(s.Webtoon.List()); n > 0 {
		add("webtoon", "Webtoon", "info", fmt.Sprintf("%d %s na disku", n, czPlural(n, "epizoda", "epizody", "epizod")))
	} else {
		add("webtoon", "Webtoon", "info", "žádná epizoda na disku; stáhni balík ve Studiu (Příprava → Webtoon knihovna)")
	}

	// party box (D20): certificate and temperature
	if s.Cfg.Box {
		if na, err := certNotAfter(s.Cfg.TLSCert); err == nil {
			days := int(time.Until(na).Hours() / 24)
			switch {
			case days < 0:
				add("cert", "Certifikát", "fail", "vypršel, kamera a mikrofon v telefonech nepojedou. Obnov doma: sudo /opt/partycockpit/bin/renew-cert.sh")
			case days < 14:
				add("cert", "Certifikát", "warn", fmt.Sprintf("vyprší za %d dní, obnov ho, dokud je box na internetu", days))
			default:
				add("cert", "Certifikát", "ok", fmt.Sprintf("platí ještě %d dní", days))
			}
		}
		if t, ok := cpuTemp(); ok {
			switch {
			case t >= 80:
				add("temp", "Teplota boxu", "fail", fmt.Sprintf("%.0f °C, box zpomaluje: zkontroluj chladič a větrání", t))
			case t >= 75:
				add("temp", "Teplota boxu", "warn", fmt.Sprintf("%.0f °C", t))
			default:
				add("temp", "Teplota boxu", "ok", fmt.Sprintf("%.0f °C", t))
			}
		}
	}

	// box health
	if free := freeBytes(s.Cfg.DataDir); free >= 0 {
		gb := float64(free) / (1 << 30)
		switch {
		case gb < 0.5:
			add("disk", "Disk", "fail", fmt.Sprintf("volno %.1f GB, fotky a klipy se nevejdou", gb))
		case gb < 2:
			add("disk", "Disk", "warn", fmt.Sprintf("volno %.1f GB", gb))
		default:
			add("disk", "Disk", "ok", fmt.Sprintf("volno %.0f GB", gb))
		}
	}
	left := time.Duration(p.EndsAt-s.Store.Now().UnixMilli()) * time.Millisecond
	switch {
	case left <= 0:
		add("ends", "Konec party", "fail", "party už skončila, hosté se nepřipojí")
	case left < 2*time.Hour:
		add("ends", "Konec party", "warn", "končí za "+strings.TrimSuffix(left.Round(time.Minute).String(), "0s"))
	default:
		add("ends", "Konec party", "ok", "končí za "+strings.TrimSuffix(left.Round(time.Minute).String(), "0s"))
	}

	writeJSON(w, 200, map[string]any{"checks": out, "at": s.Store.Now().UnixMilli()})
}
