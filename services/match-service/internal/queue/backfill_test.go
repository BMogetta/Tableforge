package queue

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	ratingv1 "github.com/recess/shared/proto/rating/v1"
	"google.golang.org/grpc"
)

// ---------------------------------------------------------------------------
// Rating client fakes
// ---------------------------------------------------------------------------

// ratingByPlayer is a rating client that returns a pre-configured MMR per
// player, or an error for IDs listed in errIDs. Unknown IDs return 0 MMR
// without error so tests don't need to seed every participant.
type ratingByPlayer struct {
	ratingv1.RatingServiceClient
	mmrs   map[string]float64
	errIDs map[string]bool
	calls  []string
	mu     sync.Mutex
}

func (r *ratingByPlayer) GetRating(_ context.Context, req *ratingv1.GetRatingRequest, _ ...grpc.CallOption) (*ratingv1.GetRatingResponse, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req.PlayerId)
	r.mu.Unlock()
	if r.errIDs[req.PlayerId] {
		return nil, errors.New("rating unavailable for " + req.PlayerId)
	}
	mmr := r.mmrs[req.PlayerId]
	return &ratingv1.GetRatingResponse{Mmr: mmr}, nil
}

func (r *ratingByPlayer) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newBackfillTestService returns a Service + miniredis with backfill enabled.
func newBackfillTestService(t *testing.T, rc ratingv1.RatingServiceClient, cfg BackfillConfig) (*Service, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	svc, mr := newQueueTestService(t, rc)
	svc.SetBackfillConfig(cfg)
	return svc, svc.rdb, mr
}

// enqueuePlayer wires queue:ranked + queue:meta directly so tests can control
// joinedAt precisely. Bypasses svc.Enqueue (which consults rating-service and
// uses time.Now) to keep the setup deterministic.
func enqueuePlayer(t *testing.T, rdb *redis.Client, playerID uuid.UUID, mmr float64, joinedAt time.Time) {
	t.Helper()
	ctx := context.Background()
	if err := rdb.ZAdd(ctx, keyQueueSortedSet, redis.Z{Score: mmr, Member: playerID.String()}).Err(); err != nil {
		t.Fatalf("zadd: %v", err)
	}
	if err := rdb.HSet(ctx, metaKey(playerID), map[string]any{
		"joined_at": joinedAt.UTC().Format(time.RFC3339),
		"mmr":       strconv.FormatFloat(mmr, 'f', -1, 64),
	}).Err(); err != nil {
		t.Fatalf("hset meta: %v", err)
	}
}

// registerBot adds a bot UUID to both bot:known and bot:available.
func registerBot(t *testing.T, rdb *redis.Client, botID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	if err := rdb.SAdd(ctx, keyBotKnown, botID.String()).Err(); err != nil {
		t.Fatalf("sadd known: %v", err)
	}
	if err := rdb.SAdd(ctx, keyBotAvailable, botID.String()).Err(); err != nil {
		t.Fatalf("sadd available: %v", err)
	}
}

// runScanWithSubscriber starts a subscriber goroutine that reports the first
// activation it sees (or "" on timeout), runs BackfillScan, then returns what
// the subscriber captured. Ensures the subscriber is ready before the scan
// publishes.
func runScanWithSubscriber(t *testing.T, svc *Service, rdb *redis.Client, timeout time.Duration) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	sub := rdb.Subscribe(ctx, ChannelBotActivate)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe confirm: %v", err)
	}

	result := make(chan string, 1)
	go func() {
		ch := sub.Channel()
		select {
		case msg := <-ch:
			var p activatePayload
			_ = json.Unmarshal([]byte(msg.Payload), &p)
			result <- p.PlayerID
		case <-ctx.Done():
			result <- ""
		}
	}()

	svc.BackfillScan(context.Background())

	// Give the subscriber a brief window to receive anything the scan published.
	select {
	case got := <-result:
		return got
	case <-time.After(timeout):
		return ""
	}
}

func testBackfillCfg(threshold time.Duration, maxActive int) BackfillConfig {
	return BackfillConfig{
		Enabled:   true,
		Threshold: threshold,
		MaxActive: maxActive,
	}
}

// ---------------------------------------------------------------------------
// Disabled path
// ---------------------------------------------------------------------------

