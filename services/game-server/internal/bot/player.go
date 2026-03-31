package bot

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/tableforge/game-server/internal/domain/engine"
)

// ErrNoMoves is returned when a search is requested on a state with no legal
// moves. Defined here (not in mcts) so callers can use errors.Is without
// importing mcts, and so mcts can return it without creating an import cycle.
var ErrNoMoves = errors.New("mcts: no legal moves available")

// SearchFunc is the signature of the MCTS search function.
// Inject mcts.Search at construction time to avoid an import cycle between
// the bot package (interfaces) and the mcts package (implementation).
type SearchFunc func(
	ctx context.Context,
	root BotGameState,
	playerID engine.PlayerID,
	adapter BotAdapter,
	cfg BotConfig,
) (BotMove, error)

// BotPlayer represents a single bot instance attached to a game session.
// It holds the adapter and config needed to run MCTS and produce a move payload.
//
// BotPlayer is stateless with respect to the game — it does not store game
// state between calls. All state is passed in via DecideMove.
type BotPlayer struct {
	// ID is the store.Player UUID for this bot.
	// Bots are registered as real player rows so AddPlayerToRoom works unchanged.
	ID uuid.UUID

	// GameID identifies which game this bot plays (e.g. "tictactoe").
	GameID string

	// Adapter is the game-specific MCTS bridge.
	Adapter BotAdapter

	// Config controls MCTS search parameters and personality.
	Config BotConfig

	// Search is the MCTS search function. Inject mcts.Search at construction.
	// Exposed as a field so tests can substitute a fast fake without running
	// full MCTS.
	Search SearchFunc
}

// NewBotPlayer constructs a BotPlayer with a freshly generated UUID.
// search must be mcts.Search (or a test double).
// Username convention for the store.Player row: "bot:<profile>:<8-char uuid prefix>"
// guarantees uniqueness across concurrent bot registrations.
func NewBotPlayer(gameID string, adapter BotAdapter, cfg BotConfig, search SearchFunc) *BotPlayer {
	return &BotPlayer{
		ID:      uuid.New(),
		GameID:  gameID,
		Adapter: adapter,
		Config:  cfg,
		Search:  search,
	}
}

// DecideMove runs MCTS on the given state and returns the move payload to apply.
// The payload is ready to pass directly to runtime.Service.ApplyMove.
//
// A context deadline equal to Config.MaxThinkTime is applied internally if the
// caller's ctx has no deadline already set. Callers may pass a tighter deadline
// to override this.
//
// Returns ErrNoMoves if the state is terminal or has no legal moves.
// Returns context.Canceled or context.DeadlineExceeded if ctx is done before
// a move is found.
func (bp *BotPlayer) DecideMove(ctx context.Context, state engine.GameState) (map[string]any, error) {
	botState := NewBotState(state)

	// Apply MaxThinkTime as deadline if the caller has not set one.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, bp.Config.MaxThinkTime)
		defer cancel()
	}

	move, err := bp.Search(ctx, botState, engine.PlayerID(bp.ID.String()), bp.Adapter, bp.Config)
	if err != nil {
		if errors.Is(err, ErrNoMoves) {
			return nil, ErrNoMoves
		}
		return nil, fmt.Errorf("bot: DecideMove: %w", err)
	}

	payload, err := move.Payload()
	if err != nil {
		return nil, fmt.Errorf("bot: DecideMove: deserialize move: %w", err)
	}

	return payload, nil
}
