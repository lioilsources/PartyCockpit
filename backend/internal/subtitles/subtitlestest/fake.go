// Package subtitlestest has a fake Translator server for tests.
package subtitlestest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
)

// Fake answers like ../Translator (docs/protocol.md): after audio and
// audio.end it sends a partial, the final transcript, a translation and the end.
func Fake(t testing.TB, token string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) })
	mux.HandleFunc("GET /v1/ws", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "no", 401)
			return
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		ctx := r.Context()
		send := func(v any) { b, _ := json.Marshal(v); c.Write(ctx, websocket.MessageText, b) }
		var langB string
		audio := 0
		id := 0
		for {
			typ, raw, err := c.Read(ctx)
			if err != nil {
				return
			}
			if typ == websocket.MessageBinary {
				audio += len(raw)
				continue
			}
			var m struct {
				Type, LangA, LangB, Mode, Side string
				TTS                            *bool
			}
			json.Unmarshal(raw, &m)
			switch m.Type {
			case "session.start":
				if m.Mode != "streaming" || m.TTS == nil || *m.TTS {
					send(map[string]string{"type": "error", "code": "bad_request", "message": "want streaming without tts"})
					return
				}
				langB = m.LangB
				send(map[string]any{"type": "session.ready", "sessionId": "s1", "protocol": 1})
			case "audio.end":
				if m.Side != "a" || audio == 0 {
					send(map[string]string{"type": "error", "code": "bad_request"})
					continue
				}
				id++
				send(map[string]any{"type": "asr.partial", "side": "a", "text": "Na zdraví"})
				send(map[string]any{"type": "asr.final", "utteranceId": id, "side": "a", "lang": "cs", "text": "Na zdraví novomanželů!", "confidence": 0.9})
				send(map[string]any{"type": "mt.final", "utteranceId": id, "side": "a", "lang": langB, "text": "Cheers to the newlyweds!"})
				send(map[string]any{"type": "utterance.end", "utteranceId": id, "status": "ok"})
				audio = 0
			}
		}
	})
	return httptest.NewServer(mux)
}
