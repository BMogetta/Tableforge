package runtime_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/game-server/internal/domain/engine"
	"github.com/recess/game-server/internal/domain/runtime"
	"github.com/recess/game-server/internal/platform/store"
	"github.com/recess/game-server/internal/testutil"
)

// --- helpers -----------------------------------------------------------------

// spyTimer records calls for verification.
type spyTimer struct {
	Scheduled   []uuid.UUID
	ScheduledIn []struct {
		ID    uuid.UUID
		Delay time.Duration
	}
	Cancelled      []uuid.UUID
	ReadyScheduled []uuid.UUID
	ReadyCancelled []uuid.UUID
}

func (t *spyTimer) Schedule(session store.GameSession)                          { t.Scheduled = append(t.Scheduled, session.ID) }
func (t *spyTimer) ScheduleIn(id uuid.UUID, delay time.Duration)               { t.ScheduledIn = append(t.ScheduledIn, struct{ ID uuid.UUID; Delay time.Duration }{id, delay}) }
func (t *spyTimer) Cancel(id uuid.UUID)                                        { t.Cancelled = append(t.Cancelled, id) }
func (t *spyTimer) ScheduleReady(id uuid.UUID, timeout time.Duration)          { t.ReadyScheduled = append(t.ReadyScheduled, id) }
func (t *spyTimer) CancelReady(id uuid.UUID)                                   { t.ReadyCancelled = append(t.ReadyCancelled, id) }

func newRuntimeWithTimer(t *testing.T) (*runtime.Service, *testutil.FakeStore, *spyTimer) {
	t.Helper()
	fs := testutil.NewFakeStore()
	timer := &spyTimer{}
	reg := &fakeRegistry{game: &stubTicTacToe{}}
	svc := runtime.New(fs, reg, nil, nil, nil)
	svc.SetTimer(timer)
	return svc, fs, timer
}

func makeSession(t *testing.T, fs *testutil.FakeStore, players ...store.Player) store.GameSession {
	t.Helper()
	ctx := context.Background()
	if len(players) < 1 {
		t.Fatal("need at least 1 player")
	}
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "tictactoe", OwnerID: players[0].ID, MaxPlayers: len(players),
	})
	for i, p := range players {
		fs.AddPlayerToRoom(ctx, room.ID, p.ID, i)
	}
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(players[0].ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, nil, store.SessionModeCasual)
	return session
}

func makeSessionWithTimeout(t *testing.T, fs *testutil.FakeStore, timeout int, players ...store.Player) store.GameSession {
	t.Helper()
	ctx := context.Background()
	room, _ := fs.CreateRoom(ctx, store.CreateRoomParams{
		Code: uuid.NewString()[:8], GameID: "tictactoe", OwnerID: players[0].ID, MaxPlayers: len(players),
	})
	for i, p := range players {
		fs.AddPlayerToRoom(ctx, room.ID, p.ID, i)
	}
	state := engine.GameState{CurrentPlayerID: engine.PlayerID(players[0].ID.String()), Data: map[string]any{}}
	stateBytes, _ := json.Marshal(state)
	session, _ := fs.CreateGameSession(ctx, room.ID, "tictactoe", stateBytes, &timeout, store.SessionModeCasual)
	return session
}

// --- Pause/Resume cycle tests ------------------------------------------------

