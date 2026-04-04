# OWASP Top 10 2025 — Remaining Audits

## Why
4 of 10 OWASP categories already audited:
- ✅ A01 Broken Access Control → `security-audit-bac.md`
- ✅ A04 Cryptographic Failures → `security-audit-crypto.md`
- ✅ A05 Injection → `security-audit-injection.md`
- ✅ A07 Authentication Failures → `security-audit-auth.md`

6 remaining categories need static code review.

## Categories to audit (run as 3 parallel agents, 2 categories each)

### Agent 1: A02 + A08
- **A02: Security Misconfiguration** — default configs, unnecessary features,
  error handling exposing internals, missing security headers, outdated deps
- **A08: Software and Data Integrity Failures** — CI/CD pipeline integrity,
  dependency verification, deserialization attacks, unsigned updates

### Agent 2: A03 + A06
- **A03: Vulnerable and Outdated Components** — scan go.mod and package.json
  for known CVEs, check dependency versions, identify unmaintained packages
- **A06: Security Logging and Monitoring Failures** — audit logging coverage,
  are auth failures logged? admin actions? bans? is there alerting?

### Agent 3: A09 + A10
- **A09: Server-Side Request Forgery (SSRF)** — check for user-controlled URLs
  in HTTP clients, gRPC targets, Redis/Postgres connection strings
- **A10: Insufficient Attack Protection** — rate limiting coverage, input size
  limits, request body size limits, file upload restrictions

## Output
Each agent produces a `security-audit-{category}.md` file in the repo root,
following the same format as existing audit files (Summary table, findings by
severity, "Done Correctly" section).

## Scope
All 8 backend services + frontend + infrastructure configs.
Static code review only (no dynamic scanning).
