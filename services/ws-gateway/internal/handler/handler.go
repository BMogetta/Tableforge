// Package handler contains the WebSocket upgrade handlers for ws-gateway.
//
// Participant and spectator checks use game.v1.GameService/IsParticipant (gRPC).
//
// Remaining direct Postgres access:
//   - listRoomPlayerIDs: reads room_players to build the initial presence snapshot.
//     Replace with game.v1.GetRoomPlayers when that RPC is added.
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
	"github.com/tableforge/shared/middleware"
	gamev1 "github.com/tableforge/shared/proto/game/v1"
	userv1 "github.com/tableforge/shared/proto/user/v1"
	sharedws "github.com/tableforge/shared/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// RoomHandler upgrades a WebSocket connection for a room.
// Determines participant vs spectator status via game.v1.IsParticipant gRPC.
func RoomHandler(h *hub.Hub, ps *presence.Store, db *pgxpool.Pool, uc userv1.UserServiceClient, gc gamev1.GameServiceClient) http.HandlerFunc {
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

		// Participant + spectator check via game-server gRPC.
		isParticipant, spectatorsAllowed, err := checkParticipant(ctx, gc, roomID, playerID)
		if err != nil {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}

		if !isParticipant && !spectatorsAllowed {
			http.Error(w, "spectators not allowed in this room", http.StatusForbidden)
			return
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
			// TODO: replace listRoomPlayerIDs with game.v1.GetRoomPlayers gRPC.
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

// checkParticipant calls game.v1.IsParticipant gRPC.
// Returns (isParticipant, spectatorsAllowed, error).
func checkParticipant(ctx context.Context, gc gamev1.GameServiceClient, roomID, playerID uuid.UUID) (bool, bool, error) {
	resp, err := gc.IsParticipant(ctx, &gamev1.IsParticipantRequest{
		RoomId:   roomID.String(),
		PlayerId: playerID.String(),
	})
	if err != nil {
		return false, false, err
	}
	return resp.IsParticipant, resp.SpectatorsAllowed, nil
}

// listRoomPlayerIDs queries room_players directly for the presence snapshot.
// TODO: replace with game.v1.GetRoomPlayers when that RPC is added.
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