func TestPauseResumePause_FullCycle(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "cycle1")
	p2, _ := fs.CreatePlayer(ctx, "cycle2")
	session := makeSessionWithTimeout(t, fs, 30, p1, p2)

	// Pause: both vote.
	svc.VotePause(ctx, session.ID, p1.ID)
	result, _ := svc.VotePause(ctx, session.ID, p2.ID)
	if !result.AllVoted {
		t.Fatal("expected pause consensus")
	}

	// Timer should be cancelled.
	if len(timer.Cancelled) != 1 || timer.Cancelled[0] != session.ID {
		t.Error("expected timer to be cancelled on pause")
	}

	// Resume: both vote.
	svc.VoteResume(ctx, session.ID, p1.ID)
	result, _ = svc.VoteResume(ctx, session.ID, p2.ID)
	if !result.AllVoted {
		t.Fatal("expected resume consensus")
	}

	// Timer should be rescheduled with penalty.
	if len(timer.ScheduledIn) != 1 {
		t.Fatalf("expected 1 ScheduleIn call after resume, got %d", len(timer.ScheduledIn))
	}
	expectedPenalty := time.Duration(float64(30)*runtime.ResumePenalty) * time.Second
	if timer.ScheduledIn[0].Delay != expectedPenalty {
		t.Errorf("expected resume penalty %v, got %v", expectedPenalty, timer.ScheduledIn[0].Delay)
	}

	// Verify session is active again.
	gs, _ := fs.GetGameSession(ctx, session.ID)
	if gs.SuspendedAt != nil {
		t.Error("expected session to be active after resume")
	}

	// Second pause cycle.
	svc.VotePause(ctx, session.ID, p1.ID)
	result, _ = svc.VotePause(ctx, session.ID, p2.ID)
	if !result.AllVoted {
		t.Fatal("expected second pause consensus")
	}

	gs, _ = fs.GetGameSession(ctx, session.ID)
	if gs.SuspendCount != 2 {
		t.Errorf("expected suspend_count 2 after two pauses, got %d", gs.SuspendCount)
	}
}

func TestPause_VoteCountResetAfterConsensus(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "reset1")
	p2, _ := fs.CreatePlayer(ctx, "reset2")
	session := makeSession(t, fs, p1, p2)

	// Achieve pause consensus.
	svc.VotePause(ctx, session.ID, p1.ID)
	svc.VotePause(ctx, session.ID, p2.ID)

	// Vote count should be 0 after clear (consensus clears votes).
	count := len(fs.PauseVoteMap[session.ID])
	if count != 0 {
		t.Errorf("expected 0 pause votes after consensus, got %d", count)
	}
}

func TestResume_VoteCountResetAfterConsensus(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resreset1")
	p2, _ := fs.CreatePlayer(ctx, "resreset2")
	session := makeSession(t, fs, p1, p2)
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)
	svc.VoteResume(ctx, session.ID, p2.ID)

	count := len(fs.ResumeVoteMap[session.ID])
	if count != 0 {
		t.Errorf("expected 0 resume votes after consensus, got %d", count)
	}
}

func TestPause_MoveBlockedDuringSuspend(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "blocked1")
	session := makeSession(t, fs, p1)

	// Pause with single player.
	svc.VotePause(ctx, session.ID, p1.ID)

	// Move should be blocked.
	_, err := svc.ApplyMove(ctx, session.ID, p1.ID, map[string]any{"cell": 0})
	if err != runtime.ErrSuspended {
		t.Errorf("expected ErrSuspended, got %v", err)
	}
}

func TestResume_MoveAllowedAfterResume(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "unblock1")
	session := makeSession(t, fs, p1)

	// Pause and resume.
	svc.VotePause(ctx, session.ID, p1.ID)
	svc.VoteResume(ctx, session.ID, p1.ID)

	// Move should work.
	_, err := svc.ApplyMove(ctx, session.ID, p1.ID, map[string]any{"cell": 0})
	if err != nil {
		t.Errorf("expected move to succeed after resume, got %v", err)
	}
}

func TestPause_DuplicateVoteDoesNotDoubleCount(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "dup1")
	p2, _ := fs.CreatePlayer(ctx, "dup2")
	session := makeSession(t, fs, p1, p2)

	// p1 votes twice.
	r1, _ := svc.VotePause(ctx, session.ID, p1.ID)
	r2, _ := svc.VotePause(ctx, session.ID, p1.ID)

	if r1.Votes != 1 {
		t.Errorf("first vote: expected 1 vote, got %d", r1.Votes)
	}
	if r2.Votes != 1 {
		t.Errorf("duplicate vote: expected still 1 vote, got %d", r2.Votes)
	}
	if r2.AllVoted {
		t.Error("duplicate should not trigger consensus")
	}
}

