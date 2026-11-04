// Package config reads the whole runtime configuration from the environment.
// Nothing secret has a default: the repo is public (DECISIONS.md D8).
package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr      string // PC_ADDR, default ":8098" (8097 is taken by djfy-relay)
	PublicURL string // PC_PUBLIC_URL, LAN URL printed into the QR; empty = from the request Host
	DataDir   string // PC_DATA_DIR
	// AdminToken guards POST /api/parties. Empty = anyone on the LAN may create a party.
	AdminToken string
	PartyTTL   time.Duration // PC_PARTY_TTL, how long a party lives; files are purged 7 days after
	Workers    int           // PC_WORKERS, concurrent GPU jobs

	// Optional HTTPS listener. Phones need a secure origin for the camera
	// (browser streaming); everything else works over plain LAN http.
	TLSAddr string // PC_TLS_ADDR, default ":8443"
	TLSCert string // PC_TLS_CERT, PEM file; empty = no HTTPS
	TLSKey  string // PC_TLS_KEY
	// ICEServers is the RTCPeerConnection iceServers JSON (PC_ICE_SERVERS); default [].
	ICEServers json.RawMessage

	// FineTuneGallery (slideshow source).
	GalleryURL               string // FINETUNE_URL
	GalleryAllowUnclassified bool   // FINETUNE_ALLOW_UNCLASSIFIED: show images without a taxonomy band

	// DJfy relay (now playing + shared queue).
	DJfyURL string // DJFY_URL

	// Restyle provider: "restyler" (Restyler backend, restyle_flux), "kontext" (AiStack
	// flux-kontext NIM) or "mock" (local colour filter, no GPU).
	RestyleProvider string // RESTYLE_PROVIDER
	RestylerURL     string // RESTYLER_API_URL
	RestylerToken   string // RESTYLER_APP_TOKEN
	RestylerUser    string // RESTYLER_USER
	AiStackURL      string // AISTACK_URL
	AiStackProject  string // AISTACK_PROJECT
	CFClientID      string // CF_ACCESS_CLIENT_ID (shared by Restyler and AiStack hosts)
	CFClientSecret  string // CF_ACCESS_CLIENT_SECRET

	// Animation provider (M4): "comfy" (ComfyUI host, e.g. the L40S via tunnel),
	// "restyler" (Restyler live_portrait; no dances) or "mock" (animated GIF).
	AnimateProvider string // ANIMATE_PROVIDER
	ComfyURL        string // COMFY_URL
	DriverDir       string // PC_DRIVER_DIR, LivePortrait driver clips <motion>.mp4 to upload
	AnimWorkers     int    // PC_ANIM_WORKERS

	// Live subtitles (M7): the Translator server (../Translator). Empty = only
	// typed subtitles from the Cockpit.
	TranslatorURL   string // TRANSLATOR_URL
	TranslatorToken string // TRANSLATOR_TOKEN

	// Moderation classifier: "none" (everything waits for the operator) or "aistack" (WD14 tagger).
	Moderation     string  // MODERATION
	AutoApproveMax float64 // SFW_AUTO_APPROVE_MAX, NSFW score at or below which relaxed mode auto-approves
}

func Load() Config {
	return Config{
		Addr:                     env("PC_ADDR", ":8098"),
		PublicURL:                strings.TrimRight(env("PC_PUBLIC_URL", ""), "/"),
		DataDir:                  env("PC_DATA_DIR", "./data"),
		AdminToken:               env("PC_ADMIN_TOKEN", ""),
		PartyTTL:                 dur("PC_PARTY_TTL", 24*time.Hour),
		Workers:                  num("PC_WORKERS", 2),
		TLSAddr:                  env("PC_TLS_ADDR", ":8443"),
		TLSCert:                  env("PC_TLS_CERT", ""),
		TLSKey:                   env("PC_TLS_KEY", ""),
		ICEServers:               iceServers(env("PC_ICE_SERVERS", "[]")),
		GalleryURL:               strings.TrimRight(env("FINETUNE_URL", "https://finetune.ol1n.com"), "/"),
		GalleryAllowUnclassified: env("FINETUNE_ALLOW_UNCLASSIFIED", "") == "1",
		DJfyURL:                  strings.TrimRight(env("DJFY_URL", "https://djfy.ol1n.com"), "/"),
		RestyleProvider:          env("RESTYLE_PROVIDER", "mock"),
		RestylerURL:              strings.TrimRight(env("RESTYLER_API_URL", "https://restyler-api.ol1n.com"), "/"),
		RestylerToken:            env("RESTYLER_APP_TOKEN", ""),
		RestylerUser:             env("RESTYLER_USER", "partycockpit"),
		AiStackURL:               strings.TrimRight(env("AISTACK_URL", "https://llm.ol1n.com"), "/"),
		AiStackProject:           env("AISTACK_PROJECT", "partycockpit"),
		CFClientID:               env("CF_ACCESS_CLIENT_ID", ""),
		CFClientSecret:           env("CF_ACCESS_CLIENT_SECRET", ""),
		AnimateProvider:          env("ANIMATE_PROVIDER", "mock"),
		ComfyURL:                 strings.TrimRight(env("COMFY_URL", "http://127.0.0.1:8188"), "/"),
		DriverDir:                env("PC_DRIVER_DIR", ""),
		AnimWorkers:              num("PC_ANIM_WORKERS", 1),
		TranslatorURL:            strings.TrimRight(env("TRANSLATOR_URL", ""), "/"),
		TranslatorToken:          env("TRANSLATOR_TOKEN", ""),
		Moderation:               env("MODERATION", "none"),
		AutoApproveMax:           flt("SFW_AUTO_APPROVE_MAX", 0.15),
	}
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func num(k string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(k)); err == nil && n > 0 {
		return n
	}
	return def
}

func flt(k string, def float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return f
	}
	return def
}

func dur(k string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(os.Getenv(k)); err == nil && d > 0 {
		return d
	}
	return def
}

func iceServers(v string) json.RawMessage {
	var list []map[string]any
	if err := json.Unmarshal([]byte(v), &list); err != nil {
		slog.Warn("PC_ICE_SERVERS is not a JSON array, ignoring", "err", err)
		return json.RawMessage("[]")
	}
	b, _ := json.Marshal(list)
	return b
}
