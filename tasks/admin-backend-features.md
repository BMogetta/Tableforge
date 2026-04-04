# Admin Backend — Missing Features

## Why
Three admin capabilities don't exist at all (no backend, no frontend).
These are needed for production moderation.

## Features to build

### 1. Audit Logs

Track all admin actions for accountability and forensics.

#### Database
New migration in `shared/db/migrations/`:
```sql
CREATE TABLE audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id UUID NOT NULL REFERENCES players(id),
  action TEXT NOT NULL,        -- 'ban_issued', 'ban_lifted', 'role_changed', etc.
  target_type TEXT NOT NULL,   -- 'player', 'room', 'session', 'email'
  target_id TEXT NOT NULL,     -- UUID or email depending on target_type
  details JSONB,               -- action-specific metadata
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_id);
CREATE INDEX idx_audit_logs_created ON audit_logs(created_at DESC);
```

#### Backend (user-service)
- Add `audit.Store` with `Log(ctx, actorID, action, targetType, targetID, details)` method
- Add `GET /api/v1/admin/audit-logs?limit=50&offset=0` endpoint (Manager role)
- Filter by: actor, action, target, date range
- Wire audit logging into ALL admin handlers:
  - `banPlayer` → log `ban_issued`
  - `liftBan` → log `ban_lifted`
  - `setRole` → log `role_changed`
  - `addAllowedEmail` → log `email_added`
  - `removeAllowedEmail` → log `email_removed`
  - `reviewReport` → log `report_reviewed`
  - `forceEndSession` → log `session_force_ended` (game-server, needs
    cross-service logging — either gRPC call or Redis event)

#### Frontend
- New "Audit Log" tab in admin panel
- Table: timestamp, admin, action, target, details
- Filter by action type and date range
- Paginated (50 per page)

#### Tests
- Go: store tests for CRUD, handler tests for each logged action
- Frontend: component test for audit log table

### 2. System Stats Dashboard

Real-time overview of the platform for admins.

#### Backend
New endpoints in game-server (or a new stats aggregation):
- `GET /api/v1/admin/stats` — returns:
  ```json
  {
    "online_players": 12,
    "active_rooms": 5,
    "active_sessions": 3,
    "total_players": 142,
    "total_sessions_today": 28,
    "queue_size": 2
  }
  ```
- Implementation: direct DB counts + Redis presence count + match-service
  queue length
- Consider caching (Redis key with 10s TTL) to avoid hammering DB

#### Frontend
- Stats cards at the top of the admin page (above tabs)
- Auto-refresh every 30s
- Simple counters with labels, no charts needed for MVP

#### Tests
- Go: handler test with mock store
- Frontend: component test with mock data

### 3. Admin Broadcast Messages

Send a notification to all online players (maintenance warning, announcements).

#### Backend
- `POST /api/v1/admin/broadcast` — body: `{ "message": "...", "type": "info|warning" }`
- Implementation options:
  - **Option A**: Create a notification for every player in DB (expensive,
    persistent, guaranteed delivery)
  - **Option B**: Publish to a Redis `ws:broadcast` channel, ws-gateway
    fans out to all connected clients (cheap, real-time, no persistence —
    only online players see it)
  - **Recommended**: Option B for MVP, add Option A later for persistent
    announcements

#### Frontend
- "Broadcast" button in admin header → dialog with message input + type selector
- Players see broadcast as a toast or banner
- ws-gateway needs to subscribe to `ws:broadcast` channel and fan out

#### Tests
- Go: handler test
- Frontend: component test for broadcast dialog
- E2E: admin sends broadcast → player sees toast

## Files per feature

### Audit Logs
- `shared/db/migrations/XXX_audit_logs.sql` (new)
- `services/user-service/internal/store/store_audit.go` (new)
- `services/user-service/internal/api/api_admin.go` (modify — add logging calls)
- `services/user-service/internal/api/api_audit.go` (new — list endpoint)
- `frontend/src/features/admin/components/AuditLogTab.tsx` (new)
- `frontend/src/features/admin/__tests__/AuditLogTab.test.tsx` (new)

### System Stats
- `services/game-server/internal/platform/api/api_admin.go` (new)
- `services/game-server/internal/platform/store/` (add count queries)
- `frontend/src/features/admin/components/StatsCards.tsx` (new)
- `frontend/src/features/admin/__tests__/StatsCards.test.tsx` (new)

### Broadcast
- `services/ws-gateway/internal/handler/handler.go` (add broadcast handler)
- `services/ws-gateway/cmd/server/main.go` (add broadcast route)
- `shared/ws/ws.go` (add broadcast channel key)
- `frontend/src/features/admin/components/BroadcastDialog.tsx` (new)
- `frontend/src/features/admin/__tests__/BroadcastDialog.test.tsx` (new)
- `frontend/src/lib/ws.ts` (handle broadcast event type)

## Execution order
1. Audit Logs first (most important for security)
2. System Stats (quick win, useful for monitoring)
3. Broadcast (nice to have)

## Dependencies
- Admin Panel UI task (`admin-panel-ui.md`) should be done first — it
  restructures the admin feature folder
- Audit logs touch user-service admin handlers — coordinate with any other
  task that modifies those files