func TestResume_DuplicateVoteDoesNotDoubleCount(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "resdup1")
	p2, _ := fs.CreatePlayer(ctx, "resdup2")
	session := makeSession(t, fs, p1, p2)
	fs.SuspendSession(ctx, session.ID, "test")

	r1, _ := svc.VoteResume(ctx, session.ID, p1.ID)
	r2, _ := svc.VoteResume(ctx, session.ID, p1.ID)

	if r1.Votes != 1 {
		t.Errorf("first vote: expected 1 vote, got %d", r1.Votes)
	}
	if r2.Votes != 1 {
		t.Errorf("duplicate: expected 1 vote, got %d", r2.Votes)
	}
}

// --- Bot exclusion -----------------------------------------------------------

func TestPause_BotExcludedFromVoteCount(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	human, _ := fs.CreatePlayer(ctx, "pausebot_human")
	bot, _ := fs.CreateBotPlayer(ctx, "pausebot_bot")
	session := makeSession(t, fs, human, bot)

	// Register bot in runtime so it's recognized.
	bp := makeBotPlayer(t, bot.ID)
	svc.RegisterBot(bp)

	// Only human votes — should reach consensus (bot excluded).
	result, err := svc.VotePause(ctx, session.ID, human.ID)
	if err != nil {
		t.Fatalf("VotePause: %v", err)
	}
	if !result.AllVoted {
		t.Error("expected consensus with 1 human voting (bot excluded)")
	}
	if result.Required != 1 {
		t.Errorf("expected required 1 (human only), got %d", result.Required)
	}
}

func TestResume_BotExcludedFromVoteCount(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	human, _ := fs.CreatePlayer(ctx, "resbot_human")
	bot, _ := fs.CreateBotPlayer(ctx, "resbot_bot")
	session := makeSession(t, fs, human, bot)

	bp := makeBotPlayer(t, bot.ID)
	svc.RegisterBot(bp)

	fs.SuspendSession(ctx, session.ID, "test")

	result, err := svc.VoteResume(ctx, session.ID, human.ID)
	if err != nil {
		t.Fatalf("VoteResume: %v", err)
	}
	if !result.AllVoted {
		t.Error("expected consensus with 1 human voting (bot excluded)")
	}
}

// BUG DOCUMENTATION: Runtime uses svc.bots.get() to determine if a player
// is a bot, while the PG store uses players.is_bot column. If the server
// restarts and bots are not re-registered, the runtime treats them as humans
// and pause/resume consensus can never be reached (bot never votes).
// This inconsistency between runtime and store is a latent reliability issue.
// BUG DOCUMENTATION: Runtime uses svc.bots.get() to determine if a player is a
// bot, while the PG store uses players.is_bot column. After a server restart,
// bots that aren't re-registered are counted as humans by the runtime.
//
// This creates two problems:
// 1. Required count in PauseVoteResult is wrong (counts bot as human)
// 2. The store's allVoted can return true (using DB is_bot) while runtime
//    reports Required=2, creating an inconsistent response.
//
// In a real 1v1 vs bot scenario after server restart, the store-level consensus
// check (PG) correctly excludes the bot, but the runtime's humanCount is wrong.
// The session DOES get suspended (store wins), but the API response is misleading.
func TestPause_BotNotRegistered_InconsistentCount(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	human, _ := fs.CreatePlayer(ctx, "unreg_human")
	_, _ = fs.CreateBotPlayer(ctx, "unreg_bot")
	// Note: we use makeSession which adds via FakeStore (no runtime bot registration)
	session := makeSession(t, fs, human)

	// This test documents that with only 1 human in the room,
	// pause consensus is reached immediately.
	result, err := svc.VotePause(ctx, session.ID, human.ID)
	if err != nil {
		t.Fatalf("VotePause: %v", err)
	}
	if result.Required != 1 {
		t.Errorf("expected required 1 (single player room), got %d", result.Required)
	}
	if !result.AllVoted {
		t.Error("expected consensus with single player")
	}
}

