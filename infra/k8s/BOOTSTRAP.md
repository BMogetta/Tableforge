# Recess — k3s Cluster Bootstrap (Raspberry Pi 5)

Reproducible steps to bring up the k3s + ArgoCD + Helm cluster described in Phase 5 of `CI_CD_IMPROVEMENT_PLAN.md`. Each section is added after being validated on real hardware.

**Target**: single-node k3s server on Raspberry Pi 5 (arm64), Debian 13 trixie.

> Note: `CI_CD_IMPROVEMENT_PLAN.md` predates the TableForge → Recess rebrand and still uses `tableforge-*` identifiers (node name, image tags). This document uses `recess-*` as the canonical form.

---

## 5.1 k3s cluster bootstrap

### 5.1.a — Node prerequisites

**Node baseline** (validated 2026-04-18):

| Check | Command | Expected result |
| --- | --- | --- |
| Architecture | `uname -m` | `aarch64` |
| OS | `grep PRETTY_NAME /etc/os-release` | Debian/Raspberry Pi OS 64-bit |
| RAM | `free -h` | >= 4Gi (Pi 5 16GB reports 15Gi usable) |
| CPU cores | `nproc` | 4 (Pi 5) |
| cgroup hierarchy | `stat -fc %T /sys/fs/cgroup/` | `cgroup2fs` |
| cgroup controllers | `cat /sys/fs/cgroup/cgroup.controllers` | must include `cpu memory` |

**cgroup flags in `cmdline.txt`**:

Legacy Raspberry Pi OS required `cgroup_memory=1 cgroup_enable=memory` to enable the memory controller under cgroup v1. Debian trixie ships cgroup v2 unified hierarchy with `memory` enabled by default, but we add the flags anyway because:

- They are the official k3s recommendation.
- They are no-ops under v2 (harmless).
- They cover the fallback case of a forced v1 downgrade.

```bash
sudo cp /boot/firmware/cmdline.txt /boot/firmware/cmdline.txt.bak
sudo sed -i 's/rootwait/rootwait cgroup_memory=1 cgroup_enable=memory/' /boot/firmware/cmdline.txt
cat /boot/firmware/cmdline.txt  # verify: single line, rootwait + new flags present
sudo reboot
```

After reboot, confirm:

```bash
stat -fc %T /sys/fs/cgroup/           # cgroup2fs
cat /sys/fs/cgroup/cgroup.controllers # cpuset cpu io memory pids
```

**Troubleshooting**:

- `/proc/cgroups` does not list memory → expected under v2 (only shows remaining v1 controllers). Not an error.
- `cmdline.txt` ends up on 2+ lines → edit by hand. It must stay a SINGLE LINE or the kernel ignores the flags.

### 5.1.b — Install k3s (single-node server)

**Pinned version**: `v1.34.6+k3s1` (stable channel as of 2026-04-18). Re-pin by fetching the channel tip:

```bash
curl -s https://update.k3s.io/v1-release/channels/stable | grep -oP 'v\d+\.\d+\.\d+\+k3s\d+' | head -1
```

**Install**:

```bash
curl -sfL https://get.k3s.io | \
  INSTALL_K3S_VERSION=v1.34.6+k3s1 \
  INSTALL_K3S_EXEC="--write-kubeconfig-mode=644 --node-name=recess-pi" \
  sh -
```

Install flags:

- `INSTALL_K3S_VERSION` — pins the binary version for reproducibility. Without this, the script pulls whatever is current on the channel.
- `--write-kubeconfig-mode=644` — makes `/etc/rancher/k3s/k3s.yaml` world-readable so the non-root user can run `kubectl` without `sudo`.
- `--node-name=recess-pi` — stable node identifier, used in Helm values and `kubectl` output.
- **Traefik is kept** (no `--disable=traefik`). k3s ships Traefik v2 as the default ingress controller; we reuse it as the L7 router in front of the services, matching the current docker-compose topology.

**Validation**:

```bash
sudo systemctl is-active k3s                    # active
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
kubectl get nodes -o wide                       # recess-pi Ready, v1.34.6+k3s1
kubectl get pods -A                             # see checklist below
kubectl top node                                # metrics-server reports CPU + memory
```

Expected pods in `kube-system` (all `Running` except Jobs which finish `Completed`):

- `coredns-*` — cluster DNS
- `local-path-provisioner-*` — default StorageClass (hostPath-based)
- `metrics-server-*` — feeds `kubectl top`
- `traefik-*` — ingress controller
- `svclb-traefik-*` — k3s klipper-lb (exposes Traefik on the node IP)
- `helm-install-traefik-*` — one-shot Jobs that install the Traefik Helm chart; stay as `Completed`

**Idle baseline** (measured 2026-04-18 right after install, no workloads):

```text
kubectl top node
NAME        CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
recess-pi   49m          1%       946Mi           5%
```

That is with coredns + metrics-server + local-path-provisioner + traefik + svclb-traefik running. Use it as the k3s-only reference point — add-ons (ArgoCD, CNPG, Redis, observability stack, the 8 services + frontend) will grow this.

**Uninstall / reinstall**:

If the node name or install flags need to change, reinstall is clean and fast:

```bash
sudo /usr/local/bin/k3s-uninstall.sh    # removes systemd unit, binary, /etc/rancher/k3s, /var/lib/rancher/k3s
# ... then re-run the curl | sh install
```

Alternatively, re-running the `curl | sh` install without uninstall is idempotent — the script overwrites the systemd unit and restarts k3s. The existing data directory is preserved.

### 5.1.c — Workstation kubeconfig

The k3s-generated kubeconfig lives on the Pi at `/etc/rancher/k3s/k3s.yaml`. It embeds the cluster CA and an admin client cert, and points at `https://127.0.0.1:6443` (loopback). To use it from the workstation we need to (a) copy it locally, (b) replace the server URL with the Pi's LAN IP, (c) rename the default context for clarity.

**Prerequisite**: matching `kubectl` on the workstation. Client skew vs. server must be within ±1 minor per k8s policy. Cluster is `v1.34.6+k3s1`, so use client `v1.34.x`:

```bash
# Official binary pinned, installed to ~/.local/bin (must be ahead of /usr/local/bin in PATH)
cd /tmp
curl -sLO https://dl.k8s.io/release/v1.34.6/bin/linux/amd64/kubectl
curl -sLO https://dl.k8s.io/release/v1.34.6/bin/linux/amd64/kubectl.sha256
echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check    # must print "kubectl: OK"
chmod +x kubectl && mv kubectl ~/.local/bin/kubectl
hash -r
kubectl version --client                                     # Client Version: v1.34.6
```

