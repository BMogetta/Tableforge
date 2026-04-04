# Game Rules / Onboarding Screens

## Why
New users have no way to learn game rules from within the app. They must know
the games beforehand or figure them out by playing.

## What to build

### Per-game rules modal
- Add a "How to Play" button/icon next to each game in the game selector
- Clicking opens a modal with:
  - Game objective (1-2 sentences)
  - Turn structure
  - Win conditions
  - Card/piece descriptions (for Love Letter / rebrand)
- Use existing `.card` and `.btn` CSS classes

### Where
- `frontend/src/games/tictactoe/Rules.tsx` — new component
- `frontend/src/games/loveletter/Rules.tsx` — new component (or whatever the
  rebrand name is)
- Each game registers its Rules component in the game registry
- The Room page and Game page can show a "Rules" button that opens the modal

### First-time experience (optional, TBD)
- Show rules modal automatically on first game join
- Use localStorage flag `seen_rules_{gameId}` to track

## No backend changes needed

## Testing
- Component test for Rules modal rendering
- Manual: verify modal opens and closes correctly
