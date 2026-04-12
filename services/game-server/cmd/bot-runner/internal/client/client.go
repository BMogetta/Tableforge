// Package client is the HTTP + WebSocket client the bot-runner uses to
// authenticate as a bot player, join ranked matchmaking, and submit moves.
//
// It is intentionally small and sync-per-bot — each bot owns one Client.
// Cookies from /auth/test-login are kept in an http.CookieJar and replayed on
// both HTTP requests and the WebSocket dial, so the ws-gateway sees the same
// session identity as the HTTP side.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Client is the per-bot API client. It is not safe for concurrent use by
// multiple goroutines; each bot goroutine owns its own Client.
type Client struct {
	baseURL  string
	wsURL    string
	playerID uuid.UUID
	http     *http.Client
}

// New constructs a Client for a single bot player.
//
// baseURL is the HTTP origin (e.g. http://localhost). The WS URL is derived
// by swapping the scheme (http→ws, https→wss). The cookie jar is fresh per
// Client — bots do not share session state.
func New(baseURL string, playerID uuid.UUID) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("bot-runner/client: cookie jar: %w", err)
	}

	baseURL = strings.TrimRight(baseURL, "/")
	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") // http→ws, https→wss

	return &Client{
		baseURL:  baseURL,
		wsURL:    wsURL,
		playerID: playerID,
		http: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
		},
	}, nil
}

// PlayerID returns the bot's player UUID.
func (c *Client) PlayerID() uuid.UUID { return c.playerID }

// Login calls /auth/test-login?player_id=... to obtain a session cookie.
// Only available when the auth-service runs with TEST_MODE=true.
func (c *Client) Login(ctx context.Context) error {
	u := fmt.Sprintf("%s/auth/test-login?player_id=%s", c.baseURL, c.playerID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("test-login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("test-login: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// JoinQueue issues POST /api/v1/queue. The response body is discarded —
// callers wait on the player WS for match_found instead of polling.
func (c *Client) JoinQueue(ctx context.Context) error {
	return c.postNoBody(ctx, "/api/v1/queue")
}

// AcceptMatch issues POST /api/v1/queue/accept with the given match ID.
func (c *Client) AcceptMatch(ctx context.Context, matchID string) error {
	body, _ := json.Marshal(map[string]string{"match_id": matchID})
	return c.postJSON(ctx, "/api/v1/queue/accept", body, http.StatusNoContent)
}

// Move submits a move to an active session. payload is the game-specific
// move payload that went through bot.DecideMove.
func (c *Client) Move(ctx context.Context, sessionID string, payload map[string]any) error {
	body, err := json.Marshal(map[string]any{"payload": payload})
	if err != nil {
		return fmt.Errorf("marshal move: %w", err)
	}
	return c.postJSON(ctx, "/api/v1/sessions/"+sessionID+"/move", body, http.StatusOK)
}

// MarkReady votes the bot ready on the session. The game only starts once all
// participants have voted; the ready timeout aborts the session if someone
// misses it. In Phase 1 the bot votes immediately after joining the room.
func (c *Client) MarkReady(ctx context.Context, sessionID string) error {
	return c.postNoBody(ctx, "/api/v1/sessions/"+sessionID+"/ready")
}

// GetSession returns the current session envelope (session + state + result)
// as raw JSON. Callers decode it into their local struct.
func (c *Client) GetSession(ctx context.Context, sessionID string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/sessions/"+sessionID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get session: status %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("get session: read body: %w", err)
	}
	return buf.Bytes(), nil
}

// DialPlayerWS opens a WebSocket to the player channel
// (/ws/players/{playerID}). Queue and match events arrive here.
func (c *Client) DialPlayerWS(ctx context.Context) (*websocket.Conn, error) {
	return c.dialWS(ctx, fmt.Sprintf("/ws/players/%s", c.playerID))
}

// DialRoomWS opens a WebSocket to the room channel (/ws/rooms/{roomID}).
// Game events (move_applied, game_over) arrive here.
func (c *Client) DialRoomWS(ctx context.Context, roomID string) (*websocket.Conn, error) {
	return c.dialWS(ctx, fmt.Sprintf("/ws/rooms/%s", roomID))
}

// --- internals --------------------------------------------------------------

func (c *Client) postNoBody(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, body []byte, wantStatus int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("%s: got status %d, want %d: %s", path, resp.StatusCode, wantStatus, strings.TrimSpace(buf.String()))
	}
	return nil
}

func (c *Client) dialWS(ctx context.Context, path string) (*websocket.Conn, error) {
	u, err := url.Parse(c.wsURL + path)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}

	// Replay cookies from the jar on the WS dial so ws-gateway sees the same
	// session the HTTP client authenticated.
	cookies := c.http.Jar.Cookies(&url.URL{
		Scheme: strings.Replace(u.Scheme, "ws", "http", 1),
		Host:   u.Host,
		Path:   "/",
	})
	header := http.Header{}
	for _, ck := range cookies {
		header.Add("Cookie", ck.String())
	}
	// Origin must match ALLOWED_ORIGINS on ws-gateway (default http://localhost
	// in compose). Gorilla's Dialer does not populate it automatically — a
	// missing Origin against a restrictive CheckOrigin yields 403.
	header.Set("Origin", c.baseURL)

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, fmt.Errorf("dial %s (status %d): %w", path, status, err)
	}
	return conn, nil
}
