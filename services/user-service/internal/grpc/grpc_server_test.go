package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	userv1 "github.com/recess/shared/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- mock store (only methods used by gRPC server) ---------------------------

type mockStore struct {
	store.Store // embed to satisfy interface; unused methods panic
	ban        *store.Ban
	friendship store.Friendship
	mutes      []store.PlayerMute
	acceptErr  error
}

func (m *mockStore) CheckActiveBan(_ context.Context, _ uuid.UUID) (*store.Ban, error) {
	return m.ban, nil
}
func (m *mockStore) GetFriendship(_ context.Context, _, _ uuid.UUID) (store.Friendship, error) {
	return m.friendship, nil
}
func (m *mockStore) GetMutedPlayers(_ context.Context, _ uuid.UUID) ([]store.PlayerMute, error) {
	return m.mutes, nil
}
func (m *mockStore) AcceptFriendRequest(_ context.Context, requesterID, addresseeID uuid.UUID) (store.Friendship, error) {
	if m.acceptErr != nil {
		return store.Friendship{}, m.acceptErr
	}
	return store.Friendship{
		RequesterID: requesterID,
		AddresseeID: addresseeID,
		Status:      store.FriendshipStatusAccepted,
	}, nil
}

// --- CheckBan ----------------------------------------------------------------

