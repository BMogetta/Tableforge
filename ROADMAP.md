# Recess — Roadmap

Goal of the project (restated so priorities are evaluated consistently):
**Simulate a mature production environment end to end.** This is explicitly
not a "minimum viable homelab" — decisions lean toward industry-standard
patterns (GitOps, GitHub App auth, SLOs, DR drills, supply-chain signing)
even when a simpler homelab shortcut would work.

Tiers reflect that bias: every P0/P1 item is on the shortest path to a
realistic prod-grade cluster running the app, not just to "it's up".

Each item carries a **parallelizable** note so multi-session work (two
Claude instances + one human, or one human with two browser tabs) stays
coherent.

**Legend:** `[ ]` open · `[~]` in progress · `[x]` done · `[!]` blocked

---

## P0 — Finish the current trunk-based setup (this week)

These items close loops left dangling by recent work. Nothing below them
makes sense until this layer is stable.

### P0.1 — Apply the main-branch ruleset
- [ ] Run `bash scripts/setup-branch-protection.sh` on `BMogetta/recess`.
- [ ] Verify `gh api repos/BMogetta/recess/rulesets` lists `main protection`
      as `active`.
- [ ] Verify a direct `git push origin main` from a local feature-branch
      checkout gets rejected (negative test).
- **Parallelizable with:** nothing else — this reshapes how every other
  change lands. Do it first.

### P0.2 — Release flow via PR + auto-merge
- [ ] Edit `.github/workflows/release.yml`: replace the direct
      `git push origin main` with `gh pr create` + `gh pr merge --auto
      --squash --delete-branch`.
- [ ] Confirm the next merged release-please PR triggers a deploy PR,
      which auto-merges once opened (since required-approval count is 0
      in the ruleset).
