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

type loadScenarioRequest struct {
	State json.RawMessage `json:"state"`
}

// POST /api/v1/sessions/{sessionID}/debug/load-state
//
// Replaces the persisted GameState of an active session with the provided
// raw JSON. Intended for reproducing specific game scenarios in dev/test
// without playing through to the desired state. Gated by
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
		if len(req.State) == 0 {
			writeError(w, http.StatusBadRequest, "state is required")
			return
		}

		result, err := rt.LoadState(r.Context(), sessionID, req.State)
		if err != nil {
			writeRuntimeError(w, err)
			return
		}

		players, _ := st.ListRoomPlayers(r.Context(), result.Session.RoomID)
		rt.BroadcastMove(r.Context(), hub, result, ws.EventMoveApplied, players)

		// If the loaded state hands the turn to a bot, kick it off so the
		// session doesn't sit idle waiting for a player.
		if currentPlayerID, perr := uuid.Parse(string(result.State.CurrentPlayerID)); perr == nil {
			rt.MaybeFireBot(r.Context(), hub, sessionID, currentPlayerID)
		}

		writeJSON(w, http.StatusOK, MoveResponse{
			Session: sessionToDTO(result.Session),
			State:   result.State,
			IsOver:  result.IsOver,
			Result:  result.Result,
		})
	}
}