On Docker Desktop WSL setups the default `/usr/local/bin/kubectl` is a symlink into `/mnt/wsl/docker-desktop/cli-tools/…` and tracks whatever Docker Desktop ships. Putting the pinned binary in `~/.local/bin` (which appears earlier in PATH) overrides it cleanly without fighting Docker Desktop.

**Copy and rewrite the kubeconfig**:

```bash
PI_IP=192.168.1.146   # LAN address of the Pi; confirm with `kubectl get nodes -o wide` from the Pi
mkdir -p ~/.kube
scp bmogetta@${PI_IP}:/etc/rancher/k3s/k3s.yaml ~/.kube/config-recess
sed -i "s|https://127.0.0.1:6443|https://${PI_IP}:6443|" ~/.kube/config-recess
kubectl --kubeconfig ~/.kube/config-recess config rename-context default recess
chmod 600 ~/.kube/config-recess
```

**Validation**:

```bash
KUBECONFIG=~/.kube/config-recess kubectl get nodes -o wide
# NAME        STATUS   ROLES           AGE   VERSION        INTERNAL-IP     ...
# recess-pi   Ready    control-plane   11m   v1.34.6+k3s1   192.168.1.146
```

**Making the config the default**:

Two options. Pick one.

1. **Per-session export** (simplest, non-invasive):

   ```bash
   export KUBECONFIG=~/.kube/config-recess
   ```

   Add to `~/.zshrc` / `~/.bashrc` to make it stick.

