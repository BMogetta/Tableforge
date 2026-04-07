# Styling Guide

CSS Modules for all component styles. Design tokens and utility classes live in `frontend/src/styles/global.css`.

## Token Architecture

Three layers, from raw values to usage:

### 1. Primitives (never use directly)

Color ramps: `amber`, `ink`, `parchment`, `obsidian`, `sand`, `slate`, `indigo`, `ivory`, `moss`, `charcoal`. Plus semantic: `danger`, `success`, `warning`.

### 2. Semantic Tokens (use these)

Theme-aware variables that change based on `[data-theme]`:

```css
/* Backgrounds */
--color-bg-base          /* page background */
--color-bg-surface       /* card/panel background */
--color-bg-elevated      /* raised elements */
--color-bg-hover         /* hover state */
--color-bg-overlay       /* modal backdrop */

/* Text */
--color-text-primary     /* main text */
--color-text-secondary   /* supporting text */
--color-text-muted       /* disabled/subtle text */

/* Interactive */
--color-interactive      /* buttons, links, accents */
--color-interactive-hover
--color-interactive-dim
--color-interactive-glow

/* Borders */
--color-border
--color-border-bright

/* Status */
--color-danger
--color-success
--color-warning

/* Focus */
--color-focus-ring
```

### 3. Themes

Four built-in themes, switchable at runtime via `data-theme` attribute on `<html>`:

| Theme | Attribute | Vibe |
|-------|-----------|------|
| Dark (default) | `data-theme="dark"` | Amber gold on near-black ("Midnight Tavern") |
| Light | `data-theme="light"` | Brown ink on warm cream ("Illuminated Manuscript") |
| Slate | `data-theme="slate"` | Indigo on cold blue-grey ("Strategy Room") |
| Ivory | `data-theme="ivory"` | Moss green on clean white ("Academy Hall") |

## Spacing

Always use `--space-N` tokens. Never hardcode pixel values for spacing.

```
--space-1: 4px    --space-5: 20px    --space-10: 40px
--space-2: 8px    --space-6: 24px    --space-12: 48px
--space-3: 12px   --space-7: 28px
--space-4: 16px   --space-8: 32px
```

## Typography

```css
--font-display: "Cinzel"        /* headings, serif */
--font-mono: "JetBrains Mono"   /* code, monospace */

--text-xs through --text-2xl    /* all rem-based, scaled by --font-scale */
```

`--font-scale` is a unitless multiplier (default `1`). Users can change it at runtime. All `--text-*` tokens are pre-multiplied -- never multiply by `--font-scale` again.

## Z-Index Scale

```
--z-base: 0        --z-sticky: 200    --z-toast: 400
--z-dropdown: 100   --z-modal: 300     --z-noise: 9999
```

## Utility Classes

```css
.btn .btn-primary .btn-secondary .btn-ghost .btn-danger .btn-sm
.input .label
.card
.badge .badge-amber .badge-muted
.divider
.skeleton .pulse .page-enter
```

## Rules

- **CSS Modules** for all page and component styles
- **Never use `!important`**
- **Never hardcode colors** -- always use semantic tokens
- **Never use primitives** (`--slate-900`) -- use semantic (`--color-bg-surface`)
- Borders, radius, shadows: raw `px` (they don't scale with font)
- Buttons: always `.btn` + variant class, never inline styles

## Responsive

Desktop-first. Default styles target 1280px+. Use `max-width` media queries to adapt down:

| Breakpoint | Target |
|-----------|--------|
| `1024px` | Small desktop / laptop |
| `768px` | Tablet: collapse sidebars |
| `640px` | Mobile: single column |

Never use `min-width` unless building a genuinely mobile-first component. For font scaling on small screens, adjust `--font-scale` on `:root`.

## Accessibility (WCAG 2.1 AA)

- Semantic elements: `<button>` for actions, `<a>` for navigation
- Every `<input>` needs `<label>` via `htmlFor` or `aria-label`
- Focus rings: never `outline: none` without `box-shadow` using `var(--color-focus-ring)`
- Dynamic content: `aria-live="polite"` on game state, chat, turn changes
- Game boards: `role` + `aria-label` on non-native elements
- Modals: `role="dialog"`, `aria-modal="true"`, `aria-labelledby`
- Never `div`/`span` with `onClick` -- use `button` or anchor
