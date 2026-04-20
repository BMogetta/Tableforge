# Changelog

## [0.3.0-alpha.1](https://github.com/BMogetta/recess/compare/ws-gateway-v0.2.0-alpha.1...ws-gateway-v0.3.0-alpha.1) (2026-04-20)


### Features

* add admin broadcast message endpoint ([6cf489b](https://github.com/BMogetta/recess/commit/6cf489b4543dcf942ca17d3b2a1a5219b424d326))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))
* **ws:** unify dual WebSocket connections into single GatewaySocket ([e78114a](https://github.com/BMogetta/recess/commit/e78114ab36498dc9a4ab18da05ae2a15d8c5b1dd))


### Bug Fixes

* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* e2e backend fixes — empty body, spectator loading, presence offline ([e9d177b](https://github.com/BMogetta/recess/commit/e9d177b776516af3caa5b83b0cb0332f7a0f54de))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))
* use detached context for pre-upgrade gRPC checks in ws-gateway ([54df8cc](https://github.com/BMogetta/recess/commit/54df8cc401d77a5c3b18611978dd6f600fe12891))
* **ws-gateway:** cancel listener goroutines when channels empty ([a0300de](https://github.com/BMogetta/recess/commit/a0300de3559d4279266389209d803756dce32958))
* **ws-gateway:** dedupe consumer events to prevent duplicate broadcasts ([54dfc08](https://github.com/BMogetta/recess/commit/54dfc0892749d506786e0878140f8da71f770123))
* **ws-gateway:** guard send channel close against races ([45e35e0](https://github.com/BMogetta/recess/commit/45e35e085fb27e315ebe05fce92291ff2542f60c))
* **ws-gateway:** spectator WS always rejected + count never broadcast ([6e8a2eb](https://github.com/BMogetta/recess/commit/6e8a2eb38871b6e66b0389364d4f56e8d3d2ad76))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
* standardize env var access (backend + frontend) ([d63b9a3](https://github.com/BMogetta/recess/commit/d63b9a304f9ab2c8959b4baa9457b3b2f7cd6d33))

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/ws-gateway-vv0.1.0-alpha.1...ws-gateway-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add admin broadcast message endpoint ([6cf489b](https://github.com/BMogetta/recess/commit/6cf489b4543dcf942ca17d3b2a1a5219b424d326))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))
* **ws:** unify dual WebSocket connections into single GatewaySocket ([e78114a](https://github.com/BMogetta/recess/commit/e78114ab36498dc9a4ab18da05ae2a15d8c5b1dd))


### Bug Fixes

* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* e2e backend fixes — empty body, spectator loading, presence offline ([e9d177b](https://github.com/BMogetta/recess/commit/e9d177b776516af3caa5b83b0cb0332f7a0f54de))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))
* use detached context for pre-upgrade gRPC checks in ws-gateway ([54df8cc](https://github.com/BMogetta/recess/commit/54df8cc401d77a5c3b18611978dd6f600fe12891))
* **ws-gateway:** cancel listener goroutines when channels empty ([a0300de](https://github.com/BMogetta/recess/commit/a0300de3559d4279266389209d803756dce32958))
* **ws-gateway:** dedupe consumer events to prevent duplicate broadcasts ([54dfc08](https://github.com/BMogetta/recess/commit/54dfc0892749d506786e0878140f8da71f770123))
* **ws-gateway:** guard send channel close against races ([45e35e0](https://github.com/BMogetta/recess/commit/45e35e085fb27e315ebe05fce92291ff2542f60c))
* **ws-gateway:** spectator WS always rejected + count never broadcast ([6e8a2eb](https://github.com/BMogetta/recess/commit/6e8a2eb38871b6e66b0389364d4f56e8d3d2ad76))


### Refactors

* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
* standardize env var access (backend + frontend) ([d63b9a3](https://github.com/BMogetta/recess/commit/d63b9a304f9ab2c8959b4baa9457b3b2f7cd6d33))
