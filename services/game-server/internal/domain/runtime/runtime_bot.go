package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/platform/ws"
	"github.com/recess/shared/platform/randutil"
)

// Minimum total time between the bot being fired and its move being applied.
// MCTS on trivial states finishes in single-digit ms, which produces an
// unplayable "instant response" experience. Padding to a floor + jitter gives
// humans time to see the previous move's animation and read the board.
const (
	botMinMoveDelay    = 1200 * time.Millisecond
	botMoveDelayJitter = 600 * time.Millisecond
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

// isBot returns true if the player is a bot. It checks the in-memory registry
// first and falls back to the store's is_bot column (survives server restarts).
func (svc *Service) isBot(ctx context.Context, playerID uuid.UUID) bool {
	if _, ok := svc.bots.get(playerID); ok {
		return true
	}
	p, err := svc.store.GetPlayer(ctx, playerID)
	if err != nil {
		return false
	}
	return p.IsBot
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

		startedAt := time.Now()
		payload, err := bp.DecideMove(botCtx, state)
		if err != nil {
			if errors.Is(err, bot.ErrNoMoves) {
				return
			}
			slog.Error("bot: decide move failed", "session_id", sessionID, "player_id", bp.ID, "error", err)
			return
		}

		// Pace the bot to at least botMinMoveDelay + random jitter so humans
		// can perceive each move. DecideMove time counts against the budget.
		jitter, _ := randutil.Intn(int(botMoveDelayJitter.Milliseconds()))
		target := botMinMoveDelay + time.Duration(jitter)*time.Millisecond
		if elapsed := time.Since(startedAt); elapsed < target {
			time.Sleep(target - elapsed)
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

		// Re-fire if the new current player is also a bot. In rootaccess,
		// `resolveRound` sets the previous round's winner as the starter of
		// the next round; if the bot just won, `state.CurrentPlayerID` is the
		// bot again and nothing else will wake it (no HTTP /move call happens
		// until the human plays, which they can't because it's not their
		// turn). The internal guard at line 119 re-reads state and no-ops if
		// the situation has changed, so redundant calls are safe.
		//
		// TODO(refactor): the same "after a move, wake the next bot" pattern
		// is duplicated at api_session.go:149, api_bots.go:157 and
		// api_lobby.go:287. Pull it into a shared helper so the rule lives
		// in one place.
		if !result.IsOver {
			if nextUUID, err := uuid.Parse(string(result.State.CurrentPlayerID)); err == nil {
				svc.MaybeFireBot(botCtx, hub, sessionID, nextUUID)
			}
		}
	}()
}
