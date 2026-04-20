# Changelog

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/user-service-vv0.1.0-alpha.1...user-service-vv0.2.0-alpha.1) (2026-04-20)


### Features

* **achievements:** add achievement system with tracking and UI ([679e8cf](https://github.com/BMogetta/recess/commit/679e8cf57dd693ffe438688bc99ae024c3ff8b2b))
* **achievements:** expose definitions API, drop frontend duplicate ([2c6f7b0](https://github.com/BMogetta/recess/commit/2c6f7b02c764d67835e2c2988ec22665d9529622))
* add admin audit logs backend ([57f7c47](https://github.com/BMogetta/recess/commit/57f7c476fb6fd07ea20577007364f09601a5cd99))
* add admin broadcast message endpoint ([6cf489b](https://github.com/BMogetta/recess/commit/6cf489b4543dcf942ca17d3b2a1a5219b424d326))
* add friend by username search in FriendsPanel ([08f23cf](https://github.com/BMogetta/recess/commit/08f23cf52fcaaf95a21d3ff18e226b9b15347e63))
* add JSON Schema coverage for user-service ([0c8c8cb](https://github.com/BMogetta/recess/commit/0c8c8cbf4514a291cc556f47166e803b121e7e0b))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* add test coverage, routing validation, and bug fixes ([8f92532](https://github.com/BMogetta/recess/commit/8f925329d876d632d2b95f39374df5ce3c28a7e8))
* **backend:** add lobby duplicate prevention + paginate DM history and admin players ([4d73800](https://github.com/BMogetta/recess/commit/4d73800139562cbf8828c0413984f47fa30f54d8))
* **bots:** add players.bot_profile + surface difficulty in UI badge ([64cf7d1](https://github.com/BMogetta/recess/commit/64cf7d1468e603f3a1ce16571df621220d83c931))
* **flags:** gate per-service flows with feature flags (3.3) ([9430375](https://github.com/BMogetta/recess/commit/94303757b76c2ced7b76326a4275da5df131eb06))
* friends system, DM inbox, and global presence ([537d358](https://github.com/BMogetta/recess/commit/537d358d1985404382e4cef56ea50b9e633f4a31))
* **frontend:** broadcast banner stack + fix moderation tab null.filter ([a005db3](https://github.com/BMogetta/recess/commit/a005db3e02c9d929176725aabb2cef6a307fe508))
* **pagination:** add limit/offset to rooms, notifications, and messages ([ae13032](https://github.com/BMogetta/recess/commit/ae13032b3b5b4070562cc2be31cb2abf2550ae14))
* **profile:** BOT badge on profile page + enriched identity in PlayerProfile ([5cd6460](https://github.com/BMogetta/recess/commit/5cd6460e14af343dc0a48f6680a0f6a8dc9fbd4d))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* add ownership and participant checks to chat-service and user-service endpoints ([b7b7879](https://github.com/BMogetta/recess/commit/b7b78793b16ef9ea7647769fed6fa759df6e7c63))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* Grafana dashboards, Prometheus networking, mutes nil slice ([82874bc](https://github.com/BMogetta/recess/commit/82874bc13299331872c01f834587f235c998a034))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* notification accept creates friendship + panel auto-clear bug ([feda800](https://github.com/BMogetta/recess/commit/feda800ac7f203bfeb3eef82a045d1b2090405de))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))


### Refactors

* **achievements:** move registry to shared/, auto-register via init() ([792fb5d](https://github.com/BMogetta/recess/commit/792fb5de952cf996606054ef1d8c41fd53311a37))
* **achievements:** push rules into Definition.ComputeProgress closures ([6bb95be](https://github.com/BMogetta/recess/commit/6bb95be8d15a8f7be1371d539d00346f880332fb))
* **achievements:** route names and descriptions through i18n keys ([74c017d](https://github.com/BMogetta/recess/commit/74c017d9067c5c83d694f4e3798460449fb1385e))
* move Grafana from path prefix to subdomain, remove login ([ee5d652](https://github.com/BMogetta/recess/commit/ee5d652cd71b158052f82cda9692ff1d18f3d4a5))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
