# Governance + Backup + Alerting Plan

Complementary plan to `CI_CD_IMPROVEMENT_PLAN.md`. Covers four additions to the
k3s + ArgoCD + Helm platform:

- **Polaris** — manifest auditing against Kubernetes best practices.
- **Goldilocks** — resource request/limit recommendations via VPA.
- **Velero** — cluster-wide backup and disaster recovery.
- **Alertmanager + Discord** — runtime alerting strategy with severity-based routing.

Each task has a mandatory validation step before marking it complete.

**Legend:** `[ ]` pending · `[~]` in progress · `[x]` done and validated · `[!]` blocked

**Prerequisites:** Phases 5 (k3s + ArgoCD + Helm) and 6 (kube-prometheus-stack +
Loki + Tempo) from `CI_CD_IMPROVEMENT_PLAN.md` must be complete.

**Current status (2026-04-19):** Phase 5.4 canary is live (`auth-service` in
`recess`). Phase 5.5 (other 7 Go services + frontend) and Phase 6 (observability
stack) are still open — Phase D in this doc is blocked on Phase 6.

---

## Phase A — Polaris (manifest auditing)

Audits every workload against ~30 built-in best-practice checks (security,
resources, reliability, networking, images). Non-blocking by default — reports
only. Single deployment + dashboard.

### A.1 Install Polaris via ArgoCD

- **A.1.a** Add Helm chart source:
  - `helm repo add fairwinds-stable https://charts.fairwinds.com/stable`
- **A.1.b** Create `infra/k8s/apps/polaris/values.yaml`:
  - `dashboard.replicas: 1`
  - `dashboard.resources.requests: { cpu: 25m, memory: 64Mi }`
  - `dashboard.resources.limits: { cpu: 100m, memory: 128Mi }`
  - `dashboard.ingress.enabled: true` with host `polaris.<domain>`
  - `dashboard.basicAuth.enabled: true` (SealedSecret with credentials)
  - Disable `webhook` and `audit` subcharts for now (audit-only via dashboard).
- **A.1.c** Create ArgoCD `Application` in `infra/k8s/apps/polaris.yaml`
  pointing to the chart + values.
- **Validation A.1**
  - ArgoCD shows `polaris` app as Synced + Healthy.
  - `https://polaris.<domain>` returns the dashboard with basic auth prompt.

### A.2 Baseline audit and chart fixes

- **A.2.a** Capture the initial cluster-wide Polaris score as baseline.
  - Expected initial findings: missing `securityContext` fields,
    missing `PodDisruptionBudget`, possibly missing `readOnlyRootFilesystem`.
- **A.2.b** Update `infra/k8s/charts/go-service/` templates to fix:
  - `securityContext.runAsNonRoot: true`
  - `securityContext.runAsUser: 65532` (nonroot distroless UID)
  - `securityContext.readOnlyRootFilesystem: true`
  - `securityContext.allowPrivilegeEscalation: false`
  - `securityContext.capabilities.drop: [ALL]`
  - Optional `emptyDir` volume for `/tmp` if the service needs writable temp.
- **A.2.c** Add `templates/pdb.yaml` to the chart (`minAvailable: 1` when
  `replicaCount >= 2`).
- **A.2.d** Re-deploy all 8 services via ArgoCD sync.
- **Validation A.2**
  - Polaris cluster score ≥ 95% across `recess` namespace.
  - All 8 Go services show "passing" for all security checks.
  - Services still start and pass readiness probes (no regressions from
    `readOnlyRootFilesystem`).

### A.3 (Optional, later) Admission controller mode

Not for now. Document the upgrade path so it's not forgotten.

- **A.3.a** Add to `infra/k8s/README.md` a section "Enabling Polaris admission
  webhook" with the exact `values.yaml` changes needed.
- **A.3.b** Document the prerequisite: cluster score stable at 100% for 2 weeks,
  otherwise legitimate deploys will be blocked.
- **Validation A.3** — documentation reviewed; no actual install.

---

## Phase B — Goldilocks (resource right-sizing)

Observes real CPU/memory usage via VPA and recommends `requests`/`limits` per
workload. 3 pods (VPA recommender + Goldilocks controller + dashboard), all
under 50Mi RAM each.

### B.1 Install VPA + Goldilocks

- **B.1.a** Install VPA (required dependency):
  - `kubectl apply -f https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/...` OR
  - Use Goldilocks chart with `vpa.enabled: true` (installs VPA as subchart).