func TestBackfillScan_Disabled_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, BackfillConfig{Enabled: false})

	human := uuid.New()
	bot := uuid.New()
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Hour))
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation when disabled, got %q", got)
	}

	// Bot should still be in available — disabled path must not SREM.
	n, _ := rdb.SCard(context.Background(), keyBotAvailable).Result()
	if n != 1 {
		t.Errorf("bot:available cardinality = %d, want 1", n)
	}
}

// ---------------------------------------------------------------------------
// Queue-shape gates
// ---------------------------------------------------------------------------

func TestBackfillScan_EmptyQueue_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	bot := uuid.New()
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation for empty queue, got %q", got)
	}
}

func TestBackfillScan_TwoHumansQueued_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	h1, h2 := uuid.New(), uuid.New()
	bot := uuid.New()
	past := time.Now().Add(-10 * time.Minute)
	enqueuePlayer(t, rdb, h1, 1000, past)
	enqueuePlayer(t, rdb, h2, 1010, past)
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation with 2 humans, got %q", got)
	}
}

func TestBackfillScan_HumanPlusBotAlreadyInQueue_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	activeBot := uuid.New()
	availableBot := uuid.New()

	past := time.Now().Add(-10 * time.Minute)
	enqueuePlayer(t, rdb, human, 1000, past)

	// An already-queued bot: in the queue AND in bot:known, but NOT in available.
	enqueuePlayer(t, rdb, activeBot, 1000, past)
	ctx := context.Background()
	if err := rdb.SAdd(ctx, keyBotKnown, activeBot.String()).Err(); err != nil {
		t.Fatal(err)
	}
	// Another bot sitting idle in the pool.
	registerBot(t, rdb, availableBot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation when bot already in queue, got %q", got)
	}

	n, _ := rdb.SCard(ctx, keyBotAvailable).Result()
	if n != 1 {
		t.Errorf("idle bot should remain in available, SCard = %d", n)
	}
}

// ---------------------------------------------------------------------------
// Wait-time gate
// ---------------------------------------------------------------------------

func TestBackfillScan_HumanBelowThreshold_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(30*time.Second, 3))

	human := uuid.New()
	bot := uuid.New()
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-2*time.Second))
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation below threshold, got %q", got)
	}
	n, _ := rdb.SCard(context.Background(), keyBotAvailable).Result()
	if n != 1 {
		t.Errorf("bot should remain available, SCard = %d", n)
	}
}

func TestBackfillScan_MalformedJoinedAt_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	bot := uuid.New()
	ctx := context.Background()
	if err := rdb.ZAdd(ctx, keyQueueSortedSet, redis.Z{Score: 1000, Member: human.String()}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.HSet(ctx, metaKey(human), map[string]any{
		"joined_at": "not-a-timestamp",
		"mmr":       "1000",
	}).Err(); err != nil {
		t.Fatal(err)
	}
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("malformed joined_at should abort, got %q", got)
	}
}

func TestBackfillScan_MissingMeta_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	bot := uuid.New()
	// queue:ranked entry without queue:meta:{id} — possible if the writer
	// crashed between ZAdd and HSet.
	if err := rdb.ZAdd(context.Background(), keyQueueSortedSet, redis.Z{Score: 1000, Member: human.String()}).Err(); err != nil {
		t.Fatal(err)
	}
	registerBot(t, rdb, bot)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("missing meta should abort, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Capacity gates
// ---------------------------------------------------------------------------

func TestBackfillScan_NoAvailableBots_NoOp(t *testing.T) {
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	// No bots registered at all.

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation with no bots, got %q", got)
	}
}

func TestBackfillScan_AllBotsBusy_NoOp(t *testing.T) {
	// Two bots known, both claimed (available set empty).
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))

	ctx := context.Background()
	bot1, bot2 := uuid.New(), uuid.New()
	if err := rdb.SAdd(ctx, keyBotKnown, bot1.String(), bot2.String()).Err(); err != nil {
		t.Fatal(err)
	}
	// available stays empty → SCard(available) == 0 → short-circuits.

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation when all bots busy, got %q", got)
	}
}

