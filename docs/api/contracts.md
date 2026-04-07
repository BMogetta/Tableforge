# Service Contracts

This document summarizes how services communicate. The full contract map with versioning rules lives in `shared/contracts.md`.

## JSON Schema (REST DTOs)

JSON Schema is the **single source of truth** for all API request/response types shared between Go and TypeScript.

```
shared/schemas/*.json  -->  Go: embedded + validated at runtime (middleware)
                       -->  TS: Zod schemas + inferred types (generated)
```

### Schema File Conventions

- Endpoint schemas: `{verb}_{noun}.{request|response}.json` (e.g., `create_room.request.json`)
- Shared types: `defs/{type_name}.json` (e.g., `defs/game_session.json`)
- Title field: PascalCase (e.g., `"title": "CreateRoomRequest"`)
- References: `{ "$ref": "defs/game_session.json" }`
- No `DTO` suffix -- use domain names

### Adding a New Endpoint

1. Create `shared/schemas/{verb}_{noun}.request.json` and/or `.response.json`
2. Create shared types in `shared/schemas/defs/` if needed
3. Run `make gen-types` to regenerate `frontend/src/lib/schema-generated.zod.ts`
4. Backend: add `validate("{verb}_{noun}.request")` middleware in the router
5. Frontend: use `validatedRequest(schema, path, init)` for the response

### Validation Rules

- **Backend requests**: all POST/PUT/DELETE with a body must have a JSON Schema + `validate()` middleware. Returns 400 with structured errors on failure.
- **Frontend responses**: use `validatedRequest()` with the Zod schema. Throws `SchemaError` on mismatch (loud in dev, generic in production).
- **Frontend requests**: not validated at runtime. TypeScript enforces shape at compile time, backend validates at runtime.

## Protobuf (gRPC)

Proto definitions live in `shared/proto/`. Regenerate stubs with `make gen-proto`.

| Proto Package | Service | Methods |
|--------------|---------|---------|
| `rating.v1` | RatingService | GetPlayerRating, GetPlayerRatings |
| `user.v1` | UserService | GetProfile, CheckBan |
| `lobby.v1` | LobbyService | CreateRankedRoom |
| `game.v1` | GameService | StartSession, IsParticipant |

## Redis Pub/Sub Events

Event structs live in `shared/events/`. Channel naming: `{domain}.{entity}.{verb}`.

All events must include an `event_id` (UUID) for idempotency. See [architecture.md](../architecture.md) for the full event table.
