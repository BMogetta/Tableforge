// Package runner drives one bot through N ranked games:
// authenticate → playerWS → queue → match → roomWS → game loop → repeat.
//
// The loop is single-threaded per bot. Each bot runs in its own goroutine
// from main, and each goroutine owns its Client and adapter.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/recess/game-server/cmd/bot-runner/internal/client"
	botpkg "github.com/recess/game-server/internal/bot"
	botadapter "github.com/recess/game-server/internal/bot/adapter"
	"github.com/recess/game-server/internal/bot/mcts"
	"github.com/recess/game-server/internal/domain/engine"
)

// Redis keys and channel mirrored from match-service/internal/queue/backfill.go.
// Duplicated as constants here rather than importing the match-service package
// to keep bot-runner free of match-service dependencies.
const (
	keyBotKnown        = "bot:known"
	keyBotAvailable    = "bot:available"
	channelBotActivate = "bot.activate"

	// backfillJoinTimeout bounds how long a backfill bot stays in the queue
	// after activation before it withdraws. If a second human arrived between
	// detection and the bot's join, the matchmaker pairs both humans and our
	// bot would otherwise sit there forever.
	backfillJoinTimeout = 45 * time.Second
)

// activateMessage is the payload the match-service publishes on
// bot.activate. Mirrors queue.activatePayload.
type activateMessage struct {
	PlayerID string `json:"player_id"`
}

// Event types we care about on the wire. Duplicated as untyped constants
// rather than imported from shared/ws so bot-runner stays free of the
// server-side ws package dependencies.
const (
	evtMatchFound  = "match_found"
	evtMatchReady  = "match_ready"
	evtGameReady   = "game_ready"
	evtMoveApplied = "move_applied"
	evtGameOver    = "game_over"
)

// wsEvent is the envelope ws-gateway sends to clients. Matches
// shared/ws.Event on the wire.
type wsEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// matchFoundPayload is the payload under a match_found event.
type matchFoundPayload struct {
	MatchID string `json:"match_id"`
}

// matchReadyPayload is the payload under a match_ready event.
type matchReadyPayload struct {
	RoomID    string `json:"room_id"`
	SessionID string `json:"session_id"`
}

// movePayload mirrors the subset of runtime.MoveResult that bot-runner needs.
// Duplicated locally so bot-runner does not depend on the server's store
// package (which brings in SQL drivers).
type movePayload struct {
	Session struct {
		ID     string `json:"id"`
		RoomID string `json:"room_id"`
	} `json:"session"`
	State  engine.GameState `json:"state"`
	IsOver bool             `json:"is_over"`
	Result *engine.Result   `json:"result,omitempty"`
}

// Runner wraps a single bot and plays numGames ranked matches.
type Runner struct {
	log      *slog.Logger
	username string
	gameID   string
	client   *client.Client
	bot      *botpkg.BotPlayer
}

// New builds a Runner.
// gameID selects which adapter to use ("rootaccess", "tictactoe").
// profileName is the name of a personality profile (easy / medium / hard / aggressive).
func New(
	log *slog.Logger,
	username string,
	playerID uuid.UUID,
	gameID string,
	profileName string,
	c *client.Client,
) (*Runner, error) {
	cfg, err := botpkg.ConfigFromProfile(profileName)
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", profileName, err)
	}
	adapter, err := botadapter.NewWithProfile(gameID, cfg.Personality)
	if err != nil {
		return nil, fmt.Errorf("adapter for %q: %w", gameID, err)
	}

	return &Runner{
		log:      log.With("bot", username, "profile", profileName),
		username: username,
		gameID:   gameID,
		client:   c,
		bot: &botpkg.BotPlayer{
			ID:      playerID,
			GameID:  gameID,
			Adapter: adapter,
			Config:  cfg,
			Search:  mcts.Search,
		},
	}, nil
}