func TestBackfillScan_MaxActiveReached_NoOp(t *testing.T) {
	// 4 known, 1 available → active = 3. With MaxActive=3, scan must not
	// claim that last idle bot.
	svc, rdb, _ := newBackfillTestService(t, &ratingByPlayer{
		mmrs: map[string]float64{},
	}, testBackfillCfg(1*time.Second, 3))

	human := uuid.New()
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))

	ctx := context.Background()
	busy := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	idle := uuid.New()
	for _, id := range busy {
		if err := rdb.SAdd(ctx, keyBotKnown, id.String()).Err(); err != nil {
			t.Fatal(err)
		}
	}
	registerBot(t, rdb, idle) // known + available

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation at max active, got %q", got)
	}
	// Idle bot remained in available (no claim).
	members, _ := rdb.SMembers(ctx, keyBotAvailable).Result()
	if len(members) != 1 || members[0] != idle.String() {
		t.Errorf("idle bot should remain untouched, got %v", members)
	}
}

func TestBackfillScan_MaxActiveBoundary_StillActivates(t *testing.T) {
	// 3 known, 1 available → active = 2. MaxActive=3 leaves room for one more.
	human := uuid.New()
	busyA, busyB := uuid.New(), uuid.New()
	idle := uuid.New()

	rc := &ratingByPlayer{mmrs: map[string]float64{idle.String(): 1000}}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 3))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	ctx := context.Background()
	if err := rdb.SAdd(ctx, keyBotKnown, busyA.String(), busyB.String()).Err(); err != nil {
		t.Fatal(err)
	}
	registerBot(t, rdb, idle)

	got := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got != idle.String() {
		t.Errorf("expected %s activated, got %q", idle, got)
	}
}

// ---------------------------------------------------------------------------
// Happy path + selection
// ---------------------------------------------------------------------------

func TestBackfillScan_PicksClosestMMR(t *testing.T) {
	human := uuid.New()
	far := uuid.New()
	closest := uuid.New()
	other := uuid.New()

	rc := &ratingByPlayer{
		mmrs: map[string]float64{
			far.String():     500,
			closest.String(): 1015,
			other.String():   900,
		},
	}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 10))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	registerBot(t, rdb, far)
	registerBot(t, rdb, closest)
	registerBot(t, rdb, other)

	got := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got != closest.String() {
		t.Errorf("expected closest bot %s, got %q", closest, got)
	}

	// Claimed bot SREM'd, others untouched.
	ctx := context.Background()
	isMember, _ := rdb.SIsMember(ctx, keyBotAvailable, closest.String()).Result()
	if isMember {
		t.Error("claimed bot should have been removed from bot:available")
	}
	// bot:known still has all 3.
	n, _ := rdb.SCard(ctx, keyBotKnown).Result()
	if n != 3 {
		t.Errorf("bot:known cardinality = %d, want 3", n)
	}

	// Rating-service called once per available bot.
	if rc.callCount() != 3 {
		t.Errorf("expected 3 rating lookups, got %d", rc.callCount())
	}
}

func TestBackfillScan_ActivationPayloadShape(t *testing.T) {
	human := uuid.New()
	bot := uuid.New()

	rc := &ratingByPlayer{mmrs: map[string]float64{bot.String(): 1000}}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 3))
	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	registerBot(t, rdb, bot)

	// Capture raw payload to verify JSON shape.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	sub := rdb.Subscribe(ctx, ChannelBotActivate)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	payloadCh := make(chan string, 1)
	go func() {
		select {
		case msg := <-sub.Channel():
			payloadCh <- msg.Payload
		case <-ctx.Done():
			payloadCh <- ""
		}
	}()

	svc.BackfillScan(context.Background())

	raw := <-payloadCh
	if raw == "" {
		t.Fatal("no payload received")
	}
	// Must decode cleanly and carry the expected field.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("not valid JSON: %v — raw: %q", err, raw)
	}
	if decoded["player_id"] != bot.String() {
		t.Errorf("player_id = %v, want %s", decoded["player_id"], bot)
	}
	if !strings.Contains(raw, "player_id") {
		t.Errorf("payload missing player_id field: %q", raw)
	}
}

// ---------------------------------------------------------------------------
// Rating-service error paths
// ---------------------------------------------------------------------------

