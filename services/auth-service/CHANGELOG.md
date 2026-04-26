# Changelog

## [0.3.0-alpha.1](https://github.com/BMogetta/recess/compare/auth-service-v0.2.0-alpha.1...auth-service-v0.3.0-alpha.1) (2026-04-25)


### Features

* **observability:** expose /metrics on auth, user, chat, rating, notification   services ([#83](https://github.com/BMogetta/recess/issues/83)) ([9b2c805](https://github.com/BMogetta/recess/commit/9b2c8054b5a4505fec57f70f0be3cd127b3b4bd6))

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/auth-service-vv0.1.0-alpha.1...auth-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* **auth:** /auth/bot-login for production-safe bot sessions ([50d2b9c](https://github.com/BMogetta/recess/commit/50d2b9c2f4752c915a34689fd6a07e7ef2981df2))
* **auth:** add GET /auth/me/capabilities endpoint (3.4) ([90d45be](https://github.com/BMogetta/recess/commit/90d45bead35b237693de2c802031bc2fefa20670))
* **backend:** add lobby duplicate prevention + paginate DM history and admin players ([4d73800](https://github.com/BMogetta/recess/commit/4d73800139562cbf8828c0413984f47fa30f54d8))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* implement JWT refresh token rotation with server-side revocation ([357940b](https://github.com/BMogetta/recess/commit/357940b3de5beb1c5328f6d8f72ba5e1635b8c0f))
* postgres and grafana mcp ([4416cc4](https://github.com/BMogetta/recess/commit/4416cc4857a01fa6dd68e7d1b1f0983ec6189cc1))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* added files that were excluded from other merges ([1f90021](https://github.com/BMogetta/recess/commit/1f9002122c78a2c72e6bec77d244a72815a3fa75))
* **auth:** session cookie must outlive JWT to trigger silent refresh ([8248048](https://github.com/BMogetta/recess/commit/8248048e97d85305836a6725ebbbd6cf41a3ef84))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
* standardize env var access (backend + frontend) ([d63b9a3](https://github.com/BMogetta/recess/commit/d63b9a304f9ab2c8959b4baa9457b3b2f7cd6d33))
* **test:** add shared NewTestRedis helper, deduplicate miniredis setup ([2cbd541](https://github.com/BMogetta/recess/commit/2cbd541b0de54f4241f865f7401fc524f4fa87e5))
