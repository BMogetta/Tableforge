package adapter_test

import (
	"strings"
	"testing"

	"github.com/recess/game-server/internal/bot"
	"github.com/recess/game-server/internal/bot/adapter"
	"github.com/recess/game-server/internal/domain/engine"
)

type fakeAdapter struct {
	profile bot.PersonalityProfile
}

func (f *fakeAdapter) ValidMoves(bot.BotGameState) []bot.BotMove { return nil }
func (f *fakeAdapter) ApplyMove(s bot.BotGameState, _ bot.BotMove) bot.BotGameState {
	return s
}
func (f *fakeAdapter) IsTerminal(bot.BotGameState) bool                 { return true }
func (f *fakeAdapter) Result(bot.BotGameState, engine.PlayerID) float64 { return 0 }
func (f *fakeAdapter) CurrentPlayer(s bot.BotGameState) engine.PlayerID {
	return s.EngineState().CurrentPlayerID
}
func (f *fakeAdapter) Determinize(s bot.BotGameState, _ engine.PlayerID) bot.BotGameState {
	return s
}
func (f *fakeAdapter) RolloutPolicy(_ bot.BotGameState, moves []bot.BotMove) bot.BotMove {
	if len(moves) == 0 {
		return bot.BotMove("")
	}
	return moves[0]
}

func TestRegistry_NewReturnsRegistered(t *testing.T) {
	adapter.Register("audit-game-a", adapter.Factory{
		New: func() bot.BotAdapter { return &fakeAdapter{} },
	})

	got, err := adapter.New("audit-game-a")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got == nil {
		t.Fatal("New returned nil adapter")
	}
}

func TestRegistry_UnknownGameReturnsError(t *testing.T) {
	_, err := adapter.New("never-registered")
	if err == nil {
		t.Fatal("expected error for unregistered game")
	}
}

func TestRegistry_NewWithProfileUsesFactory(t *testing.T) {
	var captured bot.PersonalityProfile
	adapter.Register("audit-game-b", adapter.Factory{
		New: func() bot.BotAdapter { return &fakeAdapter{} },
		NewWithProfile: func(p bot.PersonalityProfile) bot.BotAdapter {
			captured = p
			return &fakeAdapter{profile: p}
		},
	})

	profile := bot.PersonalityProfile{Aggressiveness: 0.7, RiskAversion: 0.3}
	got, err := adapter.NewWithProfile("audit-game-b", profile)
	if err != nil {
		t.Fatalf("NewWithProfile: %v", err)
	}
	if captured.Aggressiveness != 0.7 || captured.RiskAversion != 0.3 {
		t.Errorf("profile not passed to factory: got %+v", captured)
	}
	f, ok := got.(*fakeAdapter)
	if !ok {
		t.Fatalf("expected *fakeAdapter, got %T", got)
	}
	if f.profile != profile {
		t.Errorf("returned adapter has wrong profile: %+v", f.profile)
	}
}

func TestRegistry_NewWithProfileFallsBackToNew(t *testing.T) {
	adapter.Register("audit-game-c", adapter.Factory{
		New: func() bot.BotAdapter { return &fakeAdapter{} },
	})

	got, err := adapter.NewWithProfile("audit-game-c", bot.PersonalityProfile{Aggressiveness: 1})
	if err != nil {
		t.Fatalf("NewWithProfile: %v", err)
	}
	if got == nil {
		t.Fatal("fallback returned nil adapter")
	}
}

func TestRegistry_DuplicateRegistrationPanics(t *testing.T) {
	adapter.Register("audit-game-d", adapter.Factory{
		New: func() bot.BotAdapter { return &fakeAdapter{} },
	})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	adapter.Register("audit-game-d", adapter.Factory{
		New: func() bot.BotAdapter { return &fakeAdapter{} },
	})
}

func TestRegistry_RegisterNilNewPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Factory.New is nil")
		}
	}()
	adapter.Register("audit-game-e", adapter.Factory{})
}

// Error message for an unknown ID must mention the ID so callers can debug
// missing blank imports without reading registry internals.
func TestRegistry_UnknownErrorMessageContainsID(t *testing.T) {
	_, err := adapter.New("audit-game-missing")
	if err == nil {
		t.Fatal("expected error for unknown game")
	}
	if !strings.Contains(err.Error(), "audit-game-missing") {
		t.Errorf("expected error to include game id, got %q", err.Error())
	}
}
