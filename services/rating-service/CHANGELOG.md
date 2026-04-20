# Changelog

## [0.3.0-alpha.1](https://github.com/BMogetta/recess/compare/rating-service-v0.2.0-alpha.1...rating-service-v0.3.0-alpha.1) (2026-04-20)


### Features

* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* **leaderboard:** filter bots by default, opt-in toggle + BOT badges ([beb4638](https://github.com/BMogetta/recess/commit/beb4638c2c3a191781efe4b86046a6ed675df99e))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* add auth middleware to rating-service + document public endpoints ([6e294a8](https://github.com/BMogetta/recess/commit/6e294a874863732600179dbcfaee6276be585b0f))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* CSP inline scripts, otel strip prefix, rating schema, leaderboard API ([0bbcad8](https://github.com/BMogetta/recess/commit/0bbcad848e607db6d24f8d35316901c8c23a7714))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* **rating:** add ResultID to GameSessionFinished event ([667d7fc](https://github.com/BMogetta/recess/commit/667d7fc2c145c3e9574940963f27f107189ebb07))
* **rating:** dedupe session.finished delivery via unique (session_id, player_id) on rating_history ([54ffeee](https://github.com/BMogetta/recess/commit/54ffeee77b782a09c5946478eac7eb9090ba73bf))
* **rating:** keep dedupe test within rating-service scope ([3ee7bc0](https://github.com/BMogetta/recess/commit/3ee7bc0f5b5b61f1320bfac092e94e9f75251300))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))


### Documentation

* update e2e results (34/55) + fix go.mod for shared middleware ([a9f534a](https://github.com/BMogetta/recess/commit/a9f534a188730bb71353d6609aa4c5add7f50012))

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/rating-service-vv0.1.0-alpha.1...rating-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* **leaderboard:** filter bots by default, opt-in toggle + BOT badges ([beb4638](https://github.com/BMogetta/recess/commit/beb4638c2c3a191781efe4b86046a6ed675df99e))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* add auth middleware to rating-service + document public endpoints ([6e294a8](https://github.com/BMogetta/recess/commit/6e294a874863732600179dbcfaee6276be585b0f))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* CSP inline scripts, otel strip prefix, rating schema, leaderboard API ([0bbcad8](https://github.com/BMogetta/recess/commit/0bbcad848e607db6d24f8d35316901c8c23a7714))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* **rating:** add ResultID to GameSessionFinished event ([667d7fc](https://github.com/BMogetta/recess/commit/667d7fc2c145c3e9574940963f27f107189ebb07))
* **rating:** dedupe session.finished delivery via unique (session_id, player_id) on rating_history ([54ffeee](https://github.com/BMogetta/recess/commit/54ffeee77b782a09c5946478eac7eb9090ba73bf))
* **rating:** keep dedupe test within rating-service scope ([3ee7bc0](https://github.com/BMogetta/recess/commit/3ee7bc0f5b5b61f1320bfac092e94e9f75251300))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))


### Documentation

* update e2e results (34/55) + fix go.mod for shared middleware ([a9f534a](https://github.com/BMogetta/recess/commit/a9f534a188730bb71353d6609aa4c5add7f50012))
