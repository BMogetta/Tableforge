# Redis Password + Persistence

## Why
Redis has no `requirepass`, no `appendonly`, no `maxmemory`. Any container on
the network can access all data. Data lost on restart.

## What to change
- Add `requirepass` via Redis config or command
- Update `REDIS_URL` in ALL services to include password: `redis://:${REDIS_PASSWORD}@redis:6379`
- Add `appendonly yes` for persistence
- Set `maxmemory` + `maxmemory-policy allkeys-lru`
- Add `REDIS_PASSWORD` to `.env.example`

## Risk
Coordinated change — touches every service's env config. Test thoroughly.
