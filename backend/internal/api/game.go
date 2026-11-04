package api

// Intermission game (D17): the players of the last lottery draw steer their
// paddle on the stage from their phones. The stage runs the game; the server
// only knows who plays which slot and relays their input. Only those phones'
// input gets through, and the screen never learns a guest id.

import (
	"encoding/json"
	"math"
	"net/http"

	"github.com/lioilsources/PartyCockpit/backend/internal/hub"
	"github.com/lioilsources/PartyCockpit/backend/internal/store"
)

var gameColors = []string{"#ff3d8b", "#3de8ff", "#ffd84d", "#4dffa6", "#b07bff", "#ff8a3d", "#ffffff", "#7bb0ff"}

type game struct {
	players []string // guest id per slot
}

func isGuest(id string) func(hub.Peer) bool {
	return func(p hub.Peer) bool { return p.Role == hub.Guest && p.GuestID == id }
}

func (s *Server) setDraw(code string, ids []string) {
	s.gmu.Lock()
	s.draws[code] = ids
	s.gmu.Unlock()
}

func slotMsg(slot int) hub.Msg {
	return hub.Msg{Type: "game.slot", Data: map[string]any{"slot": slot, "color": gameColors[slot%len(gameColors)]}}
}

// gameHello tells a reconnecting player's phone its slot.
func (s *Server) gameHello(code string, p hub.Peer) []hub.Msg {
	if p.Role != hub.Guest || p.GuestID == "" {
		return nil
	}
	s.gmu.Lock()
	defer s.gmu.Unlock()
	if g := s.games[code]; g != nil {
		for i, id := range g.players {
			if id == p.GuestID {
				return []hub.Msg{slotMsg(i)}
			}
		}
	}
	return nil
}

// startGame (cockpit): the last draw plays; needs at least two of them.
func (s *Server) startGame(w http.ResponseWriter, r *http.Request, p store.Party) {
	s.gmu.Lock()
	ids := append([]string(nil), s.draws[p.Code]...)
	s.gmu.Unlock()
	if len(ids) < 2 {
		writeErr(w, 409, "nejdřív vylosuj aspoň dva hráče")
		return
	}
	type player struct {
		Slot     int    `json:"slot"`
		Nickname string `json:"nickname"`
		Color    string `json:"color"`
	}
	var players []player
	var kept []string
	for _, id := range ids {
		g, err := s.Store.Guest(r.Context(), p.Code, id)
		if err != nil {
			continue
		}
		slot := len(kept)
		kept = append(kept, id)
		players = append(players, player{slot, g.Nickname, gameColors[slot%len(gameColors)]})
	}
	if len(kept) < 2 {
		writeErr(w, 409, "nejdřív vylosuj aspoň dva hráče")
		return
	}
	params, _ := json.Marshal(map[string]any{"game": "kruh", "players": players})
	if _, err := s.Store.SetScene(r.Context(), p.Code, "game", params); err != nil {
		serverErr(w, r, err)
		return
	}
	s.gmu.Lock()
	s.games[p.Code] = &game{players: kept}
	s.gmu.Unlock()
	s.broadcastParty(r, p.Code)
	for i, id := range kept {
		s.Hub.SendTo(p.Code, slotMsg(i), isGuest(id))
	}
	writeJSON(w, 200, map[string]any{"players": players})
}

func (s *Server) stopGame(w http.ResponseWriter, r *http.Request, p store.Party) {
	s.endGame(p.Code)
	w.WriteHeader(204)
}

// endGame frees the players' phones (also on PANIC).
func (s *Server) endGame(code string) {
	s.gmu.Lock()
	g := s.games[code]
	delete(s.games, code)
	s.gmu.Unlock()
	if g == nil {
		return
	}
	for _, id := range g.players {
		s.Hub.SendTo(code, hub.Msg{Type: "game.end"}, isGuest(id))
	}
}

// onGameInput: {"type":"game.input","data":{"x":0..1}} from a player's phone
// reaches the stage as {"slot":n,"x":…}.
func (s *Server) onGameInput(code string, from hub.Peer, raw []byte) {
	if from.Role != hub.Guest || from.GuestID == "" {
		return
	}
	// 30 a second is smoother than any phone's touch events; the rest is dropped (D19)
	if !s.gameInputs.allow(from.ID, 1) {
		return
	}
	var env struct {
		Data struct {
			X float64 `json:"x"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &env) != nil || math.IsNaN(env.Data.X) {
		return
	}
	s.gmu.Lock()
	slot := -1
	if g := s.games[code]; g != nil {
		for i, id := range g.players {
			if id == from.GuestID {
				slot = i
			}
		}
	}
	s.gmu.Unlock()
	if slot < 0 {
		return
	}
	x := math.Round(math.Min(1, math.Max(0, env.Data.X))*1000) / 1000
	s.Hub.Broadcast(code, hub.Msg{Type: "game.input", Data: map[string]any{"slot": slot, "x": x}}, hub.Stage)
}
