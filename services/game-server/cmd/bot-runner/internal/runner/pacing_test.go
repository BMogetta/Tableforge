package runner

import (
	"context"
	"testing"
	"time"

	botpkg "github.com/recess/game-server/internal/bot"
)

func TestPickThinkTime_DisabledWhenMinZero(t *testing.T) {
	got := pickThinkTime(botpkg.PersonalityProfile{MinMoveDelay: 0, MoveDelayJitter: 500 * time.Millisecond})
	if got != 0 {
		t.Errorf("MinMoveDelay=0 should disable pacing, got %v", got)
	}
}

func TestPickThinkTime_DeterministicWhenJitterZero(t *testing.T) {
	for range 10 {
		got := pickThinkTime(botpkg.PersonalityProfile{MinMoveDelay: 800 * time.Millisecond})
		if got != 800*time.Millisecond {
			t.Errorf("zero jitter should return min exactly, got %v", got)
		}
	}
}

func TestPickThinkTime_InRange(t *testing.T) {
	min := 500 * time.Millisecond
	jitter := 400 * time.Millisecond
	p := botpkg.PersonalityProfile{MinMoveDelay: min, MoveDelayJitter: jitter}

	// Sample many draws to exercise the jitter distribution.
	var sawMin, sawAboveMid bool
	for range 200 {
		got := pickThinkTime(p)
		if got < min || got >= min+jitter {
			t.Fatalf("draw %v out of [%v, %v)", got, min, min+jitter)
		}
		if got == min {
			sawMin = true
		}
		if got > min+jitter/2 {
			sawAboveMid = true
		}
	}
	// Not strictly required, but a reasonable sanity check that the jitter
	// covers the range rather than clustering on min. With 200 draws on a
	// 400ms window the chance of missing the upper half is ~1e-60.
	if !sawAboveMid {
		t.Error("never sampled above the jitter midpoint — random source degenerate?")
	}
	// sawMin is probabilistic; keep the assertion weak.
	_ = sawMin
}

// TestPickThinkTime_ProfilesAreOrdered codifies the design intent that
// aggressive < easy < medium < hard in response time — surprise reorderings
// should force a conscious update to this test.
func TestPickThinkTime_ProfilesAreOrdered(t *testing.T) {
	profs := botpkg.Profiles
	if profs["aggressive"].MinMoveDelay >= profs["easy"].MinMoveDelay {
		t.Errorf("aggressive should be snappier than easy, got aggressive=%v easy=%v",
			profs["aggressive"].MinMoveDelay, profs["easy"].MinMoveDelay)
	}
	if profs["easy"].MinMoveDelay >= profs["medium"].MinMoveDelay {
		t.Errorf("medium should be slower than easy, got easy=%v medium=%v",
			profs["easy"].MinMoveDelay, profs["medium"].MinMoveDelay)
	}
	if profs["medium"].MinMoveDelay >= profs["hard"].MinMoveDelay {
		t.Errorf("hard should be slower than medium, got medium=%v hard=%v",
			profs["medium"].MinMoveDelay, profs["hard"].MinMoveDelay)
	}
}

func TestSleepCtx_WaitsFullDuration(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(context.Background(), 50*time.Millisecond); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("returned early: %v", elapsed)
	}
}

func TestSleepCtx_ReturnsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- sleepCtx(ctx, 5*time.Second)
	}()

	// Cancel after a short while and ensure sleepCtx unblocks promptly.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected ctx error, got nil")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sleepCtx did not return within 500ms of cancel")
	}
}

func TestSleepCtx_ZeroDuration_ReturnsImmediately(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(context.Background(), 0); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Errorf("zero duration should return instantly, took %v", elapsed)
	}
}