- **B.1.b** Create `infra/k8s/apps/goldilocks/values.yaml`:
  - `controller.enabled: true`
  - `dashboard.enabled: true`
  - `dashboard.ingress.enabled: true` with host `goldilocks.<domain>`
  - `dashboard.basicAuth.enabled: true` (SealedSecret)
  - `vpa.enabled: true`
  - Resources small: `requests: { cpu: 25m, memory: 32Mi }` per pod.
- **B.1.c** Create ArgoCD `Application` at `infra/k8s/apps/goldilocks.yaml`.
- **Validation B.1**
  - 3 pods Running in `goldilocks` namespace.
  - VPA CRDs present (`kubectl get crd | grep verticalpodautoscaler`).
  - Dashboard accessible at `https://goldilocks.<domain>`.

### B.2 Enable Goldilocks per namespace

- **B.2.a** Label target namespaces:
  - `kubectl label ns recess goldilocks.fairwinds.com/enabled=true`
  - `kubectl label ns recess-data goldilocks.fairwinds.com/enabled=true`
  - `kubectl label ns observability goldilocks.fairwinds.com/enabled=true`
- **B.2.b** Add the label as a default in `infra/k8s/namespaces.yaml` so it
  survives any namespace recreation.
- **B.2.c** Set Goldilocks QoS mode to `burstable` (default) — fits stateless
  services. Document in `infra/k8s/README.md` that `guaranteed` is appropriate
  only for Postgres/Redis if ever needed.
- **Validation B.2**
  - `kubectl get vpa -A` shows one VPA per workload in labeled namespaces.
  - Dashboard lists all 8 Go services + Postgres + Redis.

### B.3 Initial data collection + tuning pass

VPA needs 2–7 days of observation to produce meaningful recommendations.

- **B.3.a** Wait 7 days after B.2 completion with real traffic
  (E2E + manual testing).
- **B.3.b** Review dashboard recommendations for each service.
- **B.3.c** Apply recommended `requests`/`limits` to each service's
  `infra/k8s/apps/<service>/values.yaml`.
- **B.3.d** Commit changes with scope `chore(<service>): tune resources`.
- **B.3.e** Schedule recurring tuning review every 30 days (calendar reminder).
- **Validation B.3**
  - Cluster-wide memory `requests` sum is within 10% of actual p95 usage
    (check via `kubectl top nodes` vs sum of requests).
  - No OOMKills in the week following the tuning pass.

---

## Phase C — Velero (backup + disaster recovery)

Backs up manifests + PVC contents to S3-compatible storage. Enables full
cluster restore after SD-card failure or migration to a new Pi.

### C.1 Choose and provision backup storage

- **C.1.a** Pick S3-compatible backend. Options with free tiers:
  - **Cloudflare R2** (10GB free, no egress cost) — recommended.
  - **Backblaze B2** (10GB free, minimal egress).
  - **MinIO** on a secondary machine (if available).
- **C.1.b** Create bucket `recess-velero-backups`.
- **C.1.c** Generate access key + secret with write-only permissions on that
  bucket. Save as `credentials-velero` (local file, not in repo).
- **C.1.d** Document the chosen backend + endpoint in `infra/k8s/README.md`.
- **Validation C.1**
  - `aws s3 ls --endpoint-url=<endpoint>` lists the empty bucket.

### C.2 Install Velero via CLI

Velero's install step creates CRDs + CR definitions, so CLI install is cleaner
than Helm. After install, the `BackupStorageLocation` CR is committed to Git.

- **C.2.a** Install `velero` CLI on the workstation.
- **C.2.b** Run `velero install` with:
  - `--provider aws`
  - `--plugins velero/velero-plugin-for-aws:v1.9.0`
  - `--bucket recess-velero-backups`
  - `--backup-location-config region=auto,s3ForcePathStyle=true,s3Url=<endpoint>`
  - `--secret-file ./credentials-velero`
  - `--use-node-agent` (enables file-system backup for local-path PVCs)
  - `--default-volumes-to-fs-backup`
- **C.2.c** Export the generated manifests and commit to
  `infra/k8s/apps/velero/` (except the credentials secret, which is sealed).
- **C.2.d** Seal the credentials as SealedSecret `cloud-credentials` in
  `infra/k8s/secrets/`.
- **C.2.e** Convert the app to an ArgoCD `Application` for future updates.
- **Validation C.2**
  - `velero server` and `node-agent` DaemonSet pods Running.
  - `velero backup-location get` shows the location as `Available`.

### C.3 First manual backup

