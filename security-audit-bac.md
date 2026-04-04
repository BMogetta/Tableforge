# Security Audit — Broken Access Control

**Date:** 2026-04-03
**Scope:** All 8 backend services (HTTP + WebSocket + gRPC)
**Methodology:** Static code review of routers, middleware, and handlers

---

## Critical

### 1. ~~WS-Gateway: Player ID spoofing on WebSocket room connection~~ — FIXED

RoomHandler now uses `PlayerIDFromContext(r.Context())` instead of query param. Frontend no longer sends `player_id` in WS URL.

### 2. ~~Chat-Service: Unauthorized room message read~~ — FIXED (`b7b7879`)

### 3. ~~Chat-Service: Unauthorized DM read marking~~ — FIXED (`b7b7879`)

### 4. ~~User-Service: Settings IDOR (read + write)~~ — FIXED (`b7b7879`)

---

## High

### 5. Game-Server: Player data IDOR (sessions, stats, matches) — PENDING DECISION

**Endpoints:** `GET /players/{playerID}/sessions`, `/stats`, `/matches`

No ownership check. **Need to decide:** are these intentionally public (multiplayer game, opponents can see stats)? If yes, document with code comment. If no, add ownership check.

### 6. ~~User-Service: Friend list IDOR~~ — FIXED (`b7b7879`)

### 7. User-Service: JWT role trusted without DB verification — PENDING

Tied to JWT revocation / refresh token architecture. Will be addressed together.

---

## Medium

### 8. ~~User-Service: Achievements IDOR~~ — RESOLVED (`b7b7879`, documented as public)

### 9. ~~Chat-Service: Report endpoints~~ — FIXED (`b7b7879`)

### 10. ~~User-Service: Profile read~~ — RESOLVED (`b7b7879`, documented as public)

---

## Redundant `player_id` in request bodies — PARTIALLY FIXED

Removed from 9 of 12 endpoints (join, leave, start, settings, move, surrender, rematch, pause, resume). 3 remaining (`create_room`, `add_bot`, `remove_bot`) still use `req.PlayerID` in the handler — need migration to `PlayerIDFromContext` first.
