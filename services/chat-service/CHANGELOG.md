# Changelog

## [0.3.0-alpha.1](https://github.com/BMogetta/recess/compare/chat-service-v0.2.0-alpha.1...chat-service-v0.3.0-alpha.1) (2026-04-25)


### Features

* **observability:** expose /metrics on auth, user, chat, rating, notification   services ([#83](https://github.com/BMogetta/recess/issues/83)) ([9b2c805](https://github.com/BMogetta/recess/commit/9b2c8054b5a4505fec57f70f0be3cd127b3b4bd6))

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/chat-service-vv0.1.0-alpha.1...chat-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add JSON Schema coverage for chat-service ([f863af6](https://github.com/BMogetta/recess/commit/f863af670b938ba6833813321168c941e8faf037))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* **backend:** add lobby duplicate prevention + paginate DM history and admin players ([4d73800](https://github.com/BMogetta/recess/commit/4d73800139562cbf8828c0413984f47fa30f54d8))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* enable friends-only DMs setting ([09d09d7](https://github.com/BMogetta/recess/commit/09d09d7793f72872d7bebc7a42be384f3e203ed8))
* **flags:** gate per-service flows with feature flags (3.3) ([9430375](https://github.com/BMogetta/recess/commit/94303757b76c2ced7b76326a4275da5df131eb06))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **k8s:** phase 5.5 rollout of 7 Go services + Trivy fix ([06ea854](https://github.com/BMogetta/recess/commit/06ea854012a04acc13800b7c31027fee24ad37f3))
* **pagination:** add limit/offset to rooms, notifications, and messages ([ae13032](https://github.com/BMogetta/recess/commit/ae13032b3b5b4070562cc2be31cb2abf2550ae14))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))


### Bug Fixes

* add ownership and participant checks to chat-service and user-service endpoints ([b7b7879](https://github.com/BMogetta/recess/commit/b7b78793b16ef9ea7647769fed6fa759df6e7c63))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* **chat:** route dm_read notifications to the sender, not the reader ([1b31608](https://github.com/BMogetta/recess/commit/1b316085368031500fde5ac779d87cb00293e19f))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
