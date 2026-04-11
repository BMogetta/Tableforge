// Package handler contains the WebSocket upgrade handlers for ws-gateway.
//
// All external data access uses gRPC:
//   - game.v1.GameService/IsParticipant — participant vs spectator check
//   - game.v1.GameService/GetRoomPlayers — presence snapshot on connect
//   - user.v1.UserService/CheckBan — ban check
package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/recess/services/ws-gateway/internal/hub"
	"github.com/recess/services/ws-gateway/internal/presence"
	"github.com/recess/shared/middleware"
	gamev1 "github.com/recess/shared/proto/game/v1"
	userv1 "github.com/recess/shared/proto/user/v1"
	sharedws "github.com/recess/shared/ws"
)

// allowedOrigins is parsed once from the ALLOWED_ORIGINS env var (comma-separated).
// When empty, all origins are accepted (dev mode).
var allowedOrigins = parseAllowedOrigins()

func parseAllowedOrigins() map[string]struct{} {
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return nil
	}
	m := make(map[string]struct{})
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			m[o] = struct{}{}
		}
	}
	return m
}

func checkOrigin(r *http.Request) bool {
	if len(allowedOrigins) == 0 {
		return true // dev mode — no restrictions
	}
	origin := r.Header.Get("Origin")
	_, ok := allowedOrigins[origin]
	return ok
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     checkOrigin,
}

// RoomHandler upgrades a WebSocket connection for a room.
// Determines participant vs spectator status via game.v1.IsParticipant gRPC.
func RoomHandler(h *hub.Hub, ps *presence.Store, uc userv1.UserServiceClient, gc gamev1.GameServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
		if err != nil {
			http.Error(w, "invalid room id", http.StatusBadRequest)
			return
		}

		playerID, ok := middleware.PlayerIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Use a detached context for pre-upgrade gRPC checks — the HTTP request
		// context is canceled when the browser closes the connection during fast
		// React re-renders, causing "context canceled" on the gRPC round-trip.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

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

		if spectator {
			h.Publish(ctx, roomID, hub.Event{
				Type:    sharedws.EventSpectatorJoined,
				Payload: map[string]any{"spectator_count": h.SpectatorCount(roomID)},
			})
		}

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
			roomPlayerIDs, _ := getRoomPlayerIDs(ctx, gc, roomID)
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
			if spectator {
				h.Publish(context.Background(), roomID, hub.Event{
					Type:    sharedws.EventSpectatorLeft,
					Payload: map[string]any{"spectator_count": h.SpectatorCount(roomID)},
				})
			}
			if !spectator && ps != nil {
				// Use background context — the HTTP request context is already
				// cancelled by the time the WS connection closes.
				h.Publish(context.Background(), roomID, hub.Event{
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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

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

// PresenceHandler returns the online/offline status for a list of player IDs.
// GET /api/v1/presence?ids=uuid1,uuid2,...
func PresenceHandler(ps *presence.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idsParam := r.URL.Query().Get("ids")
		if idsParam == "" {
			http.Error(w, "ids query param required", http.StatusBadRequest)
			return
		}

		parts := strings.Split(idsParam, ",")
		ids := make([]uuid.UUID, 0, len(parts))
		for _, s := range parts {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			http.Error(w, "no valid ids", http.StatusBadRequest)
			return
		}

		// Cap at 100 to prevent abuse.
		if len(ids) > 100 {
			ids = ids[:100]
		}

		online, err := ps.ListOnline(r.Context(), ids)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		result := make(map[string]bool, len(online))
		for id, ok := range online {
			result[id.String()] = ok
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
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

// getRoomPlayerIDs fetches room player IDs via game.v1.GetRoomPlayers gRPC.
func getRoomPlayerIDs(ctx context.Context, gc gamev1.GameServiceClient, roomID uuid.UUID) ([]uuid.UUID, error) {
	resp, err := gc.GetRoomPlayers(ctx, &gamev1.GetRoomPlayersRequest{RoomId: roomID.String()})
	if err != nil {
		slog.Error("ws-gateway: get room players gRPC failed", "room_id", roomID, "error", err)
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(resp.PlayerIds))
	for _, s := range resp.PlayerIds {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
