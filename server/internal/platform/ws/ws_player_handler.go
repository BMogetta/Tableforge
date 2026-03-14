package ws

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// PlayerHandler upgrades a connection for a player's personal channel.
// Unlike Handler (room-scoped), this does not subscribe the client to any
// room — it only registers them in the per-player index so they can receive
// player-scoped events: match_found, match_cancelled, match_ready,
// queue_joined, queue_left, dm_received, dm_read, notification_received.
//
// The playerID is taken from the URL param {playerID}.
// Auth middleware is applied at the router level.
func PlayerHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			http.Error(w, "invalid player_id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		// roomID is uuid.Nil — this client is not in any room.
		// unsubscribe(uuid.Nil, c) in readPump's defer is a safe no-op for the
		// room index, and it calls unsubscribePlayer which cleans up the player index.
		c := newClient(hub, uuid.Nil, playerID, conn, false, nil)

		// Subscribe to player channel only — no room subscription.
		hub.subscribePlayer(playerID, c)

		go func() {
			c.readPump()
			// readPump defer calls hub.unsubscribe(uuid.Nil, c) which:
			// - removes c from rooms[uuid.Nil] (no-op, it was never added)
			// - calls unsubscribePlayer(playerID, c) since !spectator ✓
		}()

		go c.writePump()
	}
}