- **C.3.a** Run:
  ```
  velero backup create recess-initial \
    --include-namespaces recess,recess-data,argocd,observability \
    --wait
  ```
- **C.3.b** Check backup status: `velero backup describe recess-initial`.
- **C.3.c** Verify files in the R2/B2 bucket (should see `backups/recess-initial/`
  with tarballs).
- **Validation C.3**
  - Backup phase = `Completed`.
  - Errors = 0, Warnings = 0 (or understood and documented).
  - Bucket contains the backup artifacts.

### C.4 Restore test (DR drill)

This is the most important validation in the whole plan. Without it, Velero is
theoretical.

- **C.4.a** Provision a second k3s cluster (alternative: recreate the existing
  one, if you're brave). Options:
  - Second Raspberry Pi flashed fresh.
  - Local k3d/kind cluster on the workstation (fine for the drill; ignore
    arm64-only images by building amd64 versions of the relevant services, OR
    test with a single small service to prove the restore flow).
- **C.4.b** Install only Velero on the fresh cluster pointing to the same
  bucket.
- **C.4.c** `velero backup get` — should list `recess-initial` from the bucket.
- **C.4.d** `velero restore create --from-backup recess-initial`.
- **C.4.e** Watch pods come up: `kubectl get pods -A -w`.
- **C.4.f** Document what didn't work (expected: things like LoadBalancer IPs,
  node-specific annotations, cloudflared tunnel token, etc.).
- **C.4.g** Add the drill procedure to `infra/k8s/README.md` as "Disaster
  Recovery Runbook".
- **Validation C.4**
  - ArgoCD, Postgres (with its data), Redis, and at least one Go service are
    Running on the fresh cluster.
  - Runbook in the repo reflects real gotchas from the drill, not theory.

### C.5 Scheduled daily backups

- **C.5.a** Create `Schedule` CR at `infra/k8s/apps/velero/schedules.yaml`:
  - Daily: `0 3 * * *`, retention 30 days, all app namespaces.
  - Weekly: `0 4 * * 0`, retention 90 days, full cluster (excludes
    `kube-system`, `velero` itself).
- **C.5.b** Wait 48h and verify 2 daily backups exist.
- **C.5.c** Add an alert (wired in Phase D) for `backup.status.phase != Completed`.
- **Validation C.5**
  - `velero backup get` shows scheduled backups accumulating.
  - Oldest backup beyond retention is auto-deleted after the TTL.

---

## Phase D — Alertmanager + Discord

kube-prometheus-stack (from Phase 6.1) already ships Alertmanager. This phase
configures severity-based routing to 5 Discord channels and authors the initial
alert rule set.

### D.1 Create Discord channels + webhooks

- **D.1.a** In the Recess Discord server, create 5 channels:
  - `#deploys` — ArgoCD deploy events.
  - `#alerts-critical` — P1, ping-on.
  - `#alerts-warning` — P2, ping-off.
  - `#alerts-info` — P3, informational.
  - `#alerts-firehose` — copy of everything, for debugging.
- **D.1.b** For each channel, create a webhook in
  "Channel Settings → Integrations → Webhooks". Save the 5 URLs.
- **D.1.c** Store webhook URLs in a local `discord-webhooks.env` file
  (not committed).
- **Validation D.1**
  - `curl -X POST -H "Content-Type: application/json" -d '{"content":"test"}' <webhook-url>`
    posts a message to each channel.

### D.2 SealedSecret for webhooks

- **D.2.a** Create a Secret `alertmanager-discord-webhooks` with 5 keys:
  `critical`, `warning`, `info`, `firehose`, `deploys`.
- **D.2.b** Seal it as `infra/k8s/secrets/alertmanager-discord-webhooks.yaml`.
- **D.2.c** Reference the Secret in Alertmanager config via
  `webhook_url_file: /etc/alertmanager/secrets/alertmanager-discord-webhooks/<key>`.
- **Validation D.2**
  - `kubectl get secret alertmanager-discord-webhooks -n observability`
    returns the 5 keys decoded correctly.

### D.3 Alertmanager routing configuration

Configure via `AlertmanagerConfig` CR (native to kube-prometheus-stack).

- **D.3.a** Create `infra/k8s/apps/observability/alertmanager-config.yaml`:
  - Root route → `discord-firehose`.
  - Sub-routes with `continue: true`:
    - `severity=critical` → `discord-critical`.
    - `severity=warning` → `discord-warning`.
    - `severity=info` → `discord-info`.
  - `group_by: ['alertname', 'namespace', 'service']`.
  - `group_wait: 30s`, `group_interval: 5m`, `repeat_interval: 4h` for warning.
  - `repeat_interval: 1h` for critical (more aggressive re-notification).
