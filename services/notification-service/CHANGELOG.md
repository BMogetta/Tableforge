# Changelog

## [0.3.0-alpha.1](https://github.com/BMogetta/recess/compare/notification-service-v0.2.0-alpha.1...notification-service-v0.3.0-alpha.1) (2026-04-20)


### Features

* **achievements:** add achievement system with tracking and UI ([679e8cf](https://github.com/BMogetta/recess/commit/679e8cf57dd693ffe438688bc99ae024c3ff8b2b))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* add test coverage, routing validation, and bug fixes ([8f92532](https://github.com/BMogetta/recess/commit/8f925329d876d632d2b95f39374df5ce3c28a7e8))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **k8s:** phase 5.5 rollout of 7 Go services + Trivy fix ([06ea854](https://github.com/BMogetta/recess/commit/06ea854012a04acc13800b7c31027fee24ad37f3))
* **pagination:** add limit/offset to rooms, notifications, and messages ([ae13032](https://github.com/BMogetta/recess/commit/ae13032b3b5b4070562cc2be31cb2abf2550ae14))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* **game-server:** use isBot() store fallback in ready and rematch flows ([33f037a](https://github.com/BMogetta/recess/commit/33f037a6dbd64260ff764524e580c03c05ee5815))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* notification accept creates friendship + panel auto-clear bug ([feda800](https://github.com/BMogetta/recess/commit/feda800ac7f203bfeb3eef82a045d1b2090405de))
* **notifications:** collapse takeAction expiry+claim into a single UPDATE ([31b37da](https://github.com/BMogetta/recess/commit/31b37da67791d835409b5631554e362f460164ff))
* **notifications:** dedupe consumer events via source_event_id ([77af7d1](https://github.com/BMogetta/recess/commit/77af7d179d9c1bc65fdd0788ccc541b40650d866))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* return empty array instead of null for empty notification list ([9eb7598](https://github.com/BMogetta/recess/commit/9eb7598b29f0145bef031bca3278b5349975d4ea))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/notification-service-vv0.1.0-alpha.1...notification-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* **achievements:** add achievement system with tracking and UI ([679e8cf](https://github.com/BMogetta/recess/commit/679e8cf57dd693ffe438688bc99ae024c3ff8b2b))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* add test coverage, routing validation, and bug fixes ([8f92532](https://github.com/BMogetta/recess/commit/8f925329d876d632d2b95f39374df5ce3c28a7e8))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **k8s:** phase 5.5 rollout of 7 Go services + Trivy fix ([06ea854](https://github.com/BMogetta/recess/commit/06ea854012a04acc13800b7c31027fee24ad37f3))
* **pagination:** add limit/offset to rooms, notifications, and messages ([ae13032](https://github.com/BMogetta/recess/commit/ae13032b3b5b4070562cc2be31cb2abf2550ae14))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* **game-server:** use isBot() store fallback in ready and rematch flows ([33f037a](https://github.com/BMogetta/recess/commit/33f037a6dbd64260ff764524e580c03c05ee5815))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* notification accept creates friendship + panel auto-clear bug ([feda800](https://github.com/BMogetta/recess/commit/feda800ac7f203bfeb3eef82a045d1b2090405de))
* **notifications:** collapse takeAction expiry+claim into a single UPDATE ([31b37da](https://github.com/BMogetta/recess/commit/31b37da67791d835409b5631554e362f460164ff))
* **notifications:** dedupe consumer events via source_event_id ([77af7d1](https://github.com/BMogetta/recess/commit/77af7d179d9c1bc65fdd0788ccc541b40650d866))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* return empty array instead of null for empty notification list ([9eb7598](https://github.com/BMogetta/recess/commit/9eb7598b29f0145bef031bca3278b5349975d4ea))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
