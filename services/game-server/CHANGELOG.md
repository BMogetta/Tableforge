# Changelog

## [0.2.0-alpha.1](https://github.com/BMogetta/recess/compare/game-server-vv0.1.0-alpha.1...game-server-vv0.2.0-alpha.1) (2026-04-20)


### Features

* add admin system stats endpoint ([0243c42](https://github.com/BMogetta/recess/commit/0243c42f0c0137ac31666b947d6ddae3c9f2b06b))
* add MaxBytesReader body size limit middleware ([f577e0e](https://github.com/BMogetta/recess/commit/f577e0e368ff843527868fb133adb8b86f2dae82))
* add orphan room reaper + migrate remaining player_id to JWT ([18f417b](https://github.com/BMogetta/recess/commit/18f417bc859d4086339e4a77bea48132c9b18bed))
* add test coverage, routing validation, and bug fixes ([8f92532](https://github.com/BMogetta/recess/commit/8f925329d876d632d2b95f39374df5ce3c28a7e8))
* **auth:** /auth/bot-login for production-safe bot sessions ([50d2b9c](https://github.com/BMogetta/recess/commit/50d2b9c2f4752c915a34689fd6a07e7ef2981df2))
* **backend:** add lobby duplicate prevention + paginate DM history and admin players ([4d73800](https://github.com/BMogetta/recess/commit/4d73800139562cbf8828c0413984f47fa30f54d8))
* **bot-runner:** phase 1 ranked bot runner ([f12ef74](https://github.com/BMogetta/recess/commit/f12ef74c5c1c098d7bc67c2c6361dda38cc5978f))
* **bot-runner:** profile-based think-time pacing ([96fa752](https://github.com/BMogetta/recess/commit/96fa752d84c4c45a14ada42ea668e6345648a2d9))
* **bots:** add players.bot_profile + surface difficulty in UI badge ([64cf7d1](https://github.com/BMogetta/recess/commit/64cf7d1468e603f3a1ce16571df621220d83c931))
* distributed turn timers, test coverage, and tech debt cleanup ([a8e2066](https://github.com/BMogetta/recess/commit/a8e2066f699f160ec87c350a6e8a65e6e1ed1034))
* enforce active session guard on room create/join ([b55a1b3](https://github.com/BMogetta/recess/commit/b55a1b3cf55e0971171d49827686467e3f315408))
* **engine:** move response as ack, remove polling, add ended_by to WS ([d0245d5](https://github.com/BMogetta/recess/commit/d0245d51a527bc2e73ddbb977231e3cc2b58c1fe))
* **flags:** gate per-service flows with feature flags (3.3) ([9430375](https://github.com/BMogetta/recess/commit/94303757b76c2ced7b76326a4275da5df131eb06))
* frontend domain migration, notifications UI, and game bugfixes ([059b814](https://github.com/BMogetta/recess/commit/059b81459c75182a4d267fbf0e247089d42c98d5))
* game replay support, active session detection, navigation guards ([c6a3e69](https://github.com/BMogetta/recess/commit/c6a3e6985c65bdb04c79110d37cb920e6b6c7b26))
* **game-server:** debug load-state endpoint for scenario testing ([19ff35a](https://github.com/BMogetta/recess/commit/19ff35a49e92687a689d812ee9fc2a987cbca949))
* **game-server:** scenario fixtures registry + load-scenario endpoint ([69e4cca](https://github.com/BMogetta/recess/commit/69e4ccab0007f769af7e829e050bae164053ad55))
* **game:** rebrand Love Letter to Root Access ([b49db78](https://github.com/BMogetta/recess/commit/b49db78a5a02e8b0e9da0873aca0978b29aa6a90))
* migrate match-service to Asynq and implement GetMoveLog RPC ([963eed6](https://github.com/BMogetta/recess/commit/963eed6aab01a0423b079ec8b2c7d1a463f391e3))
* **pagination:** add limit/offset to rooms, notifications, and messages ([ae13032](https://github.com/BMogetta/recess/commit/ae13032b3b5b4070562cc2be31cb2abf2550ae14))
* **profile:** enrich match history with opponent identity ([1906f7c](https://github.com/BMogetta/recess/commit/1906f7cff4f94c7b30583d2aefcc091754a42e36))
* **ranked-bots:** phase 3 backfill — inject bots when humans wait alone ([74e3f92](https://github.com/BMogetta/recess/commit/74e3f921d48e7f6c4022e4a90c1974ee6fe80b3e))
* **ranked:** remove rematch in ranked, offer back-to-queue instead ([2b16e14](https://github.com/BMogetta/recess/commit/2b16e145db852e6602317a47c75402caf391dfe3))
* replace swagger pipeline with JSON Schema shared types ([6104f88](https://github.com/BMogetta/recess/commit/6104f88cac6949b2f648813222104e6e5ac0e3cc))
* **rootaccess:** aesthetic pass + central discard pile ([e2d8166](https://github.com/BMogetta/recess/commit/e2d816644fc553c7013b029cc6fae7fb074d3d37))
* **services:** init unleash client + wrap maintenance middleware (3.1+3.2) ([d696044](https://github.com/BMogetta/recess/commit/d69604400992369db085d4775466f79ca1283265))
* turn timer, pause/resume fixes, Loki dashboard, and cleanup ([f67278c](https://github.com/BMogetta/recess/commit/f67278c1cd71d9b98672d808625a3d51b65ba61d))
* ws-gateway tests, shadow key pattern, and infra cleanup ([f7d0b6e](https://github.com/BMogetta/recess/commit/f7d0b6ef58d2a27ff56812ed4b00f1f618e257bc))


### Bug Fixes

* add auth middleware to rating-service + document public endpoints ([6e294a8](https://github.com/BMogetta/recess/commit/6e294a874863732600179dbcfaee6276be585b0f))
* **api:** source pause/resume broadcast timestamps from session, not time.Now ([11a1747](https://github.com/BMogetta/recess/commit/11a1747d4351fbaa188119ba554d838ea1d6e975))
* **asynq:** pass redis password to asynq client connections ([abc721d](https://github.com/BMogetta/recess/commit/abc721ddffed8803b417940125255c2bbb5235e7))
* **bot-runner:** redial player WS per backfill activation ([686c0e1](https://github.com/BMogetta/recess/commit/686c0e17526d84b762fedaa0790debbcd40c17fc))
* **bot:** re-fire MaybeFireBot when bot stays current after its own move ([ce041d0](https://github.com/BMogetta/recess/commit/ce041d0fad5d914fe8cec7313d0079be6d15a6cd))
* **build:** cross-compile Go binaries with GOARCH, drop amd64 target ([6288399](https://github.com/BMogetta/recess/commit/6288399b194fb04d95709b513e58957deaa2368a))
* **engine:** atomic move persistence and deep-copy state in RootAccess ([f4974e5](https://github.com/BMogetta/recess/commit/f4974e56cb99b6a7d8ce787ea1a2604ed88df6b1))
* **engine:** deterministic shuffle in RootAccess via seeded PRNG ([b299644](https://github.com/BMogetta/recess/commit/b2996443c47b65701ef986097c57f1076d0a665f))
* **engine:** enforce turn order in runtime, replace math/rand in bot jitter ([53da324](https://github.com/BMogetta/recess/commit/53da324aa3c045d4468b617d9bdd95172d1e7f07))
* **engine:** filter state in HTTP responses and route all timeouts through engine ([3c2679b](https://github.com/BMogetta/recess/commit/3c2679b5501bf1736cba40e14ff3a36d474417f9))
* **engine:** homogenize deep-copy + filter initial session broadcasts ([2cfeaf9](https://github.com/BMogetta/recess/commit/2cfeaf95714fcd61c7fbaa8f2beb1460e4088e9d))
* **game-server:** allow surrender while session is paused; map ErrSuspended ([e2d8166](https://github.com/BMogetta/recess/commit/e2d816644fc553c7013b029cc6fae7fb074d3d37))
* **game-server:** fire bot first-move in OnAllReady, not in handleStartGame ([3b9f136](https://github.com/BMogetta/recess/commit/3b9f1362678db21e70da0035c308479e38f0fa7b))
* **game-server:** publish game.session.finished for every mode, not just ranked ([6e11e92](https://github.com/BMogetta/recess/commit/6e11e92a2b96d5f5da3e0453e114929075d5e34f))
* **game-server:** use isBot() store fallback in ready and rematch flows ([33f037a](https://github.com/BMogetta/recess/commit/33f037a6dbd64260ff764524e580c03c05ee5815))
* harden Docker with non-root users, resource limits, and log rotation ([b80d9dd](https://github.com/BMogetta/recess/commit/b80d9dd44861fbacc17d82c17f798ed86a48249d))
* load JWT_SECRET in test mode when available ([8c5ebf1](https://github.com/BMogetta/recess/commit/8c5ebf1e55a8fe7486f7cc2443231ab48ae514ea))
* OWASP quick fixes — recoverer, auth logging, reflection, devtools, cosign ([43da204](https://github.com/BMogetta/recess/commit/43da2043503c4140b37fcaf54498d8ece06d4884))
* **rating:** add ResultID to GameSessionFinished event ([667d7fc](https://github.com/BMogetta/recess/commit/667d7fc2c145c3e9574940963f27f107189ebb07))
* resolve e2e test failures (9/55 → 36/36 passing) ([627d39e](https://github.com/BMogetta/recess/commit/627d39ef8f406ce38669b48c1197a9d2c1bd386f))
* **rootaccess:** allow ping/target cards when no valid targets exist ([45c8b71](https://github.com/BMogetta/recess/commit/45c8b71c4f71ea9d982ad2efed625ad6fe9757d7))
* **rootaccess:** debugger modal handles duplicate choices ([2e24933](https://github.com/BMogetta/recess/commit/2e24933bf4535ec64c503041a1a2eeb5b594b719))
* security and infra hardening across all services ([2ce18f1](https://github.com/BMogetta/recess/commit/2ce18f11ecfd413c8d8cd4f1ede71fd00c80af67))
* **ws-gateway:** spectator WS always rejected + count never broadcast ([6e8a2eb](https://github.com/BMogetta/recess/commit/6e8a2eb38871b6e66b0389364d4f56e8d3d2ad76))


### Refactors

* **bot-adapter:** invert registry into init-time plugin registration ([0f3e488](https://github.com/BMogetta/recess/commit/0f3e488bbb725de7aa54075c48b7aea7c6eade20))
* rebrand TableForge → Recess ([a9cfab0](https://github.com/BMogetta/recess/commit/a9cfab0491e2afaf35196666b541a9e0c4801e8a))
* standardize env var access (backend + frontend) ([d63b9a3](https://github.com/BMogetta/recess/commit/d63b9a304f9ab2c8959b4baa9457b3b2f7cd6d33))
* **tictactoe:** co-locate bot adapter inside the game plugin ([ee2ceeb](https://github.com/BMogetta/recess/commit/ee2ceeb8e5687e464f575b860c06380d8c3351fe))


### Documentation

* **scenarios:** add duplicate_firewalls + near_round_win fixtures + README ([3db23b2](https://github.com/BMogetta/recess/commit/3db23b2d31222fb153a31045dc989373a24ba4a6))
