# Releasing

This document describes the end-to-end release flow for Tableforge deployables.

## Overview

```
commit to main (feat/fix with component scope)
           │
           ▼
   release-please opens a release PR per affected component
           │
           ▼
   merge the release PR
           │
           ▼
   release-please creates a tag:  <component>-v<X.Y.Z>
           │
           ▼
   .github/workflows/release.yml
     - builds multi-arch image (amd64 + arm64)
     - scans with Trivy (HIGH/CRITICAL fail)
     - pushes vX.Y.Z + vX.Y + latest tags to ghcr.io
     - emits SLSA provenance + SPDX SBOM
     - cosign-signs the image digest
     - bumps infra/k8s/apps/<component>/values.yaml image.tag
     - pushes the bump commit to main with [skip ci]
           │
           ▼
   ArgoCD (k3s cluster, Fase 5)
     - detects the values.yaml change
     - syncs the new tag
     - rolling update on the cluster
```

## Components and scopes

| Component | Scope (in commit message) | Release tag format |
|---|---|---|
| game-server | `game-server` | `game-server-vX.Y.Z` |
| auth-service | `auth-service` | `auth-service-vX.Y.Z` |
| user-service | `user-service` | `user-service-vX.Y.Z` |
| chat-service | `chat-service` | `chat-service-vX.Y.Z` |
| rating-service | `rating-service` | `rating-service-vX.Y.Z` |
| match-service | `match-service` | `match-service-vX.Y.Z` |
| notification-service | `notification-service` | `notification-service-vX.Y.Z` |
| ws-gateway | `ws-gateway` | `ws-gateway-vX.Y.Z` |
| frontend | `frontend` | `frontend-vX.Y.Z` |

## Initial versions

The manifest (`.release-please-manifest.json`) seeds every component at
`0.1.0-alpha.1`. The first conventional commit with a component scope after
that point triggers release-please to open a release PR.

Until the repo reaches v1 for a component, breaking changes using `!` still
bump minor (release-please convention for 0.x).

## Making a release

1. Write commits with the right scope:
   ```
   feat(game-server): add ranked tiebreaker rule
   fix(chat-service): dedupe dm_read notifications
   feat(frontend)!: drop /legacy-room route
   ```
   See `CLAUDE.md` → "Release flow (conventional commits)" for the full spec.

2. Merge to `main`.

3. Release-please opens (or updates) a PR titled
   `chore(<component>): release <X.Y.Z>`. Review the changelog, then merge.

4. Merging the release PR triggers:
   - release-please pushes a tag `<component>-v<X.Y.Z>`
   - `.github/workflows/release.yml` builds and publishes the image
   - infra/k8s values.yaml bump commits to main with `[skip ci]`

5. If ArgoCD is configured (Fase 5), it syncs automatically. Otherwise the
   image is published and ready to pull.

## Rolling back

Manifest-mode release-please does **not** rewrite tags. To roll back:

- **Option A — git revert:** `git revert <bump-commit>` and merge to main.
  ArgoCD will sync the previous tag.
- **Option B — Makefile helper (Fase 5.14):** `make rollback SVC=game-server VERSION=1.2.2`
  rewrites the values.yaml and commits it.
- Reverting a release PR **after** the tag was created leaves the tag behind
  pointing at a commit whose changelog says the version was released. This is
  OK — just make a new release that supersedes it.

## Pre-releases

Until a component hits v1, use `0.1.0-alpha.N` etc. in the manifest if you
need an alpha track. Release-please's handling of prereleases is limited — for
most homelab iterations the implicit bump (`feat` → minor, `fix` → patch) is
enough.

## GHCR image URL pattern

```
ghcr.io/<repo-owner>/tableforge-<component>:v<X.Y.Z>   # exact version
ghcr.io/<repo-owner>/tableforge-<component>:v<X.Y>     # floating minor
ghcr.io/<repo-owner>/tableforge-<component>:latest     # always latest
```

ArgoCD values should pin to the exact `vX.Y.Z` tag (set by the bump step).
Never use `:latest` in k8s manifests — it defeats the whole point of pinning.

## Throwaway-history note

This repo is throwaway at the git level. When the clean repo is forked, the
final files (this doc, `release-please-config.json`, `.release-please-manifest.json`,
workflows) come along, but the intermediate commit history does not. The first
push of the clean repo re-seeds release-please at `0.1.0-alpha.1` for every
component and starts accumulating real history from there.
