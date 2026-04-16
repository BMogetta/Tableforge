package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	"github.com/recess/game-server/internal/scenarios"
)

// debugScenariosEnabled returns true when the debug scenario endpoints should
// be wired into the router. Auto-enabled in TEST_MODE for parity with other
// dev affordances; otherwise must be opted into explicitly.
func debugScenariosEnabled() bool {
	if strings.EqualFold(os.Getenv("ALLOW_DEBUG_SCENARIOS"), "true") {
		return true
	}
	return strings.EqualFold(os.Getenv("TEST_MODE"), "true")
}

type loadStateRequest struct {
	State json.RawMessage `json:"state"`
}

type loadScenarioRequest struct {
	ScenarioID string `json:"scenario_id"`
}

// POST /api/v1/sessions/{sessionID}/debug/load-state
//
// Replaces the persisted GameState of an active session with the provided
// raw JSON. Lower-level primitive — usually you want load-scenario instead.
// Gated by debugScenariosEnabled() at registration time.
func handleLoadState(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		var req loadStateRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(req.State) == 0 {
			writeError(w, http.StatusBadRequest, "state is required")
			return
		}

		applyLoadedState(w, r, rt, hub, st, sessionID, req.State)
	}
}

// POST /api/v1/sessions/{sessionID}/debug/load-scenario
//
// Looks up a fixture by id under internal/scenarios/data/{gameID}/, swaps
// the __PLAYER_N__ placeholders for this session's actual player IDs (in
// seat-order), and loads the resulting state. Gated by
// debugScenariosEnabled() at registration time.
func handleLoadScenario(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		var req loadScenarioRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.ScenarioID == "" {
			writeError(w, http.StatusBadRequest, "scenario_id is required")
			return
		}

		session, err := st.GetGameSession(r.Context(), sessionID)
		if err != nil {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}

		players, err := st.ListRoomPlayers(r.Context(), session.RoomID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list players failed")
			return
		}
		ids := make([]string, len(players))
		for i, p := range players {
			ids[i] = p.PlayerID.String()
		}

		raw, err := scenarios.Resolve(session.GameID, req.ScenarioID, ids)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		applyLoadedState(w, r, rt, hub, st, sessionID, raw)
	}
}

// GET /api/v1/games/{gameID}/scenarios — lists fixtures available for a game.
func handleListScenarios(w http.ResponseWriter, r *http.Request) {
	gameID := chi.URLParam(r, "gameID")
	if gameID == "" {
		writeError(w, http.StatusBadRequest, "game id is required")
		return
	}
	list, err := scenarios.List(gameID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// applyLoadedState is the shared tail of load-state / load-scenario: persists
// the new state via the runtime, broadcasts to clients, and pokes a bot if
// the loaded state hands the turn to one.
func applyLoadedState(
	w http.ResponseWriter,
	r *http.Request,
	rt *runtime.Service,
	hub *ws.Hub,
	st store.Store,
	sessionID uuid.UUID,
	rawState []byte,
) {
	result, err := rt.LoadState(r.Context(), sessionID, rawState)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}

	players, _ := st.ListRoomPlayers(r.Context(), result.Session.RoomID)
	rt.BroadcastMove(r.Context(), hub, result, ws.EventMoveApplied, players)

	if currentPlayerID, perr := uuid.Parse(string(result.State.CurrentPlayerID)); perr == nil {
		rt.MaybeFireBot(r.Context(), hub, sessionID, currentPlayerID)
	}

	writeJSON(w, http.StatusOK, MoveAckResponse{
		MoveNumber: result.Session.MoveCount,
		IsOver:     result.IsOver,
	})
}
