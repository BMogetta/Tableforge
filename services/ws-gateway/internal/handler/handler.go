// Package handler contains the WebSocket upgrade handlers for ws-gateway.
//
// Migration TODO:
//
//   - isParticipant() and isSpectatorsAllowed() currently read from Postgres
//     directly (public.room_players and public.room_settings). This is acceptable
//     during the migration phase while the monolith owns those tables.
//
//   - When game-service is extracted, replace these DB reads with a gRPC call:
//     game.v1.GameService/IsParticipant(room_id, player_id)
//     See shared/proto/game/v1/game.proto for the stub definition.
//
//   - session event logging (TypePlayerConnected, TypeSpectatorJoined, etc.)
//     currently happens in the monolith's ws.Handler. Once the ws package is
//     removed from the monolith, move that logic here and publish events to
//     the events Redis Stream directly.
package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tableforge/services/ws-gateway/internal/hub"
	"github.com/tableforge/services/ws-gateway/internal/presence"
	sharedws "github.com/tableforge/shared/ws"
	"github.com/tableforge/shared/middleware"
	userv1 "github.com/tableforge/shared/proto/user/v1"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// RoomHandler upgrades a WebSocket connection for a room.
// Determines participant vs spectator status and sets up presence.
func RoomHandler(h *hub.Hub, ps *presence.Store, db *pgxpool.Pool, uc userv1.UserServiceClient) http.HandlerFunc {
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

		// Ban check via user-service gRPC.
		if uc != nil {
			resp, err := uc.CheckBan(ctx, &userv1.CheckBanRequest{PlayerId: playerID.String()})
			if err != nil {
				http.Error(w, "failed to verify player status", http.StatusInternalServerError)
				return
			}
			if resp.IsBanned {
				http.Error(w, "player is banned", http.StatusForbidden)
				return
			}
		}

		// TODO: replace with gRPC call to game-service once extracted.
		// See package-level migration comment.
		isParticipant, err := checkIsParticipant(ctx, db, roomID, playerID)
		if err != nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}

		if !isParticipant {
			allowed, err := checkSpectatorsAllowed(ctx, db, roomID)
			if err != nil || !allowed {
				http.Error(w, "spectators not allowed in this room", http.StatusForbidden)
				return
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		spectator := !isParticipant
		c := hub.NewClient(h, roomID, playerID, conn, spectator, ps)
		h.SubscribeRoom(roomID, c)
		h.SubscribePlayer(playerID, c)

		if !spectator && ps != nil {
			ps.Set(ctx, playerID)

			// Publish presence_on to the room channel.
			h.Publish(ctx, roomID, hub.Event{
				Type: sharedws.EventPresenceUpdate,
				Payload: map[string]any{
					"player_id": playerID.String(),
					"online":    true,
				},
			})

			// Send presence snapshot directly to this client.
			roomPlayerIDs, _ := listRoomPlayerIDs(ctx, db, roomID)
			if onlineMap, err := ps.ListOnline(ctx, roomPlayerIDs); err == nil {
				for id, online := range onlineMap {
					snapshot, _ := json.Marshal(hub.Event{
						Type: sharedws.EventPresenceUpdate,
						Payload: map[string]any{
							"player_id": id.String(),
							"online":    online,
						},
					})
					h.SendDirect(c, snapshot)
				}
			}
		}

		go func() {
			c.ReadPump()
			if !spectator && ps != nil {
				h.Publish(ctx, roomID, hub.Event{
					Type: sharedws.EventPresenceUpdate,
					Payload: map[string]any{
						"player_id": playerID.String(),
						"online":    false,
					},
				})
			}
		}()
		go c.WritePump()
	}
}

// PlayerHandler upgrades a WebSocket connection for a player's personal channel.
// No room subscription — only player-scoped events (DMs, notifications, queue).
func PlayerHandler(h *hub.Hub, uc userv1.UserServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, ok := middleware.PlayerIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()

		// Ban check via user-service gRPC.
		if uc != nil {
			resp, err := uc.CheckBan(ctx, &userv1.CheckBanRequest{PlayerId: playerID.String()})
			if err != nil {
				http.Error(w, "failed to verify player status", http.StatusInternalServerError)
				return
			}
			if resp.IsBanned {
				http.Error(w, "player is banned", http.StatusForbidden)
				return
			}
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		c := hub.NewClient(h, uuid.Nil, playerID, conn, false, nil)
		h.SubscribePlayer(playerID, c)

		go c.ReadPump()
		go c.WritePump()
	}
}

// --- DB helpers (migration phase — replace with gRPC when game-service extracted) ---

func checkIsParticipant(ctx context.Context, db *pgxpool.Pool, roomID, playerID uuid.UUID) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM room_players WHERE room_id = $1 AND player_id = $2)`,
		roomID, playerID,
	).Scan(&exists)
	return exists, err
}

func checkSpectatorsAllowed(ctx context.Context, db *pgxpool.Pool, roomID uuid.UUID) (bool, error) {
	var value string
	err := db.QueryRow(ctx,
		`SELECT value FROM room_settings WHERE room_id = $1 AND key = 'allow_spectators'`,
		roomID,
	).Scan(&value)
	if err != nil {
		return false, err
	}
	return value == "yes", nil
}

func listRoomPlayerIDs(ctx context.Context, db *pgxpool.Pool, roomID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx,
		`SELECT player_id FROM room_players WHERE room_id = $1 ORDER BY seat`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