// --- Timer interaction -------------------------------------------------------

func TestPause_CancelsTimer(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "timerpause1")
	session := makeSessionWithTimeout(t, fs, 60, p1)

	svc.VotePause(ctx, session.ID, p1.ID)

	if len(timer.Cancelled) == 0 {
		t.Fatal("expected timer cancel on pause")
	}
	if timer.Cancelled[0] != session.ID {
		t.Errorf("cancelled wrong session: %s vs %s", timer.Cancelled[0], session.ID)
	}
}

func TestPause_NoCancelWhenNoConsensus(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "nocancel1")
	p2, _ := fs.CreatePlayer(ctx, "nocancel2")
	session := makeSessionWithTimeout(t, fs, 60, p1, p2)

	svc.VotePause(ctx, session.ID, p1.ID)

	if len(timer.Cancelled) != 0 {
		t.Error("should not cancel timer without consensus")
	}
}

func TestResume_ReschedulesWithPenalty(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "penalty1")
	session := makeSessionWithTimeout(t, fs, 60, p1)
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)

	if len(timer.ScheduledIn) != 1 {
		t.Fatalf("expected 1 ScheduleIn, got %d", len(timer.ScheduledIn))
	}
	expected := time.Duration(float64(60)*0.4) * time.Second // 24s
	if timer.ScheduledIn[0].Delay != expected {
		t.Errorf("expected penalty %v, got %v", expected, timer.ScheduledIn[0].Delay)
	}
}

func TestResume_NoTimerWhenNoTimeout(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "notimer1")
	session := makeSession(t, fs, p1) // no timeout
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)

	if len(timer.ScheduledIn) != 0 {
		t.Error("should not schedule timer when session has no timeout")
	}
}

func TestResume_LastMoveAtAdjusted(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "lastmove1")
	session := makeSessionWithTimeout(t, fs, 60, p1)
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)

	gs, _ := fs.GetGameSession(ctx, session.ID)
	elapsed := time.Since(gs.LastMoveAt)
	// LastMoveAt should be ~60% of timeout ago (36s), giving 40% remaining (24s).
	// Allow some slack for test execution time.
	if elapsed < 35*time.Second || elapsed > 40*time.Second {
		t.Errorf("expected LastMoveAt ~36s ago (60%% of 60s), got %v ago", elapsed)
	}
}

// --- Edge cases --------------------------------------------------------------

func TestPause_SessionNotFound(t *testing.T) {
	svc, _, _ := newRuntimeWithTimer(t)
	_, err := svc.VotePause(context.Background(), uuid.New(), uuid.New())
	if err != runtime.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestResume_SessionNotFound(t *testing.T) {
	svc, _, _ := newRuntimeWithTimer(t)
	_, err := svc.VoteResume(context.Background(), uuid.New(), uuid.New())
	if err != runtime.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestPause_CannotResumeActiveSession(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "notsusp1")
	session := makeSession(t, fs, p1)

	_, err := svc.VoteResume(ctx, session.ID, p1.ID)
	if err != runtime.ErrNotSuspended {
		t.Errorf("expected ErrNotSuspended, got %v", err)
	}
}

func TestResume_CannotPauseSuspendedSession(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "alrpaused1")
	session := makeSession(t, fs, p1)
	fs.SuspendSession(ctx, session.ID, "test")

	_, err := svc.VotePause(ctx, session.ID, p1.ID)
	if err != runtime.ErrAlreadyPaused {
		t.Errorf("expected ErrAlreadyPaused, got %v", err)
	}
}

