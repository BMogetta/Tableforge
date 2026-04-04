package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/game-server/games"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/lobby"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	error_message "github.com/recess/shared/errors"
	sharedmw "github.com/recess/shared/middleware"
)

// --- Games -------------------------------------------------------------------

// gameInfo is the public representation of a registered game.
type gameInfo struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	MinPlayers int                   `json:"min_players"`
	MaxPlayers int                   `json:"max_players"`
	Settings   []engine.LobbySetting `json:"settings"`
}

// GET /api/v1/games
func handleListGames(w http.ResponseWriter, r *http.Request) {
	list := games.All()
	result := make([]gameInfo, 0, len(list))
	for _, g := range list {
		settings := engine.DefaultLobbySettings()
		if provider, ok := g.(engine.LobbySettingsProvider); ok {
			for _, s := range provider.LobbySettings() {
				settings = upsertLobbySetting(settings, s)
			}
		}
		result = append(result, gameInfo{
			ID:         g.ID(),
			Name:       g.Name(),
			MinPlayers: g.MinPlayers(),
			MaxPlayers: g.MaxPlayers(),
			Settings:   settings,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func upsertLobbySetting(settings []engine.LobbySetting, s engine.LobbySetting) []engine.LobbySetting {
	for i, existing := range settings {
		if existing.Key == s.Key {
			settings[i] = s
			return settings
		}
	}
	return append(settings, s)
}

// --- Rooms -------------------------------------------------------------------

type createRoomRequest struct {
	GameID          string `json:"game_id"`
	PlayerID        string `json:"player_id"`
	TurnTimeoutSecs *int   `json:"turn_timeout_secs,omitempty"`
}

// POST /api/v1/rooms
func handleCreateRoom(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		ownerID, err := uuid.Parse(req.PlayerID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player_id")
			return
		}
		view, err := svc.CreateRoom(r.Context(), req.GameID, ownerID, req.TurnTimeoutSecs)
		if err != nil {
			writeLobbyError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, view)
	}
}

// GET /api/v1/rooms
func handleListRooms(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rooms, err := svc.ListWaitingRooms(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list rooms")
			return
		}
		writeJSON(w, http.StatusOK, rooms)
	}
}

// GET /api/v1/rooms/{roomID}
func handleGetRoom(svc *lobby.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		view, err := svc.GetRoom(r.Context(), roomID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}

type joinRoomRequest struct {
	Code string `json:"code"`
}

// POST /api/v1/rooms/join
func handleJoinRoom(svc *lobby.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req joinRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		view, err := svc.JoinRoom(r.Context(), req.Code, playerID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}
		hub.Broadcast(view.Room.ID, ws.Event{
			Type:    ws.EventPlayerJoined,
			Payload: view,
		})
		writeJSON(w, http.StatusOK, view)
	}
}

type leaveRoomRequest struct{}

// POST /api/v1/rooms/{roomID}/leave
func handleLeaveRoom(svc *lobby.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		var req leaveRoomRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		result, err := svc.LeaveRoom(r.Context(), roomID, playerID)
		if err != nil {
			writeLobbyError(w, err)
			return
		}
		if result.RoomClosed {
			hub.Broadcast(roomID, ws.Event{Type: ws.EventRoomClosed, Payload: nil})
		} else if result.NewOwnerID != nil {
			hub.Broadcast(roomID, ws.Event{
				Type:    ws.EventOwnerChanged,
				Payload: map[string]any{"owner_id": result.NewOwnerID},
			})
		} else {
			hub.Broadcast(roomID, ws.Event{
				Type:    ws.EventPlayerLeft,
				Payload: map[string]string{"player_id": playerID.String()},
			})
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type startGameRequest struct{}

// POST /api/v1/rooms/{roomID}/start
func handleStartGame(svc *lobby.Service, rt *runtime.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		var req startGameRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		session, err := svc.StartGame(r.Context(), roomID, playerID, store.SessionModeCasual)
		if err != nil {

			slog.Error("start game failed", "room_id", roomID, "error", err) // TODO REMOVE
			writeLobbyError(w, err)
			return
		}
		rt.StartSession(r.Context(), session, hub, runtime.DefaultReadyTimeout)
		hub.Broadcast(roomID, ws.Event{
			Type:    ws.EventGameStarted,
			Payload: map[string]any{"session": session},
		})

		// If the first player to move is a registered bot, fire immediately.
		var initialState struct {
			CurrentPlayerID string `json:"current_player_id"`
		}
		if err := json.Unmarshal(session.State, &initialState); err == nil {
			if firstPlayer, err := uuid.Parse(initialState.CurrentPlayerID); err == nil {
				rt.MaybeFireBot(r.Context(), hub, session.ID, firstPlayer)
			}
		}

		writeJSON(w, http.StatusOK, session)
		activeSessions.Inc()
	}
}

type updateRoomSettingRequest struct {
	Value string `json:"value"`
}

// PUT /api/v1/rooms/{roomID}/settings/{key}
func handleUpdateRoomSetting(svc *lobby.Service, hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		key := chi.URLParam(r, "key")
		if key == "" {
			writeError(w, http.StatusBadRequest, "setting key is required")
			return
		}
		var req updateRoomSettingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		playerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}

		if err := svc.UpdateRoomSetting(r.Context(), roomID, playerID, key, req.Value); err != nil {
			writeLobbyError(w, err)
			return
		}
		hub.Broadcast(roomID, ws.Event{
			Type:    ws.EventSettingUpdated,
			Payload: map[string]any{"key": key, "value": req.Value},
		})
		w.WriteHeader(http.StatusNoContent)
	}
}
