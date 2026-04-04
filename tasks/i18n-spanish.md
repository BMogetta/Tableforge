# Spanish i18n

## Why
Settings UI has "Español" marked as "(coming soon)". The app is English-only.

## Status
TBD — needs architecture decisions:
- i18n library choice (react-i18next? custom? compile-time?)
- Translation file format (JSON? YAML?)
- How to handle game-specific strings (card names, game rules)
- RTL support not needed (Spanish is LTR)

## Rough scope
- ~200-300 UI strings to translate (buttons, labels, errors, tooltips)
- Game-specific strings (card names, status messages)
- Date/time formatting
- Number formatting (scores, ratings)

## Effort: High (touches every component with user-visible text)
