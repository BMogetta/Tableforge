package ws

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tableforge/server/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now — tighten in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler returns an http.HandlerFunc that upgrades connections for roomID.
//
// The player_id query parameter is used to determine whether the connecting
// client is a participant or a spectator:
//   - If player_id is in room_players → player connection, always allowed.
//   - If player_id is NOT in room_players → spectator connection.
//     Rejected with 403 if the room's allow_spectators setting is "no".
//
// When a spectator connects or disconnects, a spectator_joined / spectator_left
// event is broadcast to all clients in the room so the lobby can update the
// live spectator count.
func Handler(hub *Hub, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			http.Error(w, "invalid room id", http.StatusBadRequest)
			return
		}

		playerIDStr := r.URL.Query().Get("player_id")
		playerID, err := uuid.Parse(playerIDStr)
		if err != nil {
			http.Error(w, "player_id query param required", http.StatusBadRequest)
			return
		}

		ctx := r.Context()

		// Determine if this player is a room participant.
		roomPlayers, err := st.ListRoomPlayers(ctx, roomID)
		if err != nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}

		isParticipant := false
		for _, rp := range roomPlayers {
			if rp.PlayerID == playerID {
				isParticipant = true
				break
			}
		}

		// Non-participants are spectators — check if the room allows them.
		if !isParticipant {
			settings, err := st.GetRoomSettings(ctx, roomID)
			if err != nil {
				http.Error(w, "room not found", http.StatusNotFound)
				return
			}
			if settings["allow_spectators"] != "yes" {
				http.Error(w, "spectators not allowed in this room", http.StatusForbidden)
				return
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			// Upgrade writes the error response itself.
			return
		}

		spectator := !isParticipant
		c := newClient(hub, roomID, conn, spectator)
		hub.subscribe(roomID, c)

		if spectator {
			hub.Broadcast(roomID, Event{
				Type:    EventSpectatorJoined,
				Payload: map[string]any{"spectator_count": hub.SpectatorCount(roomID)},
			})

			// Notify remaining clients when the spectator disconnects.
			// We wrap readPump in a goroutine that broadcasts on exit.
			go func() {
				c.readPump()
				hub.Broadcast(roomID, Event{
					Type:    EventSpectatorLeft,
					Payload: map[string]any{"spectator_count": hub.SpectatorCount(roomID)},
				})
			}()
		} else {
			go c.readPump()
		}

		go c.writePump()
	}
}