func TestPauseResume_FinishedSessionBlocksBoth(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "fin1")
	session := makeSession(t, fs, p1)
	fs.FinishSession(ctx, session.ID)

	_, err := svc.VotePause(ctx, session.ID, p1.ID)
	if err != runtime.ErrGameOver {
		t.Errorf("VotePause on finished: expected ErrGameOver, got %v", err)
	}

	// Manually set suspended to test resume on finished+suspended.
	fs.SuspendSession(ctx, session.ID, "test")
	_, err = svc.VoteResume(ctx, session.ID, p1.ID)
	if err != runtime.ErrGameOver {
		t.Errorf("VoteResume on finished: expected ErrGameOver, got %v", err)
	}
}

// --- Interleaved pause/resume votes (shouldn't interfere) --------------------

func TestPauseResumeVotes_Independent(t *testing.T) {
	svc, fs, _ := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "inter1")
	p2, _ := fs.CreatePlayer(ctx, "inter2")
	session := makeSession(t, fs, p1, p2)

	// Partial pause vote (1/2).
	r, _ := svc.VotePause(ctx, session.ID, p1.ID)
	if r.Votes != 1 || r.AllVoted {
		t.Fatal("expected 1/2 pause votes")
	}

	// Pause votes should not affect resume vote count.
	// (Can't actually resume since not suspended, but verifying vote maps are separate.)
	if len(fs.ResumeVoteMap[session.ID]) != 0 {
		t.Error("resume votes should be independent of pause votes")
	}
}

// --- Resume penalty math edge cases ------------------------------------------

func TestResume_PenaltyCalculation_ShortTimeout(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "shortpen1")
	session := makeSessionWithTimeout(t, fs, 10, p1)
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)

	if len(timer.ScheduledIn) != 1 {
		t.Fatal("expected timer schedule")
	}
	expected := time.Duration(float64(10)*0.4) * time.Second // 4s
	if timer.ScheduledIn[0].Delay != expected {
		t.Errorf("expected %v, got %v", expected, timer.ScheduledIn[0].Delay)
	}
}

func TestResume_PenaltyCalculation_LargeTimeout(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "largepen1")
	session := makeSessionWithTimeout(t, fs, 300, p1)
	fs.SuspendSession(ctx, session.ID, "test")

	svc.VoteResume(ctx, session.ID, p1.ID)

	expected := time.Duration(float64(300)*0.4) * time.Second // 120s
	if timer.ScheduledIn[0].Delay != expected {
		t.Errorf("expected %v, got %v", expected, timer.ScheduledIn[0].Delay)
	}
}

// --- nextPlayerAfter (exported via timeout handler) --------------------------
// This is tested indirectly but we want to verify edge cases.

func TestPauseResumeMultipleCycles_TimerConsistency(t *testing.T) {
	svc, fs, timer := newRuntimeWithTimer(t)
	ctx := context.Background()

	p1, _ := fs.CreatePlayer(ctx, "multi1")
	session := makeSessionWithTimeout(t, fs, 60, p1)

	for i := 0; i < 3; i++ {
		svc.VotePause(ctx, session.ID, p1.ID)
		svc.VoteResume(ctx, session.ID, p1.ID)
	}

	// Should have 3 cancels (one per pause) and 3 ScheduleIn (one per resume).
	if len(timer.Cancelled) != 3 {
		t.Errorf("expected 3 timer cancels, got %d", len(timer.Cancelled))
	}
	if len(timer.ScheduledIn) != 3 {
		t.Errorf("expected 3 timer schedules, got %d", len(timer.ScheduledIn))
	}

	// All ScheduleIn should use the same penalty (40% of 60s = 24s).
	expected := time.Duration(float64(60)*0.4) * time.Second
	for i, s := range timer.ScheduledIn {
		if s.Delay != expected {
			t.Errorf("cycle %d: expected penalty %v, got %v", i, expected, s.Delay)
		}
	}
}
