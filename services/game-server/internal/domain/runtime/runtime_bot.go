package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/ws"
)

// botRegistry holds the in-memory map of active bot players keyed by their
// store.Player UUID. It is safe for concurrent use.
// Bots are not persisted — if the server restarts they must be re-registered
// via POST /rooms/{id}/bots.
type botRegistry struct {
	mu   sync.RWMutex
	bots map[uuid.UUID]*bot.BotPlayer
}

func newBotRegistry() *botRegistry {
	return &botRegistry{bots: make(map[uuid.UUID]*bot.BotPlayer)}
}

func (r *botRegistry) register(bp *bot.BotPlayer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bots[bp.ID] = bp
}

func (r *botRegistry) unregister(id uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bots, id)
}

func (r *botRegistry) get(id uuid.UUID) (*bot.BotPlayer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bp, ok := r.bots[id]
	return bp, ok
}

// RegisterBot adds a BotPlayer to the runtime registry.
// Call this after creating the bot's store.Player row and before the game starts.
func (svc *Service) RegisterBot(bp *bot.BotPlayer) {
	svc.bots.register(bp)
}

// UnregisterBot removes a BotPlayer from the runtime registry.
// Call this when the bot leaves the room or the room is closed.
func (svc *Service) UnregisterBot(playerID uuid.UUID) {
	svc.bots.unregister(playerID)
}

// MaybeFireBot checks whether currentPlayerID is a registered bot and, if so,
// launches a goroutine to decide and apply its move.
//
// This is a no-op if no bot is registered for currentPlayerID.
// Must only be called when the game is not yet over.
// The goroutine re-checks session state before acting to handle races with
// surrender, timeout, and other concurrent operations.
func (svc *Service) MaybeFireBot(ctx context.Context, hub *ws.Hub, sessionID uuid.UUID, currentPlayerID uuid.UUID) {
	bp, ok := svc.bots.get(currentPlayerID)
	if !ok {
		return
	}

	go func() {
		// Use a fresh background context — the HTTP request context that
		// triggered the human move may already be cancelled by the time
		// the bot finishes thinking.
		botCtx := context.Background()

		// Re-fetch session to guard against races (surrender, timeout, etc.)
		session, err := svc.store.GetGameSession(botCtx, sessionID)
		if err != nil || session.FinishedAt != nil || session.SuspendedAt != nil {
			return
		}

		var state engine.GameState
		if err := json.Unmarshal(session.State, &state); err != nil {
			slog.Error("bot: decode state failed", "session_id", sessionID, "error", err)
			return
		}

		// Guard: only act when it is actually this bot's turn.
		// This prevents spurious moves when the bot is registered but not
		// the current player (e.g. when fired from handleAddBot on an
		// already-active session where the human is still to move).
		if state.CurrentPlayerID != engine.PlayerID(currentPlayerID.String()) {
			return
		}

		// Guard: don't call DecideMove on a terminal state.
		if bp.Adapter.IsTerminal(bot.NewBotState(state)) {
			return
		}

		payload, err := bp.DecideMove(botCtx, state)
		if err != nil {
			if errors.Is(err, bot.ErrNoMoves) {
				return
			}
			slog.Error("bot: decide move failed", "session_id", sessionID, "player_id", bp.ID, "error", err)
			return
		}

		result, err := svc.ApplyMove(botCtx, sessionID, bp.ID, payload)
		if err != nil {
			// ErrGameOver and ErrSessionNotFound are normal races — the game
			// ended between DecideMove and ApplyMove. Not a bug.
			slog.Error("bot: apply move failed", "session_id", sessionID, "player_id", bp.ID, "error", err)
			return
		}

		eventType := ws.EventMoveApplied
		if result.IsOver {
			eventType = ws.EventGameOver
		}

		players, _ := svc.store.ListRoomPlayers(botCtx, result.Session.RoomID)
		svc.BroadcastMove(botCtx, hub, result, eventType, players)
	}()
}