// Run authenticates, opens the player WS, then plays numGames ranked matches.
// numGames == 0 runs indefinitely until ctx is cancelled. Fatal errors (auth,
// WS dial) abort; per-game errors are logged and the loop continues.
func (r *Runner) Run(ctx context.Context, numGames int) error {
	if err := r.client.Login(ctx); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	r.log.Info("authenticated")

	playerWS, err := r.client.DialPlayerWS(ctx)
	if err != nil {
		return fmt.Errorf("dial player ws: %w", err)
	}
	defer func() { _ = playerWS.Close() }()

	for i := 0; numGames == 0 || i < numGames; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		gameLog := r.log.With("game", i+1)
		if err := r.playOne(ctx, playerWS, gameLog); err != nil {
			gameLog.Error("game failed", "error", err)
			// Keep the bot alive — transient queue/match failures should not
			// kill the process. Small back-off so we do not spin on a server
			// that is rejecting everything.
			time.Sleep(time.Second)
			continue
		}
	}
	return nil
}

// RunBackfill authenticates once, registers the bot in bot:known + bot:available,
// subscribes to the bot.activate channel, and plays a match every time the
// match-service picks this bot to fill in for a lonely human.
//
// Between matches the bot sits idle in bot:available so the detector can pick
// a different bot for parallel activations. The bot never self-queues — if the
// activation signal never arrives, RunBackfill blocks until ctx is cancelled.
//
// The caller owns rdb; it should be a shared *redis.Client passed from main.
func (r *Runner) RunBackfill(ctx context.Context, rdb *redis.Client) error {
	if err := r.client.Login(ctx); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	r.log.Info("authenticated (backfill mode)")

	botID := r.bot.ID.String()

	// Register in the known + available sets atomically so the detector sees
	// a consistent view: a bot is never counted as "known but not available"
	// by virtue of the registration step itself.
	pipe := rdb.Pipeline()
	pipe.SAdd(ctx, keyBotKnown, botID)
	pipe.SAdd(ctx, keyBotAvailable, botID)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("register bot sets: %w", err)
	}

	// Deregister on exit using a fresh context — ctx is likely already
	// cancelled by the time we defer-run, and we still need the SREMs to
	// land so the detector stops considering us.
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := rdb.SRem(cleanupCtx, keyBotKnown, botID).Err(); err != nil {
			r.log.Warn("srem bot:known on shutdown", "error", err)
		}
		if err := rdb.SRem(cleanupCtx, keyBotAvailable, botID).Err(); err != nil {
			r.log.Warn("srem bot:available on shutdown", "error", err)
		}
	}()

	sub := rdb.Subscribe(ctx, channelBotActivate)
	defer sub.Close()
	ch := sub.Channel()

	r.log.Info("awaiting activation", "channel", channelBotActivate)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return fmt.Errorf("subscription channel closed")
			}
			var am activateMessage
			if err := json.Unmarshal([]byte(msg.Payload), &am); err != nil {
				continue
			}
			if am.PlayerID != botID {
				continue // another bot was picked
			}
			r.log.Info("activated — joining queue")

			gameLog := r.log.With("trigger", "backfill")
			// Dial player WS per activation — keeping it open between
			// activations lets intermediaries close it on idle, which then
			// surfaces as match_found: unexpected EOF right after join.
			if err := func() error {
				playerWS, err := r.client.DialPlayerWS(ctx)
				if err != nil {
					return fmt.Errorf("dial player ws: %w", err)
				}
				defer func() { _ = playerWS.Close() }()
				return r.playOneBackfill(ctx, playerWS, gameLog)
			}(); err != nil {
				gameLog.Error("backfill game failed", "error", err)
			}

			// Back into the available pool regardless of outcome — a failed
			// game should not evict the bot permanently. The detector's
			// SREM-on-claim already guarantees we won't be re-selected mid-game.
			if err := rdb.SAdd(ctx, keyBotAvailable, botID).Err(); err != nil {
				r.log.Error("re-add bot:available", "error", err)
			}
		}
	}
}

// playOneBackfill wraps playOne with a bounded wait for match_found. If the
// timeout fires, the bot leaves the queue so it doesn't sit forever when a
// second human paired with the first before the bot's POST /queue landed.
func (r *Runner) playOneBackfill(ctx context.Context, playerWS *websocket.Conn, log *slog.Logger) error {
	joinCtx, cancel := context.WithTimeout(ctx, backfillJoinTimeout)
	defer cancel()

	err := r.playOne(joinCtx, playerWS, log)
	if err == nil {
		return nil
	}
	// Timeout path — withdraw from the queue using the outer ctx so the
	// DELETE still flies if the join ctx deadline triggered the failure.
	if errors.Is(err, context.DeadlineExceeded) {
		log.Warn("match_found did not arrive in time — withdrawing")
		if leaveErr := r.client.LeaveQueue(ctx); leaveErr != nil {
			log.Warn("leave queue after timeout", "error", leaveErr)
		}
	}
	return err
}