func TestBackfillScan_RatingErrorOnOneBot_FallsThrough(t *testing.T) {
	human := uuid.New()
	broken := uuid.New()
	good := uuid.New()

	rc := &ratingByPlayer{
		mmrs:   map[string]float64{good.String(): 1005},
		errIDs: map[string]bool{broken.String(): true},
	}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 10))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	registerBot(t, rdb, broken)
	registerBot(t, rdb, good)

	got := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got != good.String() {
		t.Errorf("expected good bot %s, got %q", good, got)
	}

	// Broken bot was untouched.
	isMember, _ := rdb.SIsMember(context.Background(), keyBotAvailable, broken.String()).Result()
	if !isMember {
		t.Error("broken bot should remain in available pool for later retries")
	}
}

func TestBackfillScan_AllBotsRatingBroken_NoActivation(t *testing.T) {
	human := uuid.New()
	b1, b2 := uuid.New(), uuid.New()

	rc := &ratingByPlayer{
		errIDs: map[string]bool{b1.String(): true, b2.String(): true},
	}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 10))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	registerBot(t, rdb, b1)
	registerBot(t, rdb, b2)

	got := runScanWithSubscriber(t, svc, rdb, 200*time.Millisecond)
	if got != "" {
		t.Errorf("expected no activation when all ratings broken, got %q", got)
	}
	n, _ := rdb.SCard(context.Background(), keyBotAvailable).Result()
	if n != 2 {
		t.Errorf("no bots should have been claimed, SCard = %d", n)
	}
}

// ---------------------------------------------------------------------------
// Idempotence / claim semantics
// ---------------------------------------------------------------------------

func TestBackfillScan_TwoScansInRow_OnlyOneActivation(t *testing.T) {
	// After the first scan removes the bot from available, the second scan
	// must short-circuit (no more available bots). No re-activation.
	human := uuid.New()
	bot := uuid.New()

	rc := &ratingByPlayer{mmrs: map[string]float64{bot.String(): 1000}}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 3))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	registerBot(t, rdb, bot)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	sub := rdb.Subscribe(ctx, ChannelBotActivate)
	defer sub.Close()
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	var activations []string
	var mu sync.Mutex
	done := make(chan struct{})
	go func() {
		defer close(done)
		ch := sub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var p activatePayload
				_ = json.Unmarshal([]byte(msg.Payload), &p)
				mu.Lock()
				activations = append(activations, p.PlayerID)
				mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	svc.BackfillScan(context.Background())
	svc.BackfillScan(context.Background())

	// Let the subscriber drain.
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	mu.Lock()
	count := len(activations)
	mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 activation across two scans, got %d: %v", count, activations)
	}
}

func TestBackfillScan_BotReturnedToAvailable_IsReactivatable(t *testing.T) {
	// After an activation, simulate the bot-runner finishing a match and
	// SADD-ing itself back. The next scan with the same lonely human must
	// re-activate it (i.e. the detector does not remember past activations).
	human := uuid.New()
	bot := uuid.New()

	rc := &ratingByPlayer{mmrs: map[string]float64{bot.String(): 1000}}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 3))

	past := time.Now().Add(-1 * time.Minute)
	enqueuePlayer(t, rdb, human, 1000, past)
	registerBot(t, rdb, bot)

	got1 := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got1 != bot.String() {
		t.Fatalf("first scan expected %s, got %q", bot, got1)
	}

	// Simulate bot completing its game and returning to the pool. The human
	// never left the queue (this mirrors production: the scan doesn't touch
	// queue:ranked — only FindAndPropose pairs players and clears it).
	if err := rdb.SAdd(context.Background(), keyBotAvailable, bot.String()).Err(); err != nil {
		t.Fatal(err)
	}

	got2 := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got2 != bot.String() {
		t.Errorf("second scan expected re-activation of %s, got %q", bot, got2)
	}
}

// ---------------------------------------------------------------------------
// Config defaults
// ---------------------------------------------------------------------------

func TestDefaultBackfillConfig_IsSafeByDefault(t *testing.T) {
	cfg := DefaultBackfillConfig()
	if cfg.Enabled {
		t.Error("backfill must be disabled by default")
	}
	if cfg.Threshold <= 0 {
		t.Errorf("threshold must be positive, got %v", cfg.Threshold)
	}
	if cfg.MaxActive <= 0 {
		t.Errorf("MaxActive must be positive, got %d", cfg.MaxActive)
	}
}