- **D.3.b** Define 4 receivers using Alertmanager's Discord integration
  (`discord_configs`):
  - Critical template prepends `@here` to trigger audible Discord notification.
  - Warning / info / firehose templates do not ping.
  - Each template includes: alertname, severity, namespace, summary,
    description, link to Grafana panel (via `externalURL` + query).
- **D.3.c** Configure inhibition rules:
  - Critical alert for a namespace inhibits warning alerts in the same
    namespace (avoid double-paging during an incident).
- **Validation D.3**
  - `amtool config check` on the rendered config returns OK.
  - Alertmanager UI shows the 4 receivers and the 4 routes.

### D.4 Initial PrometheusRule set

Split into 3 `PrometheusRule` CRs by domain for maintainability.

- **D.4.a** `infra/k8s/apps/observability/rules-infra.yaml`:
  - `NodeMemoryAvailableLow` (critical): `node_memory_MemAvailable_bytes < 500MB` for 2m.
  - `NodeDiskUsageHigh` (warning): `disk usage > 70%` for 10m.
  - `NodeDiskUsageCritical` (critical): `disk usage > 90%` for 2m.
  - `NodeCPUHigh` (warning): `1 - rate(node_cpu_seconds_total{mode="idle"}[5m]) > 0.85` for 15m.
- **D.4.b** `infra/k8s/apps/observability/rules-workloads.yaml`:
  - `PodCrashLooping` (critical): `kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"} == 1` for 5m.
  - `PodOOMKilled` (warning): increase in `container_oom_events_total` > 0 over 10m.
  - `PodRestartsHigh` (warning): increase in `kube_pod_container_status_restarts_total` > 3 over 1h.
  - `PodMemoryPressure` (warning): `container_memory_working_set_bytes / container_spec_memory_limit_bytes > 0.80` for 10m.
  - `PodCPUThrottled` (warning): `rate(container_cpu_cfs_throttled_seconds_total[5m]) > 0.25` for 15m.
- **D.4.c** `infra/k8s/apps/observability/rules-app.yaml` — app-specific:
  - `HighErrorRate` (critical): service HTTP 5xx rate > 5% for 5m (per service).
  - `HighLatencyP95` (warning): p95 latency > 500ms for 15m (game-server, match-service, ws-gateway).
  - `PostgresDown` (critical): `cnpg_pg_up == 0` for 1m.
  - `RedisDown` (critical): `redis_up == 0` for 1m.
  - `WebSocketConnectionsDropped` (critical): 50% drop in active WS connections over 1m.
  - `ArgoCDAppOutOfSync` (warning): `argocd_app_info{sync_status!="Synced"} == 1` for 60m.
  - `ArgoCDAppDegraded` (critical): `argocd_app_info{health_status="Degraded"} == 1` for 5m.
  - `VeleroBackupFailed` (warning): `velero_backup_failure_total` increased in the last 24h.
  - `TrivyHighVulnerability` (warning): new `CRITICAL` or `HIGH` finding in Trivy reports.