// playOne runs one full queue → match → game → result cycle.
func (r *Runner) playOne(ctx context.Context, playerWS *websocket.Conn, log *slog.Logger) error {
	if err := r.client.JoinQueue(ctx); err != nil {
		return fmt.Errorf("join queue: %w", err)
	}
	log.Info("joined queue")

	// Wait for match proposal and accept it.
	var mf matchFoundPayload
	if err := readEventPayload(playerWS, evtMatchFound, &mf); err != nil {
		return fmt.Errorf("match_found: %w", err)
	}
	log.Info("match found", "match_id", mf.MatchID)

	if err := r.client.AcceptMatch(ctx, mf.MatchID); err != nil {
		return fmt.Errorf("accept match: %w", err)
	}

	// Wait for the room to be started (opponent accepted too).
	var mr matchReadyPayload
	if err := readEventPayload(playerWS, evtMatchReady, &mr); err != nil {
		return fmt.Errorf("match_ready: %w", err)
	}
	log.Info("match ready", "room_id", mr.RoomID, "session_id", mr.SessionID)

	roomWS, err := r.client.DialRoomWS(ctx, mr.RoomID)
	if err != nil {
		return fmt.Errorf("dial room ws: %w", err)
	}
	defer func() { _ = roomWS.Close() }()

	return r.playGame(ctx, roomWS, mr.SessionID, log)
}

// playGame drives one session from the ready vote through game_over.
//
// Ranked flow: after match_ready each player dials the room WS, votes ready,
// and waits for game_ready. The initial state is fetched via GET /sessions —
// game_ready's payload does not include it. If the bot is the first to act,
// it must take its turn proactively (no server event will wake it otherwise);
// subsequent turns are triggered by move_applied events from the opponent.
func (r *Runner) playGame(ctx context.Context, roomWS *websocket.Conn, sessionID string, log *slog.Logger) error {
	botID := engine.PlayerID(r.bot.ID.String())

	if err := r.client.MarkReady(ctx, sessionID); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	if err := waitForEvent(roomWS, evtGameReady); err != nil {
		return fmt.Errorf("await game_ready: %w", err)
	}

	// Initial state — needed because game_ready payload omits state, and if
	// the bot is on turn right now nothing else will nudge us to act.
	raw, err := r.client.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("fetch initial state: %w", err)
	}
	var initial struct {
		State  engine.GameState `json:"state"`
		Result *engine.Result   `json:"result"`
	}
	if err := json.Unmarshal(raw, &initial); err != nil {
		return fmt.Errorf("decode initial state: %w", err)
	}
	if initial.Result != nil {
		log.Info("game already over on join", "outcome", outcomeFromResult(initial.Result, botID))
		return nil
	}
	if initial.State.CurrentPlayerID == botID {
		if err := r.takeTurn(ctx, sessionID, initial.State, log); err != nil {
			return fmt.Errorf("take initial turn: %w", err)
		}
	}

	// Main loop: react to opponent moves and terminal events.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var evt wsEvent
		if err := roomWS.ReadJSON(&evt); err != nil {
			return fmt.Errorf("read room ws: %w", err)
		}

		switch evt.Type {
		case evtMoveApplied:
			var mp movePayload
			if err := json.Unmarshal(evt.Payload, &mp); err != nil {
				continue
			}
			if mp.State.CurrentPlayerID != botID {
				continue
			}
			if err := r.takeTurn(ctx, sessionID, mp.State, log); err != nil {
				return fmt.Errorf("take turn: %w", err)
			}

		case evtGameOver:
			var mp movePayload
			_ = json.Unmarshal(evt.Payload, &mp)
			log.Info("game over", "outcome", outcomeFromResult(mp.Result, botID))
			return nil

		default:
			// Ignore chat, presence, and other noise on the room channel.
		}
	}
}

