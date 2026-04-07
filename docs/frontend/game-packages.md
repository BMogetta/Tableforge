# Game Packages

Each game directory under `frontend/src/games/{game}/` is a self-contained package with strict import boundaries.

## Structure

```
src/games/mygame/
  index.ts          # barrel export (the only public API)
  MyGameBoard.tsx   # main renderer component
  MyGameRules.tsx   # rules modal content
  types.ts          # game-specific state/move types
  assets/           # game-specific images, etc.
```

## Package Boundaries

All internal exports **must** have the `/** @package */` JSDoc annotation. This marks them as package-private -- they can only be imported within the same game directory.

```tsx
// src/games/mygame/MyGameBoard.tsx
/** @package */
export function MyGameBoard(props: GameBoardProps) { ... }
```

External code must import from the barrel:

```tsx
// GOOD
import { MyGameBoard } from '@/games/mygame'

// BAD - violates package boundary
import { MyGameBoard } from '@/games/mygame/MyGameBoard'
```

Biome's `noPrivateImports` rule enforces this for relative-path imports. Note: aliased `@/` imports are not yet resolved by Biome (known limitation).

## Current Games

### TicTacToe (`src/games/tictactoe/`)
- Exports: `TicTacToeBoard`, `TicTacToeRules`, `TicTacToeState` type

### RootAccess (`src/games/rootaccess/`)
- Exports: `RootAccess`, `RootAccessRules`, `CardName` type, `CARD_META`, `CardDisplay`
- Uses the shared Card UI kit (`src/ui/cards/`) for animated card interactions

## Registration

Games are registered in `src/games/registry.tsx`:

- `GAME_RENDERERS` -- maps `gameId` to a React component
- `GAME_RULES` -- entries with `id`, `label`, and `rules` component for the Rules modal

## Card UI Kit

For card-based games, use the shared components in `src/ui/cards/`:

| Component | Purpose |
|-----------|---------|
| `Card` | Single card with flip animation |
| `CardHand` | Fan layout with hover lift, supports reveal/play actions |
| `CardPile` | Stack visualization with count |
| `CardZone` | Layout container (pile/spread/stack modes) |

Animation utilities exported from `animations.ts`: flip, lift, fly-in, deal, slide-in variants.

Games provide their own card face content via render props (e.g., `CardFace` in rootaccess). The kit handles all animation and layout.
