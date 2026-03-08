package ws

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tableforge/server/internal/events"
	"github.com/tableforge/server/internal/presence"
	"github.com/tableforge/server/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for now — tighten in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

const (
	EventPresenceUpdate EventType = "presence_update"
)

// Handler returns an http.HandlerFunc that upgrades connections for roomID.
//
// The player_id query parameter is used to determine whether the connecting
// client is a participant or a spectator:
//   - If player_id is in room_players → player connection, always allowed.
//   - If player_id is NOT in room_players → spectator connection.
//     Rejected with 403 if the room's allow_spectators setting is "no".
//
// When a player or spectator connects or disconnects, the event is appended
// to the session's Redis Stream for event sourcing.
func Handler(hub *Hub, st store.Store, ev *events.Store, ps *presence.Store) http.HandlerFunc {
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
			return
		}

		spectator := !isParticipant
		c := newClient(hub, roomID, playerID, conn, spectator, ps)
		hub.subscribe(roomID, c)

		if !spectator && ps != nil {
			// Mark the player as online and broadcast to all clients in the room
			// so existing participants see the presence dot update immediately.
			ps.Set(ctx, playerID)
			hub.Broadcast(roomID, Event{
				Type: EventPresenceUpdate,
				Payload: map[string]any{
					"player_id": playerID.String(),
					"online":    true,
				},
			})

			// Send a presence snapshot directly to the connecting client only.
			// This catches up the new client on who is already online — without
			// this, they would miss presence_update events that fired before they
			// connected and see everyone as offline until the next state change.
			playerIDs := make([]uuid.UUID, 0, len(roomPlayers))
			for _, rp := range roomPlayers {
				playerIDs = append(playerIDs, rp.PlayerID)
			}
			if onlineMap, err := ps.ListOnline(ctx, playerIDs); err == nil {
				for id, online := range onlineMap {
					snapshot, _ := json.Marshal(Event{
						Type: EventPresenceUpdate,
						Payload: map[string]any{
							"player_id": id.String(),
							"online":    online,
						},
					})
					select {
					case c.send <- snapshot:
					default:
					}
				}
			}
		}

		session, sessionErr := st.GetActiveSessionByRoom(ctx, roomID)

		if spectator {
			if sessionErr == nil {
				ev.Append(ctx, session.ID, events.TypeSpectatorJoined, &playerID, map[string]any{
					"spectator_count": hub.SpectatorCount(roomID),
				})
			}
			hub.Broadcast(roomID, Event{
				Type:    EventSpectatorJoined,
				Payload: map[string]any{"spectator_count": hub.SpectatorCount(roomID)},
			})

			go func() {
				c.readPump()
				// Spectator disconnected.
				if sessionErr == nil {
					ev.Append(ctx, session.ID, events.TypeSpectatorLeft, &playerID, map[string]any{
						"spectator_count": hub.SpectatorCount(roomID),
					})
				}
				hub.Broadcast(roomID, Event{
					Type:    EventSpectatorLeft,
					Payload: map[string]any{"spectator_count": hub.SpectatorCount(roomID)},
				})
			}()
		} else {
			if sessionErr == nil {
				ev.Append(ctx, session.ID, events.TypePlayerConnected, &playerID, map[string]any{
					"player_id": playerID.String(),
				})
			}

			go func() {
				c.readPump()
				// presence.Del is called inside readPump defer.
				if ps != nil {
					hub.Broadcast(roomID, Event{
						Type: EventPresenceUpdate,
						Payload: map[string]any{
							"player_id": playerID.String(),
							"online":    false,
						},
					})
				}
				if sessionErr == nil {
					ev.Append(ctx, session.ID, events.TypePlayerDisconnected, &playerID, map[string]any{
						"player_id": playerID.String(),
					})
				}
			}()
		}

		go c.writePump()
	}
}
