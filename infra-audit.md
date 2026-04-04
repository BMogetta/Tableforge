# Infrastructure Audit — Production Readiness

**Date:** 2026-04-03
**Scope:** Docker Compose infrastructure, networking, secrets, observability
**Current state:** Development-ready, not production-ready

---

## Containers que faltan

### 1. PgBouncer (connection pooling) — PENDING (post-deploy)

8 servicios abren conexiones directas a PostgreSQL. Sin pooling, puede quedarse sin conexiones al escalar.

### 2. PostgreSQL backup (pg_dump cron) — PENDING (pre-deploy)

No hay backup automático.

### 3. Promtail (log collection) — PENDING (post-deploy)

Sin Promtail, Loki no recibe logs y el dashboard de logs en Grafana está vacío.

---

## Configuración — Estado actual

### ~~Hardcoded DB password~~ — FIXED
Todas las `DATABASE_URL` usan `${POSTGRES_PASSWORD}` interpolation.

### ~~HTTP server timeouts~~ — FIXED
Todos los 8 servicios tienen `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`.

### ~~HSTS~~ — FIXED
`Strict-Transport-Security` agregado en Traefik security config.

### ~~Rate limiting parcial~~ — FIXED
Auth-service ahora tiene rate limit (20 avg, 10 burst). Game-server ya lo tenía.
User-service todavía no tiene — pendiente.

### ~~connect-src hardcodeado a localhost~~ — FIXED
Cambiado a `connect-src 'self' ws: wss:` (funciona con cualquier dominio).

### ~~Cookie Secure flag~~ — FIXED
Default invertido: `secure=true` a menos que `ENV=development`.

---

## Pendiente — Antes de deploy (bloqueantes)

1. **TLS/SSL** (Cloudflare Tunnel o ACME) — sin TLS, cookies viajan en texto plano
2. **Grafana auth** — acceso anónimo como Admin (`GF_AUTH_ANONYMOUS_ORG_ROLE: Admin`)
3. **Traefik dashboard auth** — `api.insecure=true` en `:9090` sin contraseña
4. **Redis password + persistence** — sin `requirepass`, sin `appendonly`, sin `maxmemory`
5. **PostgreSQL sslmode** — `sslmode=disable` en todas las conexiones
6. **Resource limits** — ningún servicio tiene `deploy.resources.limits`
7. **Log rotation** — Docker acumula logs JSON indefinidamente
8. **Backup PostgreSQL** — sin backup automático
9. **ENV hardcodeado** — `ENV=development` fijo en compose

## Pendiente — Después del primer deploy

10. PgBouncer
11. Promtail
12. Non-root users en Dockerfiles
13. CSP sin `unsafe-inline`
14. Alert rules en Prometheus
15. Retención de observabilidad aumentada (Tempo 48h→30d, Loki 72h→30d)
16. Rate limiting en user-service
17. Healthchecks para Traefik y observabilidad

## Nice to have

18. gRPC healthchecks
19. Horizontal scaling docs
20. Disaster recovery runbook
21. Circuit breakers entre servicios
