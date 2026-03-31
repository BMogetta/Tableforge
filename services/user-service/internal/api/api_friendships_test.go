package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	sharedmw "github.com/tableforge/shared/middleware"
)

func TestSendFriendRequest(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	requester := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/friends/%s", requester, target)
	rec := postJSONAs(t, router, path, requester, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify friendship stored.
	if _, ok := st.friendships[friendKey(requester, target)]; !ok {
		t.Fatal("friendship not stored")
	}
}

func TestSendFriendRequest_SelfRequest(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/friends/%s", player, player)
	rec := postJSONAs(t, router, path, player, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAcceptFriendRequest(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	requester := uuid.New()
	addressee := uuid.New()

	// Seed a pending request.
	sendPath := fmt.Sprintf("/api/v1/players/%s/friends/%s", requester, addressee)
	postJSONAs(t, router, sendPath, requester, sharedmw.RolePlayer, nil)

	// Accept as the addressee.
	acceptPath := fmt.Sprintf("/api/v1/players/%s/friends/%s/accept", addressee, requester)
	rec := putJSONAs(t, router, acceptPath, addressee, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAcceptFriendRequest_NotFound(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	addressee := uuid.New()
	requester := uuid.New()

	// No pending request exists.
	path := fmt.Sprintf("/api/v1/players/%s/friends/%s/accept", addressee, requester)
	rec := putJSONAs(t, router, path, addressee, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeclineFriendRequest(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	requester := uuid.New()
	addressee := uuid.New()

	// Seed pending request.
	sendPath := fmt.Sprintf("/api/v1/players/%s/friends/%s", requester, addressee)
	postJSONAs(t, router, sendPath, requester, sharedmw.RolePlayer, nil)

	declinePath := fmt.Sprintf("/api/v1/players/%s/friends/%s/decline", addressee, requester)
	rec := deleteJSONAs(t, router, declinePath, addressee, sharedmw.RolePlayer)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRemoveFriend(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	requester := uuid.New()
	addressee := uuid.New()

	// Create and accept friendship.
	sendPath := fmt.Sprintf("/api/v1/players/%s/friends/%s", requester, addressee)
	postJSONAs(t, router, sendPath, requester, sharedmw.RolePlayer, nil)

	acceptPath := fmt.Sprintf("/api/v1/players/%s/friends/%s/accept", addressee, requester)
	putJSONAs(t, router, acceptPath, addressee, sharedmw.RolePlayer, nil)

	// Remove as requester.
	removePath := fmt.Sprintf("/api/v1/players/%s/friends/%s", requester, addressee)
	rec := deleteJSONAs(t, router, removePath, requester, sharedmw.RolePlayer)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBlockPlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	target := uuid.New()

	path := fmt.Sprintf("/api/v1/players/%s/block/%s", player, target)
	rec := postJSONAs(t, router, path, player, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBlockPlayer_SelfBlock(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	path := fmt.Sprintf("/api/v1/players/%s/block/%s", player, player)
	rec := postJSONAs(t, router, path, player, sharedmw.RolePlayer, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnblockPlayer(t *testing.T) {
	st := newMockStore()
	router := newTestRouter(st)

	player := uuid.New()
	target := uuid.New()

	// Block first.
	blockPath := fmt.Sprintf("/api/v1/players/%s/block/%s", player, target)
	postJSONAs(t, router, blockPath, player, sharedmw.RolePlayer, nil)

	// Unblock.
	unblockPath := fmt.Sprintf("/api/v1/players/%s/block/%s", player, target)
	rec := deleteJSONAs(t, router, unblockPath, player, sharedmw.RolePlayer)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}
