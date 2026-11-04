package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles"
	"github.com/lioilsources/PartyCockpit/backend/internal/subtitles/subtitlestest"
)

func TestSubtitles(t *testing.T) {
	e := newEnv(t)
	stage := e.dial("role=stage&key=" + e.key)
	stage.waitFor("party", nil)

	// without a Translator the microphone is refused, typed lines still work
	if st := e.do("POST", "/api/parties/"+e.code+"/subtitle", map[string]any{"text": "  Dort je na stole!  "}, nil); st != 204 {
		t.Fatalf("typed subtitle: %d", st)
	}
	var l subtitles.Line
	json.Unmarshal(stage.waitFor("subtitle", nil), &l)
	if l.Text != "Dort je na stole!" || !l.Done || l.ID >= 0 {
		t.Fatalf("typed line %+v", l)
	}
	micURL := "ws" + strings.TrimPrefix(e.ts.URL, "http") + "/ws/parties/" + e.code + "/mic"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, res, err := websocket.Dial(ctx, micURL+"?token="+e.token, nil); err == nil || res == nil || res.StatusCode != 409 {
		t.Fatalf("mic without translator: %v", err)
	}

	tr := subtitlestest.Fake(t, "tok")
	defer tr.Close()
	e.srv.Subtitles = &subtitles.Client{URL: tr.URL, Token: "tok"}

	if _, res, err := websocket.Dial(ctx, micURL+"?token=wrong", nil); err == nil || res == nil || res.StatusCode != 401 {
		t.Fatalf("mic with a wrong token: %v", err)
	}
	mic, _, err := websocket.Dial(ctx, micURL+"?token="+e.token, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer mic.CloseNow()
	mic.Write(ctx, websocket.MessageText, []byte(`{"type":"start","langA":"cs","langB":"en"}`))
	_, raw, err := mic.Read(ctx)
	if err != nil || !strings.Contains(string(raw), `"ready"`) {
		t.Fatalf("mic start: %s %v", raw, err)
	}
	mic.Write(ctx, websocket.MessageBinary, make([]byte, 3200))
	mic.Write(ctx, websocket.MessageText, []byte(`{"type":"end"}`))
	json.Unmarshal(stage.waitFor("subtitle", func(d json.RawMessage) bool { return strings.Contains(string(d), `"done":true`) }), &l)
	if l.Text != "Na zdraví novomanželů!" || l.Translation != "Cheers to the newlyweds!" {
		t.Fatalf("mic line %+v", l)
	}

	// PANIC drops the microphone and clears the bar
	e.do("POST", "/api/parties/"+e.code+"/panic", nil, nil)
	stage.waitFor("subtitle.clear", nil)
	if _, _, err := mic.Read(ctx); err == nil {
		t.Fatal("PANIC must close the microphone")
	}
}