func TestSetBackfillConfig_Overrides(t *testing.T) {
	svc, _ := newQueueTestService(t, &ratingByPlayer{})
	if svc.backfillCfg.Enabled {
		t.Fatal("fresh service should not have backfill enabled")
	}
	svc.SetBackfillConfig(BackfillConfig{Enabled: true, Threshold: 42 * time.Second, MaxActive: 7})
	if !svc.backfillCfg.Enabled || svc.backfillCfg.Threshold != 42*time.Second || svc.backfillCfg.MaxActive != 7 {
		t.Errorf("SetBackfillConfig did not apply: %+v", svc.backfillCfg)
	}
}

// ---------------------------------------------------------------------------
// Misc invariants
// ---------------------------------------------------------------------------

func TestBackfillScan_InvalidBotUUIDInSet_IsSkipped(t *testing.T) {
	// A corrupt entry in bot:available should not panic or trip the rating
	// client — the invalid entry is simply skipped.
	human := uuid.New()
	good := uuid.New()

	rc := &ratingByPlayer{mmrs: map[string]float64{good.String(): 1000}}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 10))

	enqueuePlayer(t, rdb, human, 1000, time.Now().Add(-1*time.Minute))
	ctx := context.Background()
	// Register a real bot and a bogus one.
	if err := rdb.SAdd(ctx, keyBotKnown, good.String(), "not-a-uuid").Err(); err != nil {
		t.Fatal(err)
	}
	if err := rdb.SAdd(ctx, keyBotAvailable, good.String(), "not-a-uuid").Err(); err != nil {
		t.Fatal(err)
	}

	got := runScanWithSubscriber(t, svc, rdb, 500*time.Millisecond)
	if got != good.String() {
		t.Errorf("expected good bot %s, got %q", good, got)
	}
	if rc.callCount() != 1 {
		t.Errorf("expected 1 rating lookup (bogus skipped), got %d", rc.callCount())
	}
}

// Sanity check: the exported constant names line up with what documentation
// promises. If any of these values change, redis-keymap.md and bot-runner
// must be updated together — this test is the tripwire.
func TestBackfillScan_RedisNamesAreStable(t *testing.T) {
	cases := []struct {
		name string
		want string
		got  string
	}{
		{"keyBotKnown", "bot:known", keyBotKnown},
		{"keyBotAvailable", "bot:available", keyBotAvailable},
		{"ChannelBotActivate", "bot.activate", ChannelBotActivate},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q (downstream bot-runner / docs rely on this)", c.name, c.got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Regression guard: detector must tolerate a lot of idle bots without
// breaking the MMR-distance math (no NaN, no float overflow).
// ---------------------------------------------------------------------------

func TestBackfillScan_ManyBotsLargeMMRSpread_PicksClosest(t *testing.T) {
	human := uuid.New()
	// Target chosen so no bot MMR sits equidistant from it — avoids a
	// tie between two bots at which point iteration order (unordered
	// SMEMBERS) would decide the winner and flake the test.
	target := 1503.0

	// Seed 25 bots with MMRs spanning 0..3000 at 127-step intervals. 127 is
	// coprime with 2 and 5 so deltas stay distinct around the target.
	mmrs := make(map[string]float64)
	var expectedClosest uuid.UUID
	closestDelta := 999999.0
	for i := range 25 {
		id := uuid.New()
		mmr := float64(i * 127)
		mmrs[id.String()] = mmr
		delta := mmr - target
		if delta < 0 {
			delta = -delta
		}
		if delta < closestDelta {
			closestDelta = delta
			expectedClosest = id
		}
	}
	rc := &ratingByPlayer{mmrs: mmrs}
	svc, rdb, _ := newBackfillTestService(t, rc, testBackfillCfg(1*time.Second, 999))

	enqueuePlayer(t, rdb, human, target, time.Now().Add(-1*time.Minute))
	for id := range mmrs {
		u, _ := uuid.Parse(id)
		registerBot(t, rdb, u)
	}

	got := runScanWithSubscriber(t, svc, rdb, 1*time.Second)
	if got != expectedClosest.String() {
		t.Errorf("expected closest bot %s (delta=%.0f), got %q",
			expectedClosest, closestDelta, got)
	}
}

