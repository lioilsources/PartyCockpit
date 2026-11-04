package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestGuestIDsStayPrivate(t *testing.T) {
	e := newEnv(t)
	var g struct {
		ID string `json:"id"`
	}
	e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": "Ema"}, &g)
	photo := e.upload(g.ID)
	e.waitAsset(g.ID, photo.ID, "pending")
	e.do("POST", "/api/parties/"+e.code+"/assets/"+photo.ID+"/approve", nil, nil)

	stage := e.dial("role=stage&key=" + e.key)
	q := stage.waitFor("queue", func(d json.RawMessage) bool { return len(d) > 4 })
	res, _ := http.Get(e.ts.URL + "/api/parties/" + e.code + "/queue")
	pub, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.Contains(string(q), g.ID) || strings.Contains(string(pub), g.ID) {
		t.Fatal("the show queue must not carry the guest id")
	}
	if !strings.Contains(string(pub), `"guestId":"g`) {
		t.Fatalf("items still need a per-guest tag: %s", pub)
	}

	// a lottery never puts ids in the scene, and only the winner hears it won
	spectator := e.dial("role=guest")
	spectator.waitFor("party", nil)
	winner := e.dial("role=guest&guest=" + g.ID)
	winner.waitFor("party", nil)
	e.do("POST", "/api/parties/"+e.code+"/lottery", map[string]any{"count": 1, "game": "Kruh"}, nil)
	scene := spectator.waitFor("party", func(d json.RawMessage) bool { return strings.Contains(string(d), `"lottery"`) })
	if strings.Contains(string(scene), g.ID) {
		t.Fatalf("lottery scene leaks the guest id: %s", scene)
	}
	winner.waitFor("lottery.won", nil)
	spectator.none("lottery.won", 200*time.Millisecond)
}

func TestIntermissionGame(t *testing.T) {
	e := newEnv(t)
	ids := []string{}
	for _, n := range []string{"A", "B", "C"} {
		var g struct {
			ID string `json:"id"`
		}
		e.do("POST", "/api/parties/"+e.code+"/guests", map[string]any{"nickname": n}, &g)
		ids = append(ids, g.ID)
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/game", nil, nil); st != 409 {
		t.Fatalf("a game without a draw: %d, want 409", st)
	}
	stage := e.dial("role=stage&key=" + e.key)
	stage.waitFor("party", nil)
	phones := map[string]*sock{}
	for _, id := range ids {
		phones[id] = e.dial("role=guest&guest=" + id)
		phones[id].waitFor("party", nil)
	}
	e.do("POST", "/api/parties/"+e.code+"/lottery", map[string]any{"count": 2}, nil)
	var out struct {
		Players []struct {
			Slot     int
			Nickname string
		}
	}
	if st := e.do("POST", "/api/parties/"+e.code+"/game", nil, &out); st != 200 || len(out.Players) != 2 {
		t.Fatalf("start: %d %+v", st, out)
	}
	scene := stage.waitFor("party", func(d json.RawMessage) bool { return strings.Contains(string(d), `"kind":"game"`) })
	for _, id := range ids {
		if strings.Contains(string(scene), id) {
			t.Fatal("game scene leaks a guest id")
		}
	}

	// the two players get a slot; their input reaches the stage, the third phone's doesn't
	gotSlot := func(s *sock) bool {
		timeout := time.After(300 * time.Millisecond)
		for {
			select {
			case m := <-s.in:
				if m.Type == "game.slot" {
					return true
				}
			case <-timeout:
				return false
			}
		}
	}
	var players, others []string
	for _, id := range ids {
		if gotSlot(phones[id]) {
			players = append(players, id)
		} else {
			others = append(others, id)
		}
	}
	if len(players) != 2 || len(others) != 1 {
		t.Fatalf("players %v others %v", players, others)
	}
	phones[others[0]].send(map[string]any{"type": "game.input", "data": map[string]any{"x": 0.1}})
	phones[players[0]].send(map[string]any{"type": "game.input", "data": map[string]any{"x": 7}})
	in := stage.waitFor("game.input", nil)
	var gi struct {
		Slot int
		X    float64
	}
	json.Unmarshal(in, &gi)
	if gi.X != 1 {
		t.Fatalf("input must be clamped and come from the player only: %s", in)
	}
	stage.none("game.input", 150*time.Millisecond)

	// PANIC frees the phones
	e.do("POST", "/api/parties/"+e.code+"/panic", nil, nil)
	phones[players[1]].waitFor("game.end", nil)
	phones[players[0]].send(map[string]any{"type": "game.input", "data": map[string]any{"x": 0.5}})
	stage.none("game.input", 150*time.Millisecond)
}