- **D.4.d** Attach `severity` label to every rule.
- **D.4.e** Attach `runbook_url` annotation pointing to a section in the
  `infra/k8s/README.md` runbook (even if it's a stub).
- **Validation D.4**
  - `promtool check rules` on each file returns OK.
  - Prometheus UI `/rules` lists all rules as loaded.

### D.5 Dead-man heartbeat (external monitor)

Catches the case where Alertmanager itself is down and therefore silent.

- **D.5.a** Sign up for **healthchecks.io** (free tier, 20 checks).
- **D.5.b** Create a check "recess-alertmanager-heartbeat" with expected
  interval 5m, grace period 10m.
- **D.5.c** Add a Prometheus rule "Watchdog" that always fires
  (`expr: vector(1)`), with label `severity: heartbeat`.
- **D.5.d** Add a dedicated receiver in Alertmanager that hits the
  healthchecks.io webhook URL on every group interval of the Watchdog.
  Route `severity=heartbeat` → this receiver, no other destinations.
- **D.5.e** Configure healthchecks.io to email + Discord webhook to
  `#alerts-critical` if the heartbeat stops.
- **Validation D.5**
  - healthchecks.io dashboard shows "up" state.
  - Kill the Alertmanager pod manually → within 15m, healthchecks.io fires and
    a message reaches `#alerts-critical` via the external path.

### D.6 Alert noise tuning (ongoing)

First 2 weeks of real alerting are expected to be noisy. Budget time to tune.

- **D.6.a** Track every critical alert in a log (GitHub issue or shared doc)
  with: time, duration, was it actionable, was it already known.
- **D.6.b** For every critical alert that turns out non-actionable, either
  downgrade to warning or tighten the threshold / duration.
- **D.6.c** Target state: `#alerts-critical` receives 0–2 messages per month.
  More than 5/month = the rules need tuning, not Discord acclimation.
- **Validation D.6**
  - After 30 days, post in `#alerts-info` a summary of alert counts per
    severity and per rule. Review top 3 noisiest rules and adjust.

### D.7 ArgoCD Notifications → Discord

Already scoped in the main plan (`5.13.c`), completed here for consistency.

- **D.7.a** Enable `argocd-notifications` (built into ArgoCD ≥ 2.3).
- **D.7.b** Configure `argocd-notifications-cm` with:
  - `service.webhook.discord-deploys` — webhook URL via `$discord-deploys-webhook`.
  - `service.webhook.discord-alerts` — webhook URL via `$discord-alerts-webhook`.
  - Templates `app-deployed`, `app-sync-failed`, `app-health-degraded`.
  - Triggers wired: `on-deployed` → discord-deploys; `on-sync-failed` and
    `on-health-degraded` → discord-alerts-warning.
  - Default subscriptions (no per-Application annotations).
- **D.7.c** Optionally configure Grafana annotation service so deploys appear
  as vertical lines on dashboards.
- **D.7.d** Seal the webhook tokens as `argocd-notifications-secret`.
- **Validation D.7**
  - Deploy a trivial change → `#deploys` gets a message within 2 min.
  - Break a manifest on purpose → `#alerts-warning` gets the sync failure.
  - Grafana dashboards show a vertical annotation line on deploys.

---

## Cross-cutting validation

After all four phases are complete, run a coordinated "GameDay" to validate end
to end.

- **X.1** Polaris score ≥ 95% cluster-wide.
- **X.2** Goldilocks recommendations applied; no OOMKills in the past 7 days.
- **X.3** Velero daily backup succeeded for 14 consecutive days.
- **X.4** Restore drill executed in the last 60 days with documented result.
- **X.5** Induced failure drill:
  - Kill the Redis pod → `#alerts-critical` fires within 2 min.
  - Delete it again after recovery → alert auto-resolves.
- **X.6** Alertmanager kill drill:
  - Scale Alertmanager to 0 replicas.
  - Within 15 min, healthchecks.io fires a message into `#alerts-critical`
    via its external path.
  - Scale back up.
- **X.7** Deploy drill:
  - Push a `feat(game-server): ...` commit.
  - Observe: release PR → merge → tag → CD → ArgoCD sync → rolling update →
    `#deploys` message → Grafana annotation.
  - Total time from commit to healthy pods recorded in the baseline table.

---

## Resource footprint

Running totals added on top of the main plan's cluster:

| Component      | Pods        | Approx. RAM |
|----------------|-------------|-------------|
| Polaris        | 1           | ~100 Mi     |
| Goldilocks     | 3 (+ VPA)   | ~150 Mi     |
| Velero         | 1 + DaemonSet | ~200 Mi  |
| Alertmanager rules and routing | 0 new (reuses kube-prometheus-stack) | 0 |
| **Total extra**| ~5 + DS     | ~450 Mi     |

Comfortably within budget on a Raspberry Pi 5 (8 GB). Together with the base
platform, expected cluster RAM usage is ~3–4 GB, leaving headroom for the
actual app traffic.

---

## Notes / decisions

- Polaris runs audit-only; admission mode deferred until the score is stable.
- Goldilocks mode is `burstable` for stateless services; Postgres/Redis remain
  manually tuned (statefulness makes VPA recommendations less trustworthy).
- Velero backs up to S3-compatible external storage (R2 recommended) — never
  to the same Pi, since SD-card failure is the top DR scenario.
- Discord is used over Slack due to free-tier constraints; the severity-based
  channel structure is identical to what's used with Slack in production
  environments.
- Heartbeat via healthchecks.io is the only component outside the cluster, by
  design: it catches the "alerts stopped arriving" failure mode that internal
  monitoring cannot detect.
- All alert rules carry a `runbook_url` annotation pointing to a section in
  `infra/k8s/README.md`, even if the section is a stub at first — it forces
  documentation growth alongside the rule set.
