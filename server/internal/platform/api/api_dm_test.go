package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tableforge/server/internal/platform/store"
)

// --- POST /players/{playerID}/dm ---------------------------------------------

func TestSendDM(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "hey bob",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var msg store.DirectMessage
	json.NewDecoder(w.Body).Decode(&msg)
	if msg.Content != "hey bob" {
		t.Errorf("expected content 'hey bob', got %q", msg.Content)
	}
	if msg.SenderID != alice.ID {
		t.Errorf("expected sender_id %s, got %s", alice.ID, msg.SenderID)
	}
	if msg.ReceiverID != bob.ID {
		t.Errorf("expected receiver_id %s, got %s", bob.ID, msg.ReceiverID)
	}
}

func TestSendDM_EmptyContent(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSendDM_SelfMessage(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := postJSON(t, router, "/api/v1/players/"+alice.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "talking to myself",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSendDM_InvalidReceiverID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/players/not-a-uuid/dm", map[string]string{
		"player_id": "also-not-a-uuid",
		"content":   "hello",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSendDM_InvalidSenderID(t *testing.T) {
	router, s := newTestRouter(t)
	bob, _ := s.CreatePlayer(context.Background(), "bob")

	w := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": "not-a-uuid",
		"content":   "hello",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- GET /players/{playerID}/dm/{otherPlayerID} ------------------------------

func TestGetDMHistory_Empty(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String(),
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var messages []store.DirectMessage
	json.NewDecoder(w.Body).Decode(&messages)
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestGetDMHistory_AfterSend(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "first",
	})
	postJSON(t, router, "/api/v1/players/"+alice.ID.String()+"/dm", map[string]string{
		"player_id": bob.ID.String(),
		"content":   "second",
	})

	// Either participant can fetch the history.
	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String(),
		map[string]string{"player_id": bob.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var messages []store.DirectMessage
	json.NewDecoder(w.Body).Decode(&messages)
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestGetDMHistory_Forbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")
	charlie, _ := s.CreatePlayer(ctx, "charlie")

	// Charlie is not part of the alice-bob conversation.
	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String(),
		map[string]string{"player_id": charlie.ID.String()},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetDMHistory_InvalidPlayerID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSONWithBody(t, router,
		"/api/v1/players/not-a-uuid/dm/also-not-a-uuid",
		map[string]string{"player_id": "x"},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetDMHistory_InvalidOtherPlayerID(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/not-a-uuid",
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- GET /players/{playerID}/dm/unread ---------------------------------------

func TestGetUnreadDMCount_Zero(t *testing.T) {
	router, s := newTestRouter(t)
	alice, _ := s.CreatePlayer(context.Background(), "alice")

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/unread",
		map[string]string{"player_id": alice.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]int
	json.NewDecoder(w.Body).Decode(&result)
	if result["count"] != 0 {
		t.Errorf("expected count=0, got %d", result["count"])
	}
}

func TestGetUnreadDMCount_AfterSend(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "hey",
	})

	w := getJSONWithBody(t, router,
		"/api/v1/players/"+bob.ID.String()+"/dm/unread",
		map[string]string{"player_id": bob.ID.String()},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]int
	json.NewDecoder(w.Body).Decode(&result)
	if result["count"] != 1 {
		t.Errorf("expected count=1, got %d", result["count"])
	}
}

func TestGetUnreadDMCount_Forbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	// Bob tries to read Alice's unread count.
	w := getJSONWithBody(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/unread",
		map[string]string{"player_id": bob.ID.String()},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUnreadDMCount_InvalidPlayerID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := getJSONWithBody(t, router,
		"/api/v1/players/not-a-uuid/dm/unread",
		map[string]string{"player_id": "x"},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- POST /dm/{messageID}/read -----------------------------------------------

func TestMarkDMRead(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	sendResp := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "hey",
	})
	var msg store.DirectMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	w := postJSON(t, router, "/api/v1/dm/"+msg.ID.String()+"/read", map[string]string{
		"player_id": bob.ID.String(),
	})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Unread count should now be zero.
	unreadResp := getJSONWithBody(t, router,
		"/api/v1/players/"+bob.ID.String()+"/dm/unread",
		map[string]string{"player_id": bob.ID.String()},
	)
	var result map[string]int
	json.NewDecoder(unreadResp.Body).Decode(&result)
	if result["count"] != 0 {
		t.Errorf("expected count=0 after read, got %d", result["count"])
	}
}

func TestMarkDMRead_Idempotent(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	sendResp := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "hey",
	})
	var msg store.DirectMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	postJSON(t, router, "/api/v1/dm/"+msg.ID.String()+"/read", map[string]string{
		"player_id": bob.ID.String(),
	})
	w := postJSON(t, router, "/api/v1/dm/"+msg.ID.String()+"/read", map[string]string{
		"player_id": bob.ID.String(),
	})

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on second read, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarkDMRead_InvalidMessageID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router, "/api/v1/dm/not-a-uuid/read", map[string]string{
		"player_id": "also-not-a-uuid",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- POST /players/{playerID}/dm/{otherPlayerID}/report ----------------------

func TestReportDM(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	sendResp := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "offensive message",
	})
	var msg store.DirectMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	w := postJSON(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String()+"/report",
		map[string]string{
			"player_id":  bob.ID.String(),
			"message_id": msg.ID.String(),
		},
	)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReportDM_Forbidden(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")
	charlie, _ := s.CreatePlayer(ctx, "charlie")

	sendResp := postJSON(t, router, "/api/v1/players/"+bob.ID.String()+"/dm", map[string]string{
		"player_id": alice.ID.String(),
		"content":   "message",
	})
	var msg store.DirectMessage
	json.NewDecoder(sendResp.Body).Decode(&msg)

	// Charlie is not part of the conversation.
	w := postJSON(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String()+"/report",
		map[string]string{
			"player_id":  charlie.ID.String(),
			"message_id": msg.ID.String(),
		},
	)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReportDM_InvalidMessageID(t *testing.T) {
	router, s := newTestRouter(t)
	ctx := context.Background()
	alice, _ := s.CreatePlayer(ctx, "alice")
	bob, _ := s.CreatePlayer(ctx, "bob")

	w := postJSON(t, router,
		"/api/v1/players/"+alice.ID.String()+"/dm/"+bob.ID.String()+"/report",
		map[string]string{
			"player_id":  alice.ID.String(),
			"message_id": "not-a-uuid",
		},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReportDM_InvalidPlayerID(t *testing.T) {
	router, _ := newTestRouter(t)

	w := postJSON(t, router,
		"/api/v1/players/not-a-uuid/dm/also-not-a-uuid/report",
		map[string]string{
			"player_id":  "x",
			"message_id": "y",
		},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
