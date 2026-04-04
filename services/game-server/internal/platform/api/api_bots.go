package api

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/game-server/internal/bot"
	botadapter "github.com/recess/game-server/internal/bot/adapter"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/platform/ws"
	error_message "github.com/recess/shared/errors"
	sharedmw "github.com/recess/shared/middleware"
)

// GET /api/v1/bots/profiles
//
// Returns all available bot personality profiles in a stable order.
// The response is static — profiles are defined at compile time in bot.Profiles.
func handleListBotProfiles() http.HandlerFunc {
	type profileResponse struct {
		Name             string  `json:"name"`
		Iterations       int     `json:"iterations"`
		Determinizations int     `json:"determinizations"`
		ExplorationC     float64 `json:"exploration_c"`
		Aggressiveness   float64 `json:"aggressiveness"`
		RiskAversion     float64 `json:"risk_aversion"`
	}

	// Build and sort once at construction time — response never changes.
	profiles := make([]profileResponse, 0, len(bot.Profiles))
	for _, p := range bot.Profiles {
		profiles = append(profiles, profileResponse{
			Name:             p.Name,
			Iterations:       p.Iterations,
			Determinizations: p.Determinizations,
			ExplorationC:     p.ExplorationC,
			Aggressiveness:   p.Aggressiveness,
			RiskAversion:     p.RiskAversion,
		})
	}
	order := map[string]int{"easy": 0, "medium": 1, "hard": 2, "aggressive": 3}
	sort.Slice(profiles, func(i, j int) bool {
		oi, oj := order[profiles[i].Name], order[profiles[j].Name]
		if oi != oj {
			return oi < oj
		}
		return profiles[i].Name < profiles[j].Name
	})

	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, profiles)
	}
}

// POST /api/v1/rooms/{roomID}/bots
//
// Adds a fill bot to the room. The bot is registered as a real store.Player
// with is_bot = TRUE so it can be identified and removed later.
// It is also registered in the runtime registry so MaybeFireBot triggers
// its moves automatically after each human move.
// The caller (room owner) is identified via JWT context.
//
// Request body:
//
//	{ "profile": "easy|medium|hard|aggressive" }
//
// Response: the created store.Player for the bot.
func handleAddBot(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}

		var req struct {
			Profile string `json:"profile"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Profile == "" {
			req.Profile = "medium"
		}

		// Validate profile before touching the store.
		cfg, err := bot.ConfigFromProfile(req.Profile)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown bot profile: %s", req.Profile))
			return
		}

		room, err := st.GetRoom(r.Context(), roomID)
		if err != nil {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}

		// Verify the caller is the room owner.
		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}
		if room.OwnerID != callerID {
			writeError(w, http.StatusForbidden, "only the room owner can add bots")
			return
		}

		adapter, err := botadapter.New(room.GameID)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("no bot adapter for game %q", room.GameID))
			return
		}

		// Generate a unique bot username: "bot:<profile>:<8-char uuid prefix>"
		username := fmt.Sprintf("bot:%s:%s", req.Profile, uuid.New().String()[:8])

		// CreateBotPlayer sets is_bot = TRUE in the DB.
		botPlayer, err := st.CreateBotPlayer(r.Context(), username)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create bot player")
			return
		}

		// Determine the next available seat.
		roomPlayers, err := st.ListRoomPlayers(r.Context(), roomID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list room players")
			return
		}

		if err := st.AddPlayerToRoom(r.Context(), roomID, botPlayer.ID, len(roomPlayers)); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to add bot to room")
			return
		}

		bp := &bot.BotPlayer{
			ID:      botPlayer.ID,
			GameID:  room.GameID,
			Adapter: adapter,
			Config:  cfg,
			Search:  mcts.Search,
		}
		rt.RegisterBot(bp)

		// If a session is already active, check whether it is the bot's turn
		// and fire immediately if so. MaybeFireBot decodes state internally
		// and is a no-op if the bot is not the current player.
		if session, err := st.GetActiveSessionByRoom(r.Context(), roomID); err == nil {
			rt.MaybeFireBot(r.Context(), hub, session.ID, botPlayer.ID)
		}

		writeJSON(w, http.StatusCreated, botPlayer)
	}
}

// DELETE /api/v1/rooms/{roomID}/bots/{botID}
//
// Removes a fill bot from the room and unregisters it from the runtime.
// Only the room owner (identified via JWT context) can call this endpoint.
// Returns 404 if botID is not a registered bot in this room.
// No request body required.
func handleRemoveBot(rt *runtime.Service, hub *ws.Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid room id")
			return
		}
		botID, err := uuid.Parse(chi.URLParam(r, "botID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid bot id")
			return
		}

		room, err := st.GetRoom(r.Context(), roomID)
		if err != nil {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}

		// Verify the caller is the room owner.
		callerID, ok := sharedmw.PlayerIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, error_message.Unauthorized)
			return
		}
		if room.OwnerID != callerID {
			writeError(w, http.StatusForbidden, "only the room owner can remove bots")
			return
		}

		// Verify the target is actually a registered bot.
		botPlayer, err := st.GetPlayer(r.Context(), botID)
		if err != nil {
			writeError(w, http.StatusNotFound, "bot not found")
			return
		}
		if !botPlayer.IsBot {
			writeError(w, http.StatusBadRequest, "player is not a bot")
			return
		}

		// Verify the bot is in this room.
		roomPlayers, err := st.ListRoomPlayers(r.Context(), roomID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list room players")
			return
		}
		botInRoom := false
		for _, p := range roomPlayers {
			if p.PlayerID == botID {
				botInRoom = true
				break
			}
		}
		if !botInRoom {
			writeError(w, http.StatusNotFound, "bot is not in this room")
			return
		}

		rt.UnregisterBot(botID)

		if err := st.RemovePlayerFromRoom(r.Context(), roomID, botID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to remove bot from room")
			return
		}

		hub.Broadcast(roomID, ws.Event{
			Type:    ws.EventPlayerLeft,
			Payload: map[string]string{"player_id": botID.String()},
		})

		w.WriteHeader(http.StatusNoContent)
	}
}
