# Changelog

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/match-service-vv0.1.0-alpha.1...match-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add JSON Schema coverage for match-service ([3c946f4](https://github.com/BMogetta/recess/commit/3c946f4c517368ff9cb0a43e3b2b28c4e6ece4d2))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* add test coverage, routing validation, and bug fixes ([8f92532](https://github.com/BMogetta/recess/commit/8f925329d876d632d2b95f39374df5ce3c28a7e8))
* **flags:** gate per-service flows with feature flags (3.3) ([9430375](https://github.com/BMogetta/recess/commit/94303757b76c2ced7b76326a4275da5df131eb06))
* **game:** rebrand Love Letter to Root Access ([b49db78](https://github.com/BMogetta/recess/commit/b49db78a5a02e8b0e9da0873aca0978b29aa6a90))
* **match-service:** make matchmaker tick and spread configurable ([4af5d3c](https://github.com/BMogetta/recess/commit/4af5d3c37352cf6f59e60e88d78c18b923618868))
* migrate match-service to Asynq and implement GetMoveLog RPC ([963eed6](https://github.com/BMogetta/recess/commit/963eed6aab01a0423b079ec8b2c7d1a463f391e3))
* **ranked-bots:** phase 3 backfill — inject bots when humans wait alone ([74e3f92](https://github.com/BMogetta/recess/commit/74e3f921d48e7f6c4022e4a90c1974ee6fe80b3e))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* **asynq:** pass redis password to asynq client connections ([abc721d](https://github.com/BMogetta/recess/commit/abc721ddffed8803b417940125255c2bbb5235e7))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* **match:** dedupe player.banned consumer by event_id ([f050aa0](https://github.com/BMogetta/recess/commit/f050aa02992d5da9fff288896c907307ad6641de))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* **rating:** add ResultID to GameSessionFinished event ([667d7fc](https://github.com/BMogetta/recess/commit/667d7fc2c145c3e9574940963f27f107189ebb07))
* resolve e2e test failures (9/55 → 36/36 passing) ([627d39e](https://github.com/BMogetta/recess/commit/627d39ef8f406ce38669b48c1197a9d2c1bd386f))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* **flags:** rename ranked-matchmaking-disabled → ranked-matchmaking-enabled ([6f1e99b](https://github.com/BMogetta/recess/commit/6f1e99b955905909cd5ad7fb68ce24c1daba067e))
* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
* **test:** add shared NewTestRedis helper, deduplicate miniredis setup ([2cbd541](https://github.com/BMogetta/recess/commit/2cbd541b0de54f4241f865f7401fc524f4fa87e5))
