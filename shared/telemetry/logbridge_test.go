package telemetry

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// captureHandler records every call so tests can assert on fan-out behaviour.
type captureHandler struct {
	name      string
	enabled   bool
	returnErr error
	records   []slog.Record
	attrs     []slog.Attr
	group     string
}

func (c *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return c.enabled }

func (c *captureHandler) Handle(_ context.Context, r slog.Record) error {
	c.records = append(c.records, r)
	return c.returnErr
}

func (c *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append([]slog.Attr{}, c.attrs...)
	newAttrs = append(newAttrs, attrs...)
	clone := *c
	clone.attrs = newAttrs
	return &clone
}

func (c *captureHandler) WithGroup(name string) slog.Handler {
	clone := *c
	clone.group = name
	return &clone
}

func TestMultiHandler_EnabledIsOrFold(t *testing.T) {
	cases := []struct {
		name     string
		children []bool
		want     bool
	}{
		{"all disabled", []bool{false, false}, false},
		{"one enabled", []bool{false, true}, true},
		{"all enabled", []bool{true, true}, true},
		{"no children", nil, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			children := make([]slog.Handler, len(tc.children))
			for i, en := range tc.children {
				children[i] = &captureHandler{enabled: en}
			}
			h := NewMultiHandler(children...)
			if got := h.Enabled(context.Background(), slog.LevelInfo); got != tc.want {
				t.Fatalf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMultiHandler_HandleFansOutToEveryEnabledChild(t *testing.T) {
	a := &captureHandler{name: "a", enabled: true}
	b := &captureHandler{name: "b", enabled: true}
	h := NewMultiHandler(a, b)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	r.AddAttrs(slog.String("k", "v"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(a.records) != 1 || len(b.records) != 1 {
		t.Fatalf("both children should receive the record, got a=%d b=%d", len(a.records), len(b.records))
	}
	if a.records[0].Message != "hello" || b.records[0].Message != "hello" {
		t.Fatalf("record message lost in fan-out")
	}
}

func TestMultiHandler_HandleSkipsDisabledChildren(t *testing.T) {
	on := &captureHandler{enabled: true}
	off := &captureHandler{enabled: false}
	h := NewMultiHandler(on, off)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(on.records) != 1 {
		t.Fatalf("enabled child should receive record, got %d", len(on.records))
	}
	if len(off.records) != 0 {
		t.Fatalf("disabled child should be skipped, got %d", len(off.records))
	}
}

func TestMultiHandler_HandleContinuesAfterChildError(t *testing.T) {
	// The critical case from 5.4.f: the OTel exporter might error, but the
	// stdout handler must still receive the record so kubectl logs shows it.
	sentinel := errors.New("exporter down")
	broken := &captureHandler{enabled: true, returnErr: sentinel}
	healthy := &captureHandler{enabled: true}
	h := NewMultiHandler(broken, healthy)

	r := slog.NewRecord(time.Now(), slog.LevelError, "db down", 0)
	err := h.Handle(context.Background(), r)

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected first child's error returned, got %v", err)
	}
	if len(healthy.records) != 1 {
		t.Fatalf("healthy child must still receive the record despite broken sibling, got %d", len(healthy.records))
	}
}

func TestMultiHandler_HandleUsesIndependentRecordPerChild(t *testing.T) {
	// Handle calls r.Clone() per child. Mutating one child's record (via
	// AddAttrs in its Handle) must not affect what the other child sees.
	recordingA := &captureHandler{enabled: true}
	recordingB := &captureHandler{enabled: true}
	h := NewMultiHandler(recordingA, recordingB)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "parent", 0)
	r.AddAttrs(slog.String("shared", "1"))

	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mutate the record captured by A and verify B's copy is untouched.
	recordingA.records[0].AddAttrs(slog.String("a-only", "x"))

	var aAttrs, bAttrs []string
	recordingA.records[0].Attrs(func(attr slog.Attr) bool {
		aAttrs = append(aAttrs, attr.Key)
		return true
	})
	recordingB.records[0].Attrs(func(attr slog.Attr) bool {
		bAttrs = append(bAttrs, attr.Key)
		return true
	})

	if len(aAttrs) != 2 {
		t.Fatalf("A should have shared + a-only, got %v", aAttrs)
	}
	if len(bAttrs) != 1 || bAttrs[0] != "shared" {
		t.Fatalf("B must not see A's mutation, got %v", bAttrs)
	}
}

func TestMultiHandler_WithAttrsPropagatesToAllChildren(t *testing.T) {
	a := &captureHandler{enabled: true}
	b := &captureHandler{enabled: true}
	h := NewMultiHandler(a, b)

	derived := h.WithAttrs([]slog.Attr{slog.String("svc", "auth-service")})

	// The originals must not have changed (WithAttrs returns a new handler).
	if len(a.attrs) != 0 || len(b.attrs) != 0 {
		t.Fatalf("original children must not be mutated")
	}

	// Emit via the derived handler and inspect which children got the attr.
	// captureHandler clones itself on WithAttrs, so walk the returned multi.
	mh, ok := derived.(*multiHandler)
	if !ok {
		t.Fatalf("WithAttrs did not return a *multiHandler")
	}
	for i, child := range mh.handlers {
		ch, ok := child.(*captureHandler)
		if !ok {
			t.Fatalf("child %d is not *captureHandler", i)
		}
		if len(ch.attrs) != 1 || ch.attrs[0].Key != "svc" {
			t.Fatalf("child %d did not receive the attr: %v", i, ch.attrs)
		}
	}
}

func TestMultiHandler_WithGroupPropagatesToAllChildren(t *testing.T) {
	a := &captureHandler{enabled: true}
	b := &captureHandler{enabled: true}
	h := NewMultiHandler(a, b)

	derived := h.WithGroup("http")

	mh, ok := derived.(*multiHandler)
	if !ok {
		t.Fatalf("WithGroup did not return a *multiHandler")
	}
	for i, child := range mh.handlers {
		ch, ok := child.(*captureHandler)
		if !ok {
			t.Fatalf("child %d is not *captureHandler", i)
		}
		if ch.group != "http" {
			t.Fatalf("child %d did not receive the group, got %q", i, ch.group)
		}
	}
}

func TestMultiHandler_IntegratesWithTextHandler(t *testing.T) {
	// Canary scenario: text handler to a buffer + a broken child. Even if the
	// broken child errors, the text buffer must contain the message. This is
	// exactly what the 5.4.f fix protects against.
	var buf bytes.Buffer
	text := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	broken := &captureHandler{enabled: true, returnErr: errors.New("boom")}

	logger := slog.New(NewMultiHandler(text, broken))
	logger.Error("startup failed", "reason", "redis parse")

	if !bytes.Contains(buf.Bytes(), []byte("startup failed")) {
		t.Fatalf("text handler did not receive the record despite broken sibling; buf=%q", buf.String())
	}
	if !bytes.Contains(buf.Bytes(), []byte("redis parse")) {
		t.Fatalf("record attrs missing from text output; buf=%q", buf.String())
	}
}
