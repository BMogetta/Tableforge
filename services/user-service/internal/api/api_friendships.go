package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/recess/services/user-service/internal/store"
	sharedmw "github.com/recess/shared/middleware"
)

// GET /api/v1/players/search?username=xxx
func handleSearchPlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			writeError(w, http.StatusBadRequest, "username query param required")
			return
		}

		player, err := st.FindPlayerByUsername(r.Context(), username)
		if err != nil {
			writeError(w, http.StatusNotFound, "player not found")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"id":         player.ID,
			"username":   player.Username,
			"avatar_url": player.AvatarURL,
		})
	}
}

func handleListFriends(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		friends, err := st.ListFriends(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list friends")
			return
		}
		writeJSON(w, http.StatusOK, friends)
	}
}

func handleListPendingRequests(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		pending, err := st.ListPendingFriendRequests(r.Context(), playerID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list pending requests")
			return
		}
		writeJSON(w, http.StatusOK, pending)
	}
}

func handleSendFriendRequest(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requesterID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		targetID, err := uuid.Parse(chi.URLParam(r, "targetID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid target id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != requesterID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if requesterID == targetID {
			writeError(w, http.StatusBadRequest, "cannot send friend request to yourself")
			return
		}
		friendship, err := st.SendFriendRequest(r.Context(), requesterID, targetID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to send friend request")
			return
		}
		username, _ := sharedmw.UsernameFromContext(r.Context())
		pub.PublishFriendshipRequested(r.Context(), requesterID.String(), username, targetID.String())
		writeJSON(w, http.StatusCreated, friendship)
	}
}

func handleAcceptFriendRequest(st store.Store, pub *Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addresseeID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		requesterID, err := uuid.Parse(chi.URLParam(r, "requesterID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid requester id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != addresseeID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		friendship, err := st.AcceptFriendRequest(r.Context(), requesterID, addresseeID)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "friend request not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to accept friend request")
			return
		}
		pub.PublishFriendshipAccepted(r.Context(), friendship)
		writeJSON(w, http.StatusOK, friendship)
	}
}

func handleDeclineFriendRequest(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		addresseeID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		requesterID, err := uuid.Parse(chi.URLParam(r, "requesterID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid requester id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != addresseeID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if err := st.DeclineFriendRequest(r.Context(), requesterID, addresseeID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "friend request not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to decline friend request")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleRemoveFriend(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		friendID, err := uuid.Parse(chi.URLParam(r, "friendID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid friend id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if err := st.RemoveFriend(r.Context(), playerID, friendID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "friendship not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to remove friend")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleBlockPlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		targetID, err := uuid.Parse(chi.URLParam(r, "targetID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid target id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if playerID == targetID {
			writeError(w, http.StatusBadRequest, "cannot block yourself")
			return
		}
		friendship, err := st.BlockPlayer(r.Context(), playerID, targetID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to block player")
			return
		}
		writeJSON(w, http.StatusOK, friendship)
	}
}

func handleUnblockPlayer(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, err := uuid.Parse(chi.URLParam(r, "playerID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid player id")
			return
		}
		targetID, err := uuid.Parse(chi.URLParam(r, "targetID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid target id")
			return
		}
		callerID, _ := sharedmw.PlayerIDFromContext(r.Context())
		if callerID != playerID {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if err := st.UnblockPlayer(r.Context(), playerID, targetID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusNotFound, "block not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to unblock player")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