func TestCheckBan_NotBanned(t *testing.T) {
	s := NewServer(&mockStore{ban: nil})

	resp, err := s.CheckBan(context.Background(), &userv1.CheckBanRequest{
		PlayerId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("CheckBan: %v", err)
	}
	if resp.IsBanned {
		t.Error("expected not banned")
	}
}

func TestCheckBan_Banned(t *testing.T) {
	reason := "cheating"
	expiry := time.Now().Add(24 * time.Hour)
	s := NewServer(&mockStore{ban: &store.Ban{
		ID:        uuid.New(),
		Reason:    &reason,
		ExpiresAt: &expiry,
	}})

	resp, err := s.CheckBan(context.Background(), &userv1.CheckBanRequest{
		PlayerId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("CheckBan: %v", err)
	}
	if !resp.IsBanned {
		t.Error("expected banned")
	}
	if resp.Reason != "cheating" {
		t.Errorf("expected reason cheating, got %s", resp.Reason)
	}
	if resp.ExpiresAt == "" {
		t.Error("expected expires_at to be set")
	}
}

func TestCheckBan_BannedNoExpiry(t *testing.T) {
	reason := "permanent"
	s := NewServer(&mockStore{ban: &store.Ban{
		ID:     uuid.New(),
		Reason: &reason,
	}})

	resp, err := s.CheckBan(context.Background(), &userv1.CheckBanRequest{
		PlayerId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("CheckBan: %v", err)
	}
	if !resp.IsBanned {
		t.Error("expected banned")
	}
	if resp.ExpiresAt != "" {
		t.Error("expected empty expires_at for permanent ban")
	}
}

func TestCheckBan_InvalidPlayerID(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.CheckBan(context.Background(), &userv1.CheckBanRequest{
		PlayerId: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid player_id")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

// --- GetFriendship -----------------------------------------------------------

func TestGetFriendship_None(t *testing.T) {
	s := NewServer(&mockStore{friendship: store.Friendship{}})

	resp, err := s.GetFriendship(context.Background(), &userv1.GetFriendshipRequest{
		PlayerAId: uuid.NewString(),
		PlayerBId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("GetFriendship: %v", err)
	}
	if resp.Status != "none" {
		t.Errorf("expected none, got %s", resp.Status)
	}
}

func TestGetFriendship_Accepted(t *testing.T) {
	a := uuid.New()
	b := uuid.New()
	s := NewServer(&mockStore{friendship: store.Friendship{
		RequesterID: a,
		AddresseeID: b,
		Status:      store.FriendshipStatusAccepted,
	}})

	resp, err := s.GetFriendship(context.Background(), &userv1.GetFriendshipRequest{
		PlayerAId: a.String(),
		PlayerBId: b.String(),
	})
	if err != nil {
		t.Fatalf("GetFriendship: %v", err)
	}
	if resp.Status != "accepted" {
		t.Errorf("expected accepted, got %s", resp.Status)
	}
	if resp.BlockedBy != "" {
		t.Error("expected no blocked_by for accepted friendship")
	}
}

func TestGetFriendship_Blocked(t *testing.T) {
	blocker := uuid.New()
	s := NewServer(&mockStore{friendship: store.Friendship{
		RequesterID: blocker,
		AddresseeID: uuid.New(),
		Status:      store.FriendshipStatusBlocked,
	}})

	resp, err := s.GetFriendship(context.Background(), &userv1.GetFriendshipRequest{
		PlayerAId: blocker.String(),
		PlayerBId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("GetFriendship: %v", err)
	}
	if resp.Status != "blocked" {
		t.Errorf("expected blocked, got %s", resp.Status)
	}
	if resp.BlockedBy != blocker.String() {
		t.Errorf("expected blocked_by %s, got %s", blocker, resp.BlockedBy)
	}
}

func TestGetFriendship_InvalidPlayerA(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.GetFriendship(context.Background(), &userv1.GetFriendshipRequest{
		PlayerAId: "bad",
		PlayerBId: uuid.NewString(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestGetFriendship_InvalidPlayerB(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.GetFriendship(context.Background(), &userv1.GetFriendshipRequest{
		PlayerAId: uuid.NewString(),
		PlayerBId: "bad",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

// --- GetMutes ----------------------------------------------------------------

func TestGetMutes_Empty(t *testing.T) {
	s := NewServer(&mockStore{mutes: []store.PlayerMute{}})

	resp, err := s.GetMutes(context.Background(), &userv1.GetMutesRequest{
		PlayerId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("GetMutes: %v", err)
	}
	if len(resp.MutedPlayerIds) != 0 {
		t.Errorf("expected 0 mutes, got %d", len(resp.MutedPlayerIds))
	}
}

func TestGetMutes_WithMutes(t *testing.T) {
	muter := uuid.New()
	m1, m2 := uuid.New(), uuid.New()
	s := NewServer(&mockStore{mutes: []store.PlayerMute{
		{MuterID: muter, MutedID: m1},
		{MuterID: muter, MutedID: m2},
	}})

	resp, err := s.GetMutes(context.Background(), &userv1.GetMutesRequest{
		PlayerId: muter.String(),
	})
	if err != nil {
		t.Fatalf("GetMutes: %v", err)
	}
	if len(resp.MutedPlayerIds) != 2 {
		t.Fatalf("expected 2 mutes, got %d", len(resp.MutedPlayerIds))
	}
	if resp.MutedPlayerIds[0] != m1.String() {
		t.Errorf("expected %s, got %s", m1, resp.MutedPlayerIds[0])
	}
}

func TestGetMutes_InvalidPlayerID(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.GetMutes(context.Background(), &userv1.GetMutesRequest{
		PlayerId: "bad",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

// --- AcceptFriendRequest -----------------------------------------------------

func TestAcceptFriendRequest_Success(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.AcceptFriendRequest(context.Background(), &userv1.AcceptFriendRequestRequest{
		RequesterId: uuid.NewString(),
		AddresseeId: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("AcceptFriendRequest: %v", err)
	}
}

func TestAcceptFriendRequest_InvalidRequesterID(t *testing.T) {
	s := NewServer(&mockStore{})

	_, err := s.AcceptFriendRequest(context.Background(), &userv1.AcceptFriendRequestRequest{
		RequesterId: "bad",
		AddresseeId: uuid.NewString(),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", status.Code(err))
	}
}

func TestAcceptFriendRequest_StoreError(t *testing.T) {
	s := NewServer(&mockStore{acceptErr: context.DeadlineExceeded})

	_, err := s.AcceptFriendRequest(context.Background(), &userv1.AcceptFriendRequestRequest{
		RequesterId: uuid.NewString(),
		AddresseeId: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("expected error when store fails")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal, got %s", status.Code(err))
	}
}
