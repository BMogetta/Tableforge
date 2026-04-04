# Achievements UI

## Why
Backend endpoint `GET /api/v1/players/{id}/achievements` exists but there's no
frontend to display achievements. No achievement definitions or tracking logic
exists yet either.

## Status
TBD — needs design decisions:
- What achievements to implement (first win, 10 wins, play 100 games, etc.)
- How to track progress (event-driven via Redis Pub/Sub? batch job?)
- Where to display (profile page? dedicated tab? notification on unlock?)
- Visual design (badges? icons? progress bars?)

## Rough scope
### Backend (user-service)
- Define achievement types and thresholds
- Add achievement tracking (probably a consumer listening to `game.session.finished`)
- Store: `achievements` table with player_id, achievement_id, unlocked_at

### Frontend
- Profile page: achievements section with badges
- Notification on unlock
- Achievement detail modal with progress

## Effort: Medium-High (needs design first)