func outcomeFromResult(result *engine.Result, bot engine.PlayerID) string {
	if result == nil || result.WinnerID == nil {
		return "draw"
	}
	if *result.WinnerID == bot {
		return "win"
	}
	return "loss"
}

// takeTurn runs MCTS on the current state and submits the resulting move.
// Errors returned from DecideMove are logged but not fatal — a game should
// continue even if a single turn fails to pick a move (shouldn't happen
// under normal conditions, but do not kill the whole runner).
//
// Before submitting, re-fetches authoritative state from the server. The
// WS payload can lag the server when the opponent's reaction to our own
// move races our event read — using it to decide what move to send risks
// a "not your turn" 400 because the server already advanced.
func (r *Runner) takeTurn(ctx context.Context, sessionID string, hintState engine.GameState, log *slog.Logger) error {
	botID := engine.PlayerID(r.bot.ID.String())

	raw, err := r.client.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("refresh state: %w", err)
	}
	var fresh struct {
		State  engine.GameState `json:"state"`
		Result *engine.Result   `json:"result"`
	}
	if err := json.Unmarshal(raw, &fresh); err != nil {
		return fmt.Errorf("decode fresh state: %w", err)
	}
	if fresh.Result != nil {
		return nil
	}
	if fresh.State.CurrentPlayerID != botID {
		log.Debug("skipped take turn — server says not our turn",
			"server_turn", fresh.State.CurrentPlayerID,
			"hint_turn", hintState.CurrentPlayerID)
		return nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, r.bot.Config.MaxThinkTime+2*time.Second)
	defer cancel()

	startedAt := time.Now()
	payload, err := r.bot.DecideMove(searchCtx, fresh.State)
	if err != nil {
		if errors.Is(err, botpkg.ErrNoMoves) {
			log.Warn("no moves available, skipping turn")
			return nil
		}
		return fmt.Errorf("decide move: %w", err)
	}

	// Profile-based pacing: if MCTS finished before the "thinking" floor,
	// sleep out the remainder so humans perceive a deliberate move. MCTS
	// time counts against the budget — slow profiles with short searches
	// get sleep, strong profiles with long searches submit as soon as they
	// decide. Zero MinMoveDelay disables pacing.
	if target := pickThinkTime(r.bot.Config.Personality); target > 0 {
		if remaining := target - time.Since(startedAt); remaining > 0 {
			if err := sleepCtx(ctx, remaining); err != nil {
				return err
			}
		}
	}

	if err := r.client.Move(ctx, sessionID, payload); err != nil {
		return fmt.Errorf("submit move: %w", err)
	}
	return nil
}

// pickThinkTime returns a random duration in
// [MinMoveDelay, MinMoveDelay+MoveDelayJitter) or 0 if pacing is disabled
// for the profile.
func pickThinkTime(p botpkg.PersonalityProfile) time.Duration {
	if p.MinMoveDelay <= 0 {
		return 0
	}
	if p.MoveDelayJitter <= 0 {
		return p.MinMoveDelay
	}
	return p.MinMoveDelay + time.Duration(rand.Int64N(int64(p.MoveDelayJitter)))
}

// sleepCtx blocks for d or until ctx is cancelled, whichever comes first.
// Returns ctx.Err() on cancellation so the caller stops the game loop.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// --- wire helpers -----------------------------------------------------------

// readEventPayload reads events from conn until one of type wantType arrives,
// then decodes its payload into out. Non-matching events are silently
// dropped — the player channel also carries presence / notification traffic.
func readEventPayload(conn *websocket.Conn, wantType string, out any) error {
	for {
		var evt wsEvent
		if err := conn.ReadJSON(&evt); err != nil {
			return err
		}
		if evt.Type != wantType {
			continue
		}
		return json.Unmarshal(evt.Payload, out)
	}
}

// waitForEvent is readEventPayload without payload decoding — used when we
// only care about the arrival of an event (e.g. game_ready).
func waitForEvent(conn *websocket.Conn, wantType string) error {
	for {
		var evt wsEvent
		if err := conn.ReadJSON(&evt); err != nil {
			return err
		}
		if evt.Type == wantType {
			return nil
		}
	}
}
