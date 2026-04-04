# Rebrand Love Letter

## Why
"Love Letter" is a trademarked game by Z-Man Games. Using the name and card
identities is a legal risk. Must rename before any public-facing deployment.

## Scope
Full rename of the game across backend and frontend. The game engine logic
stays the same — only names, labels, and identifiers change.

## New name
TBD — pick a thematic name that fits the card-game genre. Suggestions:
- "Court Intrigue"
- "Royal Favor"  
- "The Last Letter"

## What to change

### Backend (`services/game-server/`)
- `games/loveletter/` → rename directory to new game ID
- `loveletter.go` — change `GameID` constant, card names, role names
- Game registry entry in `games/registry.go`
- Database: existing `game_id = 'loveletter'` rows need a migration

### Frontend (`frontend/src/games/`)
- `games/loveletter/` → rename directory
- Component names, CSS module files
- Card face content (Princess, Countess, King, etc.) → new character names
- Game registry in `games/registry.tsx`
- Any hardcoded "Love Letter" strings in UI

### Shared
- `shared/schemas/defs/` — if any schema references the game by name
- Test fixtures that reference 'loveletter'

### Database migration
- `shared/db/migrations/` — add migration to UPDATE game_id in:
  - `games` table
  - `sessions` table  
  - `game_results` table
  - `ratings` table
  - Any other table with `game_id` column

## Testing
- `go test ./services/game-server/...`
- `npm test` in frontend
- Verify game is playable end-to-end after rename