- **Parallelizable with:** P0.3 (different file).
- **Depends on:** P0.1 (otherwise the current direct-push still works
  and we don't discover the bug).

### P0.3 — Commit staged Governance A.2 follow-ups (task #21)
- [ ] `topologySpreadConstraint` + `priorityClassName` + `tmpEmptyDir`
      chart changes + the `priorityclasses.yaml` already drafted in the
      working tree.
- [ ] Polaris re-audit expected to land ≈90%.
- **Parallelizable with:** P0.1, P0.2 (different files entirely).

### P0.4 — Folder rename `tableforge/` → `recess/`
- [ ] Follow `docs/runbooks/folder-rename.md` end to end.
- **Parallelizable with:** nothing — requires this Claude session to be
  closed, so it's blocking on itself. 5-minute task.

### P0.5 — Close task #20 (root sync stall)
The sync-wave removal + controller concurrency bump from the previous
session fixed the symptom but we never verified the root reconciles in
its new steady-state faster. Measure one cycle end to end, then close
the task (or reopen with new findings).
- [ ] Trigger a deliberate no-op bump (touch one values file, commit,
      push), measure `deployStartedAt` → `deployedAt` on the affected
      child app. Should be < 2 min.
- [ ] Update task #20 with the measurement and close.

---

## P1 — Split to two-repo GitOps (next 1–2 weeks)

Target end state: `BMogetta/recess` holds source; `BMogetta/recess-deploy`
holds the k8s manifests that ArgoCD watches. This is the single biggest
architectural improvement in the queue.

### P1.1 — Create a GitHub App for bot pushes (task #25)
- [ ] Register "recess-deploy-bot" GitHub App on the `BMogetta` account.
      Scopes: `contents: write`, `pull-requests: write`, `metadata: read`.
- [ ] Install on `BMogetta/recess-deploy` (will be created next step) and
      on `BMogetta/recess` (needs to read source commits to build
      changelog bumps).
- [ ] Store `APP_ID` + `APP_PRIVATE_KEY` as repo-level secrets on
      `recess`.
- [ ] Update `release.yml` to mint an install token from the App instead
      of relying on `GITHUB_TOKEN`. That install token is what pushes the
      image-tag bump to the deploy repo.
- **Parallelizable with:** P1.2 (different deliverables), but both are
  prereqs for P1.3.

### P1.2 — Create `recess-deploy` repo
- [ ] New empty repo on GitHub.
- [ ] Seed with `infra/k8s/` contents copied from `recess`.
- [ ] Apply a ruleset similar to `recess`'s, but with the `recess-deploy-bot`
      App in the bypass list so auto-bumps land without needing PRs.
- [ ] Configure ArgoCD's `recess-root` Application to point at
      `recess-deploy/main` instead of `recess/main`.
- **Parallelizable with:** P1.1.

### P1.3 — Wire release.yml to push to the deploy repo
- [ ] release.yml: clone `recess-deploy`, bump the relevant
      `charts/go-service/values/<svc>.yaml`, push using the App token.
- [ ] Test end to end: merge a release-please PR → watch release.yml log
      → confirm a commit lands on `recess-deploy` → ArgoCD syncs the new
      image.
- **Depends on:** P1.1 + P1.2.

### P1.4 — Strip k8s manifests out of `recess`
- [ ] Move `infra/k8s/` entirely to `recess-deploy`.
- [ ] `recess` keeps: source code, Dockerfiles, `charts/go-service/` (the
      Helm chart template) — arguable whether the chart belongs in source
      or deploy; pragmatic default is to keep it in `recess` because it
      changes alongside code.
- [ ] Update `.github/paths-filters.yml` so CI's path-based triggers no
      longer reference `infra/k8s`.
- [ ] Update `CLAUDE.md` sections that describe the repo layout.
- **Depends on:** P1.3 (must have verified the deploy repo path works
  before deleting the source-repo copy).

### P1.5 — Tighten `recess` ruleset
With manifests gone, `recess` is pure code. Flip on the strict rules
that conflicted with bot auto-pushes before.
- [ ] Re-enable `required_status_checks` with `CI Success` + `CodeQL`.
- [ ] Consider requiring `signed_commits` (you already use GPG).
- [ ] Leave `bypass_actors` empty — no bot writes to source.
- **Depends on:** P1.4.

---

## P2 — Governance hardening (parallel friendly, run alongside P3/P4)

After the split, these items are almost fully independent. Each can be
picked up by a separate Claude session without stepping on the others,
because they touch different namespaces or different systems.

### P2.1 — Secret rotation strategy (task #11)
Currently `DATABASE_URL` and `REDIS_URL` are sealed as full DSNs in the
`recess` namespace. Rotations of `pg-app` or `redis-auth` in `recess-data`
silently invalidate them.
- [ ] Evaluate ESO (External Secrets Operator) vs Reflector for
      cross-namespace Secret replication.
- [ ] Install the chosen controller.
- [ ] Rewrite the go-service chart to bind password-only Secrets and
      assemble the DSN in the Deployment (`env` with prefix, or an
      entrypoint wrapper). Retire the sealed-DSN pattern.
- **Parallelizable with:** P2.2, P2.3, P2.4.
- **Touches:** chart + new operator namespace; orthogonal to observability.

### P2.2 — Velero + DR drill (GOVERNANCE_PLAN Phase C)
- [ ] Provision S3-compatible bucket (Cloudflare R2 free tier).
- [ ] Install Velero via ArgoCD Application.
- [ ] Seal the bucket credentials.
- [ ] First manual backup, verify tarballs in the bucket.
- [ ] Restore drill on a throwaway `k3d` cluster on the workstation.
- [ ] Daily `Schedule` CR for automated backups.
- **Parallelizable with:** P2.1, P2.3, P2.4.
- **Human-input blocker:** needs the bucket credentials (one-time 10 min).

### P2.3 — Alertmanager routing + initial rules (GOVERNANCE_PLAN Phase D)
- [ ] Discord workspace with 5 channels + webhooks.
- [ ] Seal webhooks as `alertmanager-discord-webhooks` SealedSecret.
- [ ] `AlertmanagerConfig` CR with severity-based routing.
- [ ] Three `PrometheusRule` CRs: infra, workloads, app-specific.
- [ ] Watchdog + healthchecks.io dead-man heartbeat.
- **Parallelizable with:** P2.1, P2.2, P2.4, P2.5.
- **Human-input blocker:** Discord server + webhooks + healthchecks.io
  signup (30 min one-time).

### P2.4 — NetworkPolicies
Currently every namespace is wide-open cross-namespace. Polaris flags
`missingNetworkPolicy` on every pod.
- [ ] Design the mesh: `recess-data` accepts only `recess`; `observability`
      accepts `recess` + ingress-from-cluster; `cloudflared` egress only
      to `traefik` in `kube-system`; etc.
- [ ] Emit `NetworkPolicy` CRs per namespace.
- [ ] Verify with a negative test (pod without a matching label can't
      reach Postgres).
- **Parallelizable with:** P2.1, P2.2, P2.3, P2.5.
- **Touches:** infrastructure only, no code changes.

### P2.5 — Goldilocks + VPA (GOVERNANCE_PLAN Phase B)
- [ ] Install VPA.
- [ ] Install Goldilocks Application.
- [ ] Label `recess`, `recess-data`, `observability` namespaces.
- [ ] Wait 7 days of VPA observation before applying
      recommendations (non-interactive wait).
- [ ] Apply recommendations, verify no OOMKills for 7 more days.
- **Parallelizable with:** everything in P2.
- **Calendar blocker:** 14-day observation window whether you touch it
  or not.

### P2.6 — /metrics endpoints on the 5 remaining services
- [ ] Add `r.Get("/metrics", promhttp.Handler(...))` and
      `go get github.com/prometheus/client_golang/...` to: auth-service,
      user-service, chat-service, rating-service, notification-service.
- [ ] Flip `metrics.enabled: true` in each service's chart values.
- [ ] Verify Prometheus targets in the UI.
- **Parallelizable with:** P2.1 through P2.5.
- **Note:** low priority since OTel Collector already re-exposes these
  metrics on :8889; direct scrape is a redundancy, not a must-have.

### P2.7 — Polaris A.2 leftovers (task #21 residuals after the first pass)
- [ ] `readOnlyRootFilesystem: true` on frontend (Nginx): migrate
      `/var/cache/nginx` + `/var/run` to emptyDir volumes in nginx.conf
      + chart.
- [ ] Bump replica counts on a subset of services (stateless ones) so
      the PDB template actually renders — currently no service has ≥2
      replicas.
- **Parallelizable with:** all of P2.

---

## P3 — Production maturity (weeks 3–4+)

These are the things that separate "running in k8s" from "running a
service people would pay for".

### P3.1 — Argo Rollouts for progressive delivery
Replace the default Deployment rolling update with canary / blue-green
strategies. Argo Rollouts is ArgoCD's sister project — standard stack.
- [ ] Install the Rollouts operator Application.
- [ ] Migrate 1 service (suggestion: `match-service`) from Deployment to
      Rollout CR with a 20%/80% canary split.
- [ ] Verify automatic rollback on a bad deploy (inject a readiness probe
      failure).
- **Parallelizable with:** anything in P2.

### P3.2 — Multi-cluster (dev / staging / prod)
Spin up one more k3s cluster (second Pi or k3d on the workstation) to
learn the GitOps-for-many-clusters pattern. ArgoCD's `ApplicationSet`
is the right tool.
- [ ] Second cluster bootstrap.
- [ ] `ApplicationSet` with cluster generator so the same
      `recess-deploy/infra/k8s/apps/` yields one Application per cluster.
- [ ] Different per-cluster values (dev = replicaCount:1, prod = :2;
      dev = non-sealed creds, prod = Vault-backed, etc).
- **Depends on:** P1 (single-cluster patterns need to be clean first).

### P3.3 — Supply-chain: signed images + SBOM attestation
CD already publishes SBOM + cosign-signs. Close the loop by verifying
at the cluster:
- [ ] Install Kyverno or Sigstore Policy Controller.
- [ ] Policy: reject any Pod whose image wasn't signed by the Recess key.
- **Parallelizable with:** P2 and other P3 items.

### P3.4 — SLOs + burn-rate alerting
Define SLIs/SLOs in Prometheus recording rules, alert on burn rate.
- [ ] SLI definitions per service (latency p95, error rate).
- [ ] `PrometheusRule` with burn-rate expressions.
- [ ] Grafana dashboard for SLO status.
- **Depends on:** P2.3 (alerting pipeline) + P2.6 (real /metrics).

### P3.5 — Chaos engineering
- [ ] Install Chaos Mesh or Litmus.
- [ ] Scheduled chaos experiments (kill a random pod, inject network
      latency) — verify alerts fire + system self-heals.
- **Depends on:** P2.3 (so alerts actually fire when chaos happens).

---

## P4 — Polish, research, and deliverables (anytime)

Self-contained improvements that don't block anything.

- [ ] **Frontend PWA opt-in** — move `Notification.requestPermission()`
      from Game.tsx auto-call into a settings toggle. Trivial UX fix
      spotted in task #23.
- [ ] **BOOTSTRAP.md gaps** — sections for Phase 5.5 (full services
      rollout), 5.8 (cloudflared), 6.3/6.4 (Tempo + OTel), A.1 (Polaris).
      Each is ≤30 min.
- [ ] **`/tmp/seal-canary-secrets.sh`** — move to
      `scripts/seal-canary-secrets.sh` with the urlencode wrapper the
      ad-hoc script already has. Parallel of `reseal-dsn-secrets.sh`.
- [ ] **Public portfolio README** — once P1 lands, write a top-of-repo
      overview explaining the architecture. This repo is meant to
      *simulate a real environment*; a reader hitting GitHub should get
      "oh this is a monorepo with a GitOps split, here's the diagram".

---

## Parallelization notes (the part you asked about)

**Short answer:** the two-repo split (P1) **helps** parallelization a
lot, but it helps *for the items you'd do after it*, not the items you'd
do *to get it done*. P1 itself is sequential because each step builds on
the previous one (App → deploy repo → release.yml → strip source →
tighten rules).

### Before P1 lands (what we have today)

One repo. Any change — code, infra, or CI — lands in the same git tree.
Two Claude sessions working at the same time would collide on:

- `.github/workflows/*.yml` (CI and release-flow edits often touch the
  same files)
- `infra/k8s/` (chart and manifest edits clash)
- Anything in `services/*/` (Go code + matching manifests pair up)

Safe parallelization today is mostly **across language boundaries**:
- one session on frontend (`frontend/` + locales)
- one session on Go services (`services/` + `shared/`)
- maybe a third on docs (`docs/`, `*.md`, `CLAUDE.md`)

Infra work (`infra/k8s/`, workflows, charts) **is hard to parallelize**
because every piece ties to every other.

### After P1 lands

You split cleanly:
- `recess`: code-only. A Claude session here edits Go/frontend/tests.
  Never touches manifests.
- `recess-deploy`: manifests-only. A separate session here edits helm
  values, Application CRs, cluster-wide resources. Never touches code.

The two repos have **no files in common**. Two sessions can run
concurrently without any chance of merge conflict.

### Concrete multi-session recipes (post-P1)

| You | Session A (on `recess`) | Session B (on `recess-deploy`) |
| --- | --- | --- |
| Reviewing auto-PRs | — | — |
| Feature work on a service | Code, tests, commit, open PR. Wait on CI. | Stale until the merge triggers release.yml, which lands an auto-PR here. |
| Platform hardening (P2.1–P2.5) | — | All P2 items go here in parallel — NetworkPolicies in one session, Alertmanager in another, Velero in a third. They touch different namespaces and don't conflict. |
| Debugging a production issue | Log investigation, open a fix PR. | Inspect Application sync state, force refreshes, check values drift. Cross-reference with `/add-dir` if needed. |

**Rule of thumb post-split:** if the work is "what the cluster runs" →
`recess-deploy`. If the work is "what the app does" → `recess`. Almost
every task below falls cleanly on one side or the other.

### Why P1 itself is sequential

P1.1 (App) must exist before P1.2 (deploy repo uses it in the bypass
list). P1.2 must exist before P1.3 (release.yml writes to it). P1.3
must be verified end to end before P1.4 (we can't delete manifests from
`recess` before confirming the deploy repo is the real source).

That said, P1.1 and P1.2 *can* be worked on in parallel — the App doesn't
need the deploy repo to exist yet; the deploy repo can be seeded before
the App is fully configured. It's only P1.3 that has to wait on both.

## Suggested ordering for this week

Do these in order; items on the same line are safe to run in parallel.

```
Day 1    P0.4 (folder rename, 5 min)
         └─ then P0.1 (ruleset)
Day 1–2  P0.2 (release.yml PR flow)  ||  P0.3 (commit A.2 follow-ups)
Day 3    P0.5 (measure task #20 closure)
Day 4–5  P1.1 (GitHub App)           ||  P1.2 (create recess-deploy repo)
Day 6    P1.3 (wire release.yml)
Day 7    P1.4 (strip source repo) + P1.5 (tighten source ruleset)
```

After that, P2 items fan out to whichever session has bandwidth.
