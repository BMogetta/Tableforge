package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/recess/shared/events"
)

type mockProcessor struct {
	called bool
	evt    events.GameSessionFinished
	err    error
}

func (m *mockProcessor) ProcessGameFinished(_ context.Context, evt events.GameSessionFinished) error {
	m.called = true
	m.evt = evt
	return m.err
}

func newTestConsumer(p *mockProcessor) *Consumer {
	return &Consumer{svc: p, log: slog.Default()}
}

func sessionFinishedJSON(sessionID, gameID, mode string) string {
	evt := events.GameSessionFinished{
		SessionID: sessionID,
		GameID:    gameID,
		Mode:      mode,
		EndedBy:   "win",
		WinnerID:  uuid.NewString(),
		Players: []events.SessionPlayer{
			{PlayerID: uuid.NewString(), Seat: 0, Outcome: "win"},
			{PlayerID: uuid.NewString(), Seat: 1, Outcome: "loss"},
		},
	}
	b, _ := json.Marshal(evt)
	return string(b)
}

func TestHandle_ValidGameSessionFinished(t *testing.T) {
	mock := &mockProcessor{}
	c := newTestConsumer(mock)

	sessionID := uuid.NewString()
	msg := &redis.Message{
		Channel: channelGameSessionFinished,
		Payload: sessionFinishedJSON(sessionID, "tictactoe", "ranked"),
	}

	if err := c.handle(context.Background(), msg); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !mock.called {
		t.Fatal("expected ProcessGameFinished to be called")
	}
	if mock.evt.SessionID != sessionID {
		t.Errorf("expected session_id %s, got %s", sessionID, mock.evt.SessionID)
	}
	if mock.evt.GameID != "tictactoe" {
		t.Errorf("expected game_id tictactoe, got %s", mock.evt.GameID)
	}
	if mock.evt.Mode != "ranked" {
		t.Errorf("expected mode ranked, got %s", mock.evt.Mode)
	}
}

func TestHandle_InvalidJSON(t *testing.T) {
	c := newTestConsumer(&mockProcessor{})

	msg := &redis.Message{Channel: channelGameSessionFinished, Payload: "not json"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestHandle_ProcessError(t *testing.T) {
	mock := &mockProcessor{err: errors.New("db error")}
	c := newTestConsumer(mock)

	msg := &redis.Message{
		Channel: channelGameSessionFinished,
		Payload: sessionFinishedJSON(uuid.NewString(), "tictactoe", "ranked"),
	}

	err := c.handle(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if !errors.Is(err, mock.err) {
		t.Errorf("expected wrapped db error, got %v", err)
	}
}

func TestHandle_UnknownChannel(t *testing.T) {
	c := newTestConsumer(&mockProcessor{})

	msg := &redis.Message{Channel: "unknown.channel", Payload: "{}"}
	if err := c.handle(context.Background(), msg); err == nil {
		t.Fatal("expected error for unknown channel")
	}
}