2. **Merge into `~/.kube/config`** (kubectl's default path, coexists with other clusters):

   ```bash
   KUBECONFIG=~/.kube/config:~/.kube/config-recess kubectl config view --flatten > ~/.kube/config.new
   mv ~/.kube/config.new ~/.kube/config
   chmod 600 ~/.kube/config
   kubectl config use-context recess
   ```

**Rotation**: if the Pi's IP changes, re-run the `sed` line on `~/.kube/config-recess` (or the merged `~/.kube/config`). If the cluster is reinstalled, the embedded client cert changes — repeat the full scp flow.

### 5.1.d — Helm on the workstation

Helm runs on the workstation (not on the Pi) — all chart installs are pushed to the cluster via the workstation's kubeconfig.

Current version on this machine: **v3.13.0** (pre-existing). Kept as-is; all charts we will install (argo-cd, cnpg, bitnami/redis, kube-prometheus-stack) support v3.x since 3.8. If a future chart requires a newer Helm, upgrade then — not preemptively.

Verify:

```bash
helm version
# version.BuildInfo{Version:"v3.13.0", ...}
```

### 5.1.e — Storage: local-path-provisioner

k3s ships Rancher's `local-path-provisioner` enabled by default. It creates `hostPath` PVs under `/var/lib/rancher/k3s/storage/` on the node — no extra install needed for homelab-scale workloads.

Verified on the live cluster:

```text
kubectl get storageclass
NAME                   PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
local-path (default)   rancher.io/local-path   Delete          WaitForFirstConsumer   false                  13m
```

Notes:

- **Default class** — any PVC without an explicit `storageClassName` lands here.
- **`WaitForFirstConsumer`** — the PV is only provisioned once a Pod schedules and requests the PVC. Normal for node-local storage.
- **`Delete` reclaim policy** — when a PVC is deleted, the underlying host directory is removed. Fine for stateless recreate-on-replace workflows; Postgres/Redis PVCs must be handled carefully (never `kubectl delete pvc` without a backup).
- **`AllowVolumeExpansion: false`** — capacity cannot be grown in place. Plan PVC sizes with headroom (Postgres 20Gi, Redis 5Gi per the plan are fine for the Pi 5's SD/NVMe).

For any workload that needs to outlive the node's local disk (replication, faster disks, external NVMe), a different provisioner would be needed — out of scope for single-node homelab.

### 5.1.f — Backups (deferred)

Node-state backups are **deferred until there is data worth recovering**. Rationale:

- The cluster is currently empty — recreating it from scratch is faster than restoring.
- All cluster configuration lives in this repo (Helm charts, Application CRs, SealedSecrets). A full wipe is recoverable from `git` + a re-run of this BOOTSTRAP document.
- No external disk / NAS currently attached to the Pi to serve as a backup target.

**Re-enable when any of these land**:

1. CNPG Postgres cluster with user data (Phase 5.6).
2. SealedSecrets with irreplaceable secret material (Phase 5.9).
3. Grafana dashboards edited in UI instead of git (Phase 6).

**When re-enabled, the minimum viable backup is**:

```bash
# On the Pi — cron @daily
sudo tar -czf /path/to/backup-target/k3s-$(date +%F).tar.gz \
    /var/lib/rancher/k3s/server/db \
    /var/lib/rancher/k3s/server/token \
    /var/lib/rancher/k3s/server/tls
# rotate / offsite as desired
```

For a single-node k3s server with the default SQLite backend (no `--cluster-init`), `server/db/state.db` is the full datastore — losing it means losing the cluster. `server/tls/` holds the CA the kubeconfig trusts; losing it forces reissuing client configs.

---

## 5.1 — Validation checklist

All boxes must be ticked before moving on to Phase 5.2.

- [x] `uname -m` on the Pi returns `aarch64`
- [x] `/sys/fs/cgroup/cgroup.controllers` on the Pi includes `cpu memory`
- [x] `systemctl is-active k3s` on the Pi returns `active`
- [x] `kubectl get nodes` from the workstation lists `recess-pi Ready`
- [x] `kubectl top node` reports non-zero CPU and memory
- [x] `kubectl get pods -A` shows coredns, local-path-provisioner, metrics-server, traefik, svclb-traefik all `Running`
- [x] `kubectl get storageclass` lists `local-path (default)`
- [ ] Backups — deferred; re-check when entering Phase 5.6 / 5.9 / 6

---

## 5.2 Namespaces and conventions

### 5.2.a — Apply the namespace manifest

All namespaces live in a single declarative file at `infra/k8s/namespaces.yaml` (not created imperatively with `kubectl create ns`). This keeps the cluster recreatable from git.

```bash
export KUBECONFIG=~/.kube/config-recess
kubectl apply -f infra/k8s/namespaces.yaml
kubectl get ns --show-labels
```

Expected: five namespaces in `Active` state with the labels below.

### 5.2.b — Convention

| Namespace | `recess.io/tier` | Owns |
| --- | --- | --- |
| `recess` | `app` | Eight Go services + frontend (Phases 5.4, 5.5) |
| `recess-data` | `data` | CNPG Postgres (5.6), Redis (5.7) |
| `observability` | `platform` | kube-prometheus-stack, Loki, Tempo, OTel Collector (Phase 6) |
| `argocd` | `platform` | ArgoCD server + controllers (5.3) |
| `cloudflared` | `ingress` | Cloudflare Tunnel daemon (5.8) |

**Shared labels** on every namespace:

- `app.kubernetes.io/part-of: recess` — groups everything that belongs to this project for bulk selection (e.g., `kubectl get pods -A -l app.kubernetes.io/part-of=recess`).
- `recess.io/tier: <tier>` — targeted selector for future NetworkPolicies. The planned rule set is: `tier=app` can reach `tier=data`; nothing else can reach `tier=data`; `tier=ingress` can reach `tier=platform` (for the ArgoCD UI) and `tier=app`.

**Why a single manifest instead of separate files**: 5 namespaces is not enough volume to justify a directory. If the count grows past ~10 or namespaces start carrying more configuration (ResourceQuotas, LimitRanges, NetworkPolicies), split into `infra/k8s/namespaces/<name>.yaml`.

### 5.2 — Validation

- [x] `kubectl get ns` lists `recess`, `recess-data`, `observability`, `argocd`, `cloudflared` as `Active`
- [x] Every namespace carries `app.kubernetes.io/part-of=recess` and a `recess.io/tier` label

---

## 5.3 ArgoCD

Single-instance ArgoCD for homelab. No OIDC yet, no Slack/email notifications, no HA Redis. Access is via `kubectl port-forward` in this phase; public exposure via Cloudflare Tunnel lands in Phase 5.8.

### 5.3.a — Add the chart repo

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm search repo argo/argo-cd --versions | head -3
```

Pinned chart: **`argo/argo-cd 9.5.2`** (app version `v3.3.7`). Re-pin by re-running the `helm search` and editing `infra/k8s/argocd/values.yaml` header comment + the install command in 5.3.b.

### 5.3.b — Install

Values live in `infra/k8s/argocd/values.yaml`. Key settings and why:

| Setting | Value | Reason |
| --- | --- | --- |
| `configs.params.server.insecure` | `true` | No TLS inside the cluster. Traefik + cloudflared terminate TLS externally (Phase 5.8). |
| `redis-ha.enabled` | `false` | Single-node cluster — HA Redis would schedule three pods that cannot colocate. |
| `dex.enabled` | `false` | No OIDC login yet; admin password is enough. Saves one pod. |
| `notifications.enabled` | `false` | No alerting channels configured. Saves one pod. |
| `*.replicas` | `1` | Single-instance across every component. |
| `*.resources.*` | small | Conservative requests/limits — see the values file for per-component numbers. |

Install:

```bash
helm upgrade --install argocd argo/argo-cd --version 9.5.2 --namespace argocd -f infra/k8s/argocd/values.yaml --wait --timeout 10m
```

The `--wait` flag blocks until every Deployment / StatefulSet is `ready`. Expect 2-4 minutes on a Pi 5 (image pulls + probe warm-up).

**Validation**:

```bash
kubectl -n argocd get pods
kubectl -n argocd get svc
```

Five long-lived pods must be `Running` (one-shot Jobs like `argocd-redis-secret-init-*` finish `Completed`):

- `argocd-application-controller-0` (StatefulSet)
- `argocd-applicationset-controller-*`
- `argocd-redis-*`
- `argocd-repo-server-*`
- `argocd-server-*`

Four ClusterIP services: `argocd-server` (80/443), `argocd-redis` (6379), `argocd-repo-server` (8081), `argocd-applicationset-controller` (7000).

**Accessing the UI** (port-forward, leave running in a dedicated terminal):

```bash
KUBECONFIG=~/.kube/config-recess kubectl -n argocd port-forward svc/argocd-server 8080:80
```

Retrieve the auto-generated admin password:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d; echo
```

Open <http://localhost:8080>, log in as `admin` + the password above. The initial-admin secret is meant to be deleted once you set your own password or wire OIDC — `kubectl -n argocd delete secret argocd-initial-admin-secret` when ready.

### 5.3.c — Register this repo

For a public repo no credentials are needed, but registering it as a `Secret` with label `argocd.argoproj.io/secret-type: repository` makes it show up in ArgoCD's Settings → Repositories and simplifies the switch back to private (add `password` / `sshPrivateKey` to the same Secret).

Manifest: `infra/k8s/argocd-apps/repo.yaml`.

```bash
kubectl apply -f infra/k8s/argocd-apps/repo.yaml
```

In the UI, `Settings → Repositories` lists `https://github.com/BMogetta/recess.git` with `Connection Status: Successful`.

### 5.3.d — AppProject + Root App-of-Apps

**AppProject `recess`** (`infra/k8s/argocd-apps/project.yaml`) — narrower scoping than the built-in `default` Project:

- `sourceRepos`: only `https://github.com/BMogetta/recess.git`. An Application inside this Project cannot reference any other git URL.
- `destinations`: only the five project namespaces + `kube-system` / `cnpg-system` (where operators live). Prevents a misconfigured Application from landing in a random namespace.
- Resource whitelists kept permissive (`*/*`) — the cluster runs operators that create arbitrary CRDs / ClusterRoles; tightening is a later hardening task.

Every Recess Application sets `spec.project: recess`.

**Root `Application`** (`infra/k8s/argocd-apps/root.yaml`) — plain `Application` + directory sync, not an ApplicationSet. Points at `infra/k8s/apps/` in git; ArgoCD reads every YAML under that path as a child Application.

> Note on the original plan: 5.3.d called for an `ApplicationSet`, but 5.4.d prescribes a full `Application` CR per service file — which is incompatible with ApplicationSet's git-file generator (where files are template-var configs, not CRs). The canonical App-of-Apps pattern from the ArgoCD docs uses a plain `Application` root. Documented here as the resolved approach; 5.3.d's phrasing was imprecise.

**Sync policy**: `automated: { prune: true, selfHeal: true }`. Together they mean (a) drift from git is silently reverted on the cluster, and (b) a file removed from `infra/k8s/apps/` deletes the corresponding child Application (and, transitively, its workload) on the next reconcile.

**Finalizer**: `resources-finalizer.argocd.argoproj.io`. Without it, deleting the root Application leaves children orphaned.

Apply (order matters — Project must exist before the root references it):

```bash
kubectl apply -f infra/k8s/argocd-apps/project.yaml -f infra/k8s/argocd-apps/repo.yaml -f infra/k8s/argocd-apps/root.yaml
```

> On apply you may see a k8s warning:
> `metadata.finalizers: "resources-finalizer.argocd.argoproj.io": prefer a domain-qualified finalizer name including a path (/)`
> — Cosmetic. The name is ArgoCD's own well-known value and will be fixed upstream. A future chart upgrade will replace it.

Expected state in the UI within ~30 s:

- `Settings → Projects` lists `recess` alongside `default`.
- Applications list shows `recess-root` under project `recess`, with **Sync Status: Synced**, **Health: Healthy**, **0 child resources** (the `apps/` path only contains `.gitkeep`).

As Phase 5.4+ adds child Application CRs to `infra/k8s/apps/`, the root automatically picks them up — and they too must declare `spec.project: recess` to satisfy the Project's scope.

### 5.3.e — Resource footprint

Idle footprint measured 2026-04-18, a few minutes after ArgoCD install, no child Applications yet:

```text
kubectl top pods -n argocd
NAME                                                CPU(cores)   MEMORY(bytes)
argocd-application-controller-0                     6m           81Mi
argocd-applicationset-controller-6685cdfd44-cglt7   2m           23Mi
argocd-redis-6d8578cb6f-cbqrt                       8m           6Mi
argocd-repo-server-6f7b79498f-fhb85                 2m           32Mi
argocd-server-78db985d75-5ptcp                      1m           42Mi
```

Pod totals: **19m CPU, 184Mi memory**.

Node totals (k3s baseline + ArgoCD):

```text
kubectl top node
NAME        CPU(cores)   CPU(%)   MEMORY(bytes)   MEMORY(%)
recess-pi   72m          1%       1509Mi          9%
```

Node delta vs. the 5.1.b baseline (49m / 946Mi): **+23m CPU, +563Mi memory**. The memory delta is larger than the pod RSS sum because `kubectl top pod` reports container working-set only — container init, image decompression, and containerd overhead add on top. Normal.

Plenty of headroom on a 16GB Pi 5 — the eight Go services, frontend, Postgres, Redis, and observability stack will fit well within budget.

### 5.3 — Validation

- [x] `helm list -n argocd` shows `argocd 9.5.2` deployed
- [x] All five ArgoCD pods are `Running`
- [x] Admin login via port-forward succeeds at `http://localhost:8080`
- [x] Settings → Repositories lists the repo as `Successful`
- [x] `recess-root` Application is `Synced` + `Healthy` with zero child resources
- [ ] GitHub OAuth — deferred (Phase 5.3.b roadmap; enable when ready)
- [ ] Public exposure via cloudflared — deferred to Phase 5.8

---

## 5.6 PostgreSQL (CloudNativePG)

Primary datastore. Managed by the CloudNativePG operator (CNPG) running in the `cnpg-system` namespace; the actual Postgres instance runs in `recess-data` as a single-instance Cluster CR.

### 5.6.a — CNPG operator

The operator is installed by Helm directly (not through ArgoCD) — it is platform, not application workload. Phase 5.4+ services are ArgoCD-managed; operators are bootstrapped once.

```bash
helm repo add cnpg https://cloudnative-pg.github.io/charts
helm repo update
helm search repo cnpg/cloudnative-pg --versions | head -3
```

Pinned chart: **`cnpg/cloudnative-pg 0.28.0`** (app version `1.29.0`).

Values in `infra/k8s/cnpg/values.yaml` only trim request sizes and turn off the PodMonitor (Prometheus lands in Phase 6).

```bash
helm upgrade --install cnpg cnpg/cloudnative-pg --version 0.28.0 --namespace cnpg-system -f infra/k8s/cnpg/values.yaml --wait --timeout 5m
```

**Validation**:

```bash
kubectl -n cnpg-system get pods
kubectl get crds | grep cnpg
```

Expect one `cnpg-cloudnative-pg-*` pod Running and the full CRD set:
`backups`, `clusterimagecatalogs`, `clusters`, `databases`, `failoverquorums`, `imagecatalogs`, `poolers`, `publications`, `scheduledbackups`, `subscriptions` — all under `postgresql.cnpg.io`.

### 5.6.b — Cluster CR

Manifest: `infra/k8s/data/postgres/cluster.yaml`. ArgoCD picks this up through the `postgres` Application (`infra/k8s/apps/postgres.yaml`) which the root App-of-Apps materialises.

Key settings:

| Field | Value | Reason |
| --- | --- | --- |
| `spec.instances` | `1` | Single-node homelab. Scale by bumping to 2+ on a multi-node cluster; CNPG provisions a standby automatically. |
| `spec.imageName` | `ghcr.io/cloudnative-pg/postgresql:16.6` | PG 16 pinned to a specific minor. CNPG publishes arm64 images — verified Running on Pi 5. |
| `spec.storage` | `local-path`, `20Gi` | Uses the default k3s StorageClass. WaitForFirstConsumer, so the PV lands wherever the primary pod is scheduled (only one node, so always this one). |
| `spec.resources` | req `100m / 256Mi`, lim `1000m / 1Gi` | Postgres tunes `shared_buffers` off `limits.memory` — 1Gi gives CNPG room to set ~256Mi of shared buffers. |
| `spec.bootstrap.initdb.database` | `recess` | Application database. |
| `spec.bootstrap.initdb.owner` | `recess` | Application user. CNPG creates the role and puts the credentials into the `pg-app` Secret. |
| `spec.bootstrap.initdb.postInitApplicationSQL` | `CREATE SCHEMA IF NOT EXISTS users/ratings AUTHORIZATION recess` | Three schemas end up owned by `recess`: `public` (implicit), `users`, `ratings`. |
| `spec.monitoring.enablePodMonitor` | `false` | Prometheus wiring is Phase 6. |

**Validation after ArgoCD finishes syncing**:

```bash
kubectl -n recess-data get cluster
```

```text
NAME   AGE    INSTANCES   READY   STATUS                     PRIMARY
pg     ~2m    1           1       Cluster in healthy state   pg-1
```

```bash
kubectl -n recess-data get pods,svc,pvc,secret
```

Expected resources (generated by CNPG, not by git):

- **Pod** `pg-1` Running.
- **Services** `pg-rw` (primary), `pg-ro` (replicas only — empty on a 1-instance cluster), `pg-r` (primary + replicas round-robin). All `ClusterIP:5432`.
- **PVC** `pg-1` Bound to a 20Gi `local-path` PV.
- **Secrets**:
  - `pg-app` — `kubernetes.io/basic-auth`, 11 keys: `username`, `password`, `host`, `port`, `dbname`, `uri`, `jdbc-uri`, etc. Services reference this via `valueFrom.secretKeyRef`.
  - `pg-ca`, `pg-replication`, `pg-server` — TLS material. CNPG enforces TLS for replication and client streaming by default.
  - No `pg-superuser` — CNPG 1.29 default is `enableSuperuserAccess: false`. The in-pod `postgres` OS user still peer-auths locally as the Postgres superuser role, which is what we use for admin queries from `kubectl exec`.

### 5.6.c — Verify schemas

```bash
kubectl -n recess-data exec pg-1 -c postgres -- psql -d recess -c "\dn"
```

Expected output:

```text
       List of schemas
  Name   |       Owner
---------+-------------------
 public  | pg_database_owner
 ratings | recess
 users   | recess
(3 rows)
```

Note: `-U recess` over the local socket fails with `Peer authentication failed` because the OS user inside the pod is `postgres`, not `recess`. Two ways to work around it:

- **Superuser via peer** (shown above): `psql -d recess` — OS user `postgres` peer-auths as PG role `postgres` (built-in superuser), then connects to the `recess` database. Good for admin / read queries.
- **App user via TCP + password**:

  ```bash
  PG_PASS=$(kubectl -n recess-data get secret pg-app -o jsonpath='{.data.password}' | base64 -d)
  kubectl -n recess-data exec pg-1 -c postgres -- env PGPASSWORD="$PG_PASS" psql -h localhost -U recess -d recess -c "\dn"
  ```

### 5.6.d — Connecting from application workloads

Phase 5.4 services will reference the `pg-app` Secret directly. DNS for the primary:

```text
pg-rw.recess-data.svc.cluster.local:5432
```

For HA reads (none on a 1-instance cluster, but the service exists): `pg-ro.recess-data.svc.cluster.local:5432`.

### 5.6.e — Applying SQL migrations

CNPG's `initdb.postInitApplicationSQL` only creates the `users` and `ratings` schemas (see 5.6.b). The rest of the DDL — tables, indexes, enums, schema `admin` — lives in `shared/db/migrations/`. Those files run automatically on a docker-compose bring-up (they're mounted into Postgres' `docker-entrypoint-initdb.d`), but on CNPG we have to apply them ourselves.

**What to apply and what to skip**:

| File | Apply on CNPG? | Why |
| --- | --- | --- |
| `000_init-databases.sh` | No | Creates the `recess` user and `unleash` DB. CNPG already creates the `recess` user via `initdb.owner`. Unleash isn't deployed yet (Phase 5.5+). |
| `001_initial.sql` … `009_notifications_source_event_id.sql` | **Yes** | Pure SQL, idempotent `CREATE SCHEMA IF NOT EXISTS`, no `DROP`. |
| `998_grant_permissions.sh` | No (for now) | Creates a `claude_mcp_ro` read-only user and sets default privileges. Nice-to-have for local dev tooling, not required for service operation. |
| `999_seed.sql` | No | Only loads test-mode fixtures. |

**Run as the application user over TCP**. The in-pod OS user is `postgres` (superuser), but we want tables owned by the `recess` application role, so we connect with its password explicitly:

```bash
PG_PASS=$(kubectl -n recess-data get secret pg-app -o jsonpath='{.data.password}' | base64 -d)
for f in shared/db/migrations/00[1-9]_*.sql; do
  echo "-- applying $f"
  cat "$f"
done | kubectl -n recess-data exec -i pg-1 -c postgres -- \
    env PGPASSWORD="$PG_PASS" \
    psql -v ON_ERROR_STOP=1 -h localhost -U recess -d recess
unset PG_PASS
```

`-v ON_ERROR_STOP=1` halts at the first error instead of marching on through cascading failures. Expected output: ~30-50 `CREATE TABLE` / `CREATE INDEX` / `CREATE TYPE` lines plus `UPDATE 0` on migration 005 (rebrand, no rows to touch on a fresh DB).

**Verify** the expected tables exist:

```bash
kubectl -n recess-data exec pg-1 -c postgres -- psql -d recess -c "\dt public.*" | head -20
kubectl -n recess-data exec pg-1 -c postgres -- psql -d recess -c "\dn"
```

Expected: schemas `public`, `users`, `ratings`, `admin` (the first three owned by `recess`, `admin` created by migration 004); tables `players`, `allowed_emails`, `oauth_identities`, `rooms`, `sessions`, etc. in `public`.

**Sustainable pattern — TODO**: manual `kubectl exec` is fine for bootstrap but it has known weaknesses:

- Nothing tracks which migrations already ran; a re-run fails because most of these `.sql` files use `CREATE TABLE` (without `IF NOT EXISTS`).
- A fresh cluster depends on someone remembering the command above.
- On a future `010_*.sql` there's no safe way to apply only the new file.
- No concurrent-apply protection, no rollback path.

**Picked solution for the next pass: Option A — `golang-migrate` + ArgoCD-managed Job.**

Plan when we pick this up:

1. Split each existing `00N_*.sql` into `00N_<name>.up.sql` (plus optional `.down.sql`). The SQL bodies stay the same.
2. Build a small OCI image that bundles the `migrate` binary + the `shared/db/migrations/` directory (multi-stage Dockerfile, push to GHCR).
3. New ArgoCD Application `db-migrations` with a Job manifest that runs:
   `migrate -source file:///migrations -database "$DATABASE_URL" up`
   against the CNPG primary (`pg-rw.recess-data.svc.cluster.local`). Job consumes a SealedSecret in `recess-data` for the admin DSN.
4. Annotate the Job with `argocd.argoproj.io/sync-wave: "-10"` so it runs before every service Application in the root App-of-Apps. `golang-migrate` keeps its state in `schema_migrations`, so re-runs are no-ops when there's nothing pending.
5. Rejected alternatives: Atlas Operator (heavier dependency for the current scale), CNPG `postInitApplicationSQLRefs` (init-only, can't retrofit), per-service PreSync hooks (migrations are shared across all 8 services, splitting them doesn't map).

Until that lands, treat the manual `kubectl exec` block above as the source of truth for recreating the DB from scratch, and flag every new `shared/db/migrations/*.sql` at review time so it actually gets applied on the cluster.

### 5.6 — Validation

- [x] CNPG operator pod Running
- [x] CNPG CRDs registered
- [x] `Cluster pg` reports `Cluster in healthy state`, 1/1 ready
- [x] PVC `pg-1` Bound to 20Gi on `local-path`
- [x] `pg-app` Secret present with 11 keys
- [x] Schemas `public`, `users`, `ratings` exist and are owned correctly
- [x] Migrations 001-009 applied; schema `admin` + all application tables present
- [ ] PgBouncer Pooler CR — deferred until we see connection-count pressure (CNPG includes a `Pooler` CR, trivial to add later)
- [ ] Backups via `barmanObjectStore` — deferred (see 5.1.f)
- [ ] Sustainable migration runner (Job / Atlas / PreSync Hook) — deferred, see 5.6.e

---

## 5.7 Redis

Single-instance Redis for session state, pub/sub channels, rate limiting, and Asynq task queues. Bitnami chart, standalone architecture. Managed by ArgoCD (unlike CNPG, which is a direct helm install — Redis is a pure workload, not an operator).

### 5.7.a — Whitelist the bitnami Helm repo in the AppProject

Because `AppProject recess.sourceRepos` only listed this git repo, ArgoCD rejects any Application whose source is a Helm repo. Add `https://charts.bitnami.com/bitnami` to `infra/k8s/argocd-apps/project.yaml` (will grow as new chart sources appear — sealed-secrets, kube-prometheus-stack, Loki, Tempo).

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
helm search repo bitnami/redis --versions | head -3
```

Pinned chart: **`bitnami/redis 25.3.11`** (app version `8.6.2`).

Re-apply the project manifest so ArgoCD picks up the new `sourceRepos` entry:

```bash
kubectl apply -f infra/k8s/argocd-apps/project.yaml
```

### 5.7.b — Application CR

Manifest: `infra/k8s/apps/redis.yaml`. Values are inlined in the Application's `spec.source.helm.values` block because the config is small. When it grows, split into `infra/k8s/redis/values.yaml` + multi-source Application.

Key settings:

| Value | Reason |
| --- | --- |
| `architecture: standalone` | Single Redis pod, no sentinel, no cluster. Homelab only needs one. |
| `auth.enabled: true` | Random password is generated into Secret `redis` (key `redis-password`). Phase 5.9 seals this. |
| `commonConfiguration: maxmemory 200mb + allkeys-lru` | Bitnami's default leaves `maxmemory` unset so LRU eviction never fires even at the resource limit. Setting an explicit value mirrors the docker-compose config. |
| `master.persistence: 5Gi, local-path` | Small PV on the default StorageClass. |
| `master.resources` | req `25m / 64Mi`, lim `200m / 256Mi`. Sized for the session-state workload. |
| `metrics.enabled: false` | Prometheus exporter lives in Phase 6. |

### 5.7.c — Apply and verify

Commit + push the Application; ArgoCD syncs automatically (root App-of-Apps picks up the new file). To skip polling:

```bash
kubectl -n argocd patch app recess-root --type merge -p '{"operation":{"sync":{}}}'
kubectl -n argocd patch app redis --type merge -p '{"operation":{"sync":{}}}'
```

Expected state in `recess-data` within ~2 min:

```bash
kubectl -n recess-data get pods,pvc,secret
```

- Pod `redis-master-0` (StatefulSet) Running.
- PVC `redis-data-redis-master-0` Bound, 5Gi.
- Secret `redis` with key `redis-password`.

**Ping test**:

```bash
kubectl -n recess-data exec redis-master-0 -- sh -c 'redis-cli -a "$(cat $REDIS_PASSWORD_FILE)" ping'
```

Returns `PONG`. The Bitnami image mounts the Secret as a file at `$REDIS_PASSWORD_FILE`; the env var `REDIS_PASSWORD` is *not* set. Using `$REDIS_PASSWORD` as plain env fails with `AUTH failed`.

### 5.7.d — Gotcha: ArgoCD repo-server OOM

First-time rendering of `bitnami/redis` OOMKilled the ArgoCD repo-server at the `512Mi` limit we originally set in `infra/k8s/argocd/values.yaml`, then again at `1Gi`. It succeeded at `2Gi`. Bitnami charts expand heavily at template time (dependencies, macros, included templates) and the repo-server renders every source it sees.

Fix: `repoServer.resources.limits.memory: 2Gi` in `infra/k8s/argocd/values.yaml`. Apply with:

```bash
helm upgrade --install argocd argo/argo-cd --version 9.5.2 --namespace argocd -f infra/k8s/argocd/values.yaml --wait --timeout 10m
```

Detection: the Application's ArgoCD UI shows `ComparisonError: failed to generate manifest ... transport: connection refused` and `kubectl -n argocd get pods -l app.kubernetes.io/name=argocd-repo-server` shows a recent `RESTARTS` count. `lastState.terminated.reason` is `OOMKilled`.

Rule of thumb: whenever a new upstream Helm chart joins the cluster (bitnami/*, prometheus-community/*, grafana/*), watch the first sync for a repo-server restart. If it OOMs, bump the limit. 2Gi covers every chart we plan to use; if that changes, raise again — the Pi 5 has 16Gi.

### 5.7 — Validation

- [x] AppProject `recess` whitelists `https://charts.bitnami.com/bitnami`
- [x] ArgoCD `redis` Application `Synced + Healthy`
- [x] Pod `redis-master-0` Running; PVC `redis-data-redis-master-0` Bound
- [x] Secret `redis` contains `redis-password`
- [x] `redis-cli -a $(cat $REDIS_PASSWORD_FILE) ping` returns `PONG`
- [x] ArgoCD repo-server memory bumped to 2Gi (prevents OOM on large Helm charts)

---

## 5.9 SealedSecrets (partial — Redis first)

This phase covers the end-to-end flow: controller, CLI, one SealedSecret, one application consuming it. The full Secret inventory (Postgres, JWT, GitHub OAuth, CLOUDFLARE_TUNNEL_TOKEN, GHCR pull) gets migrated in follow-up passes as each service needs them.

### 5.9.a — Controller

Installed with Helm directly (same pattern as CNPG — platform component, not ArgoCD-managed):

```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm repo update
helm search repo sealed-secrets/sealed-secrets --versions | head -3
```

Pinned chart: **`sealed-secrets/sealed-secrets 2.18.5`** (app version `0.36.6`).

Values in `infra/k8s/sealed-secrets/values.yaml` only trim the request size.

```bash
helm upgrade --install sealed-secrets sealed-secrets/sealed-secrets --version 2.18.5 --namespace kube-system -f infra/k8s/sealed-secrets/values.yaml --wait --timeout 5m
```

**Validation**:

```bash
kubectl -n kube-system get pods -l app.kubernetes.io/name=sealed-secrets
```

One pod `sealed-secrets-*` Running. A single restart right after install is normal — the controller regenerates the RSA master key on first boot and cycles once.

### 5.9.b — kubeseal CLI on the workstation

Pinned to the matching app version so the wire format and API stay in sync with the controller.

```bash
cd /tmp && curl -sLO https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.36.6/kubeseal-0.36.6-linux-amd64.tar.gz
tar -xzf /tmp/kubeseal-0.36.6-linux-amd64.tar.gz -C /tmp kubeseal && chmod +x /tmp/kubeseal && mv /tmp/kubeseal ~/.local/bin/kubeseal && hash -r
kubeseal --version
```

### 5.9.c — Sealing the Redis password

The Bitnami Redis chart auto-generates a password into Secret `redis` on first install. That is not reproducible (fresh clusters get fresh passwords) and it lives only as cluster state, not in git. We replace it with a SealedSecret.

Generate a random password, save it in a password manager (e.g. 1Password — the private key cannot be recovered from git), pipe the plain Secret through `kubeseal`, commit the sealed output:

```bash
REDIS_PASS=$(openssl rand -base64 32 | tr -d '\n')
echo "Save in 1Password: $REDIS_PASS"
kubectl create secret generic redis-auth --from-literal=redis-password="$REDIS_PASS" --namespace=recess-data --dry-run=client -o yaml | kubeseal --controller-name=sealed-secrets --controller-namespace=kube-system --format yaml > infra/k8s/secrets/redis-auth.yaml
unset REDIS_PASS
```

The resulting file `infra/k8s/secrets/redis-auth.yaml` contains a `SealedSecret` CR with an `encryptedData.redis-password` block. It is asymmetrically encrypted against this cluster's controller key and is safe to commit in a public repo — only this cluster's controller can decrypt it.

**Portability caveat**: the SealedSecret is tied to the current controller's RSA key. If the controller is reinstalled (losing `kube-system/sealed-secrets-key-*` Secrets), previously sealed files cannot be decrypted. Back up those Secrets, or be prepared to re-seal from the 1Password-stored plaintext. A full key-rotation procedure belongs in a later hardening pass.

### 5.9.d — Wiring the Redis chart to the SealedSecret

1. A dedicated Application `secrets` (in `infra/k8s/apps/secrets.yaml`) directory-syncs `infra/k8s/secrets/`. Any new SealedSecret file committed to that directory becomes a `SealedSecret` CR in the cluster, which the controller decrypts into the namespace declared in the CR's `metadata`.
2. The Redis Application's Helm values switch from auto-generation to `auth.existingSecret`:

   ```yaml
   auth:
     enabled: true
     existingSecret: redis-auth
     existingSecretPasswordKey: redis-password
   ```

   The chart now reads the password from our managed Secret instead of generating one.

### 5.9.e — Verify the cascade

```bash
kubectl -n recess-data get sealedsecret
```

```text
NAME         STATUS   SYNCED   AGE
redis-auth            True     ...
```

```bash
kubectl -n recess-data get secret
```

`secret/redis-auth` appears (the controller materialised it from the SealedSecret). `secret/redis` is gone — the chart no longer generates its own since `existingSecret` is set. Redis StatefulSet re-deploys with the new password; pod `redis-master-0` shows a fresh `AGE`.

`redis-cli -a "$(cat $REDIS_PASSWORD_FILE)" ping` returns `PONG` against the sealed password.

### 5.9 — Validation

- [x] sealed-secrets controller pod Running in `kube-system`
- [x] `kubeseal` CLI installed and reports the same app version as the controller (`0.36.6`)
- [x] `infra/k8s/secrets/redis-auth.yaml` committed; contains `kind: SealedSecret` with `encryptedData`
- [x] `Application secrets` Synced + Healthy in ArgoCD
- [x] `SealedSecret/redis-auth` status `SYNCED: True`
- [x] `Secret/redis-auth` materialised with the `redis-password` key
- [x] Redis re-deployed using `existingSecret`; PING succeeds with the sealed password
- [ ] Postgres credentials sealed — deferred (CNPG manages `pg-app` adequately; revisit when cluster recreation becomes a scenario)
- [x] JWT secret sealed — see 5.4.a (`infra/k8s/secrets/jwt.yaml`)
- [ ] GHCR image-pull secret sealed — deferred; GHCR images are public today, flip when we move to private repos
- [ ] CLOUDFLARE_TUNNEL_TOKEN sealed — deferred to Phase 5.8
- [ ] Key rotation procedure documented — deferred hardening task

---

## 5.4 Canary: auth-service

First ArgoCD-managed workload, deployed after the data layer is up (Order B). Validates the end-to-end GitOps flow — Helm chart in git, per-service values, SealedSecrets, automated sync — before rolling the remaining seven services through the same chart in Phase 5.5.

### 5.4.a — Per-service SealedSecrets

`auth-service` needs five env vars backed by Secrets. Four are sealed fresh in namespace `recess`; the fifth (`JWT_SECRET`) replaces the one we sealed in Phase 5.9 with a cleaner name.

| Secret | Key | SealedSecret file | Source |
| --- | --- | --- | --- |
| `db-auth` | `DATABASE_URL` | `infra/k8s/secrets/db-auth.yaml` | CNPG `pg-app.password` + hard-coded DSN to `pg-rw.recess-data.svc.cluster.local` |
| `redis-url` | `REDIS_URL` | `infra/k8s/secrets/redis-url.yaml` | SealedSecret `redis-auth.redis-password` + hard-coded DSN to `redis-master.recess-data.svc.cluster.local` |
| `github-oauth` | `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET` | `infra/k8s/secrets/github-oauth.yaml` | Real GitHub OAuth app creds (saved in 1Password) |
| `jwt` | `JWT_SECRET` | `infra/k8s/secrets/jwt.yaml` | Freshly generated HS256 key (`openssl rand -base64 48`); saved in 1Password. Replaced the earlier `jw`/`secret` sealing from 5.9.b — name and key now match the env var convention. |

All four SealedSecrets are picked up by the `secrets` Application (see 5.9.d), which directory-syncs `infra/k8s/secrets/`. The moment we commit a new file there, ArgoCD + the sealed-secrets controller materialise the plain `Secret` into the target namespace.

**Reproducing the seals** on a fresh cluster: `/tmp/seal-canary-secrets.sh` and `/tmp/seal-jwt.sh` (kept in this workstation's `/tmp/`, not in the repo — they take plaintext inputs and rewrite the sealed files). The cluster-specific controller key lives in `kube-system/sealed-secrets-key-*`; if that key is lost, the only recovery path is re-sealing from the 1Password-stored plaintext.

**Why cross-namespace DSNs and not Secret replication** (Reflector / ESO): both are viable but add a controller. For a single canary we preferred sealing dedicated DSN Secrets in the app namespace — each service has its own `DATABASE_URL` / `REDIS_URL` sealed into `recess`, with the DSN pinning the full DNS of the data-tier Service. If we add more namespaces later, a replicator becomes worth it; for now it would be extra moving parts for no benefit.

**Known follow-up**: the `pg-app` password is still CNPG-managed (auto-generated on Cluster creation, never rotated). A rotation rotates only inside `recess-data` and silently invalidates the sealed `DATABASE_URL`. When rotation becomes a requirement we'll need either a pre-sealed bootstrap secret on the Cluster CR (`spec.bootstrap.initdb.secret.name`) or a controller that refreshes the sealed DSN whenever the source Secret changes.

### 5.4.b — The `go-service` Helm chart

Reusable chart at `infra/k8s/charts/go-service/`, consumed by one ArgoCD Application per service. All eight Go services share this chart; per-service differences live in `values/<service>.yaml`.

**Layout**:

```text
infra/k8s/charts/go-service/
├── Chart.yaml
├── values.yaml            # defaults — all feature gates off
├── templates/
│   ├── _helpers.tpl       # name / label / SA helpers
│   ├── deployment.yaml    # HTTP always, gRPC port conditional on service.grpcPort > 0
│   ├── service.yaml       # same port-gating as deployment
│   ├── serviceaccount.yaml
│   ├── ingressroute.yaml  # gated on ingress.enabled (Traefik CRD)
│   ├── servicemonitor.yaml# gated on metrics.enabled (Prometheus CRD)
│   └── hpa.yaml           # gated on autoscaling.enabled
└── values/
    └── auth-service.yaml  # per-service overrides; one file per service
```

**Values contract** — two env-var maps plus the usual image / resources / probes:

```yaml
env:                          # plain literal env vars
  ENV: production
  OTEL_SERVICE_NAME: auth-service

envFromSecret:                # env vars bound to Secrets in the same namespace
  DATABASE_URL:
    secretName: db-auth
    key: DATABASE_URL
```

The deployment template renders each entry of `env` as `name/value` and each entry of `envFromSecret` as `name/valueFrom.secretKeyRef`. `ADDR` is auto-injected from `service.httpPort` so the Go binary binds to the same port the Service expects.

**What is gated off by default and why**:

| Gate | Default | Flip on when |
| --- | --- | --- |
| `ingress.enabled` | `false` | Phase 5.8 — Cloudflare Tunnel lands and we start routing `*.recess.io` via Traefik IngressRoutes. |
| `metrics.enabled` | `false` | Phase 6 — `kube-prometheus-stack` is installed and the services expose `/metrics`. |
| `autoscaling.enabled` | `false` | After Phase 6 — HPA needs metrics to be useful. |

**Rendering locally** to preview a change before pushing:

```bash
helm template auth-service infra/k8s/charts/go-service \
  -f infra/k8s/charts/go-service/values/auth-service.yaml \
  --namespace recess
```

### 5.4.c — auth-service Application

Single-source ArgoCD Application at `infra/k8s/apps/auth-service.yaml`:

- `repoURL` + `path` point at the chart directory in this repo; `helm.valueFiles` uses a path relative to the chart (`values/auth-service.yaml`). A multi-source Application is not needed because chart and values live together.
- `spec.project: recess` (enforced by the AppProject).
- `automated.selfHeal: true` + `prune: true` so drift / deleted resources get reconciled.
- Destination `namespace: recess`, `CreateNamespace=false` (the namespace is owned by `namespaces.yaml`, not by this Application — prevents two Applications fighting over it).
- `ServerSideApply=true` avoids the client-side apply warnings on large CRDs.

No finalizer gymnastics beyond the repo-wide `resources-finalizer.argocd.argoproj.io` — deleting the Application cleans up its rendered resources.

### 5.4.d — Sync, rollout, and smoke test

The root App-of-Apps reconciles every ~30 s. When the commit lands, the new Application CR shows up under `recess-root → auth-service` and should walk through:

```text
Sync Status: OutOfSync  →  Progressing  →  Synced
Health:      Missing    →  Progressing  →  Healthy
```

If it stalls in `Progressing` for more than a few minutes, the first thing to check is that the four SealedSecrets in `infra/k8s/secrets/` were materialised into `recess` (the `secrets` Application syncs them, and the pod's env binding needs them to exist before it can start).

**Smoke test** (run once the Application reports Healthy):

```bash
# 1. Pod Running and the image pulled from GHCR
kubectl -n recess get pods -l app.kubernetes.io/name=auth-service

# 2. All five env vars bound (no `envFrom` surprises)
kubectl -n recess describe deploy auth-service | grep -A1 "Environment"

# 3. Logs show DB + Redis connected and the HTTP server listening
kubectl -n recess logs -l app.kubernetes.io/name=auth-service --tail=50

# 4. Health check via port-forward
kubectl -n recess port-forward svc/auth-service 8081:8081 &
curl -s http://localhost:8081/healthz         # expect: ok
kill %1
```

**Expected log lines** (abbreviated):

```text
redis: connected
auth-service listening addr=:8081
unleash: error (Get "http://unleash:4242/api/client/features": dial tcp: lookup unleash on ...: no such host)
```

The Unleash error is expected and harmless in this phase — `featureflags.Init()` fails open (see `shared/featureflags/client.go`), and `IsEnabled` returns the caller-supplied default on a nil client. Unleash lands in Phase 5.5 alongside the other services that use feature flags.

### 5.4 — Validation

Authored: checkboxes flip as the smoke test in 5.4.d passes.

- [x] Four new SealedSecrets in `infra/k8s/secrets/` (db-auth, redis-url, github-oauth, jwt)
- [x] Reusable `go-service` chart renders cleanly under `helm template` with per-service values
- [x] `auth-service` Application CR committed under `infra/k8s/apps/`
- [ ] `secrets` Application syncs the four new SealedSecrets; plain Secrets visible in `recess`
- [ ] `Application auth-service` is `Synced` + `Healthy` in ArgoCD
- [ ] Pod `auth-service-*` is `Running`, image pulled from GHCR
- [ ] `curl http://auth-service:8081/healthz` returns `ok` from inside the cluster
- [ ] Logs show `redis: connected` and `auth-service listening`
- [ ] GitHub OAuth end-to-end flow — deferred to Phase 5.8 when the service is reachable from the public internet via Cloudflare Tunnel; the full login flow needs the callback URL registered with GitHub.
- [ ] ServiceAccount → IRSA / Workload Identity — N/A on a homelab cluster (no cloud IAM); revisit if we ever move to EKS/GKE.
