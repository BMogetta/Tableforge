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

### 5.6 — Validation

- [x] CNPG operator pod Running
- [x] CNPG CRDs registered
- [x] `Cluster pg` reports `Cluster in healthy state`, 1/1 ready
- [x] PVC `pg-1` Bound to 20Gi on `local-path`
- [x] `pg-app` Secret present with 11 keys
- [x] Schemas `public`, `users`, `ratings` exist and are owned correctly
- [ ] PgBouncer Pooler CR — deferred until we see connection-count pressure (CNPG includes a `Pooler` CR, trivial to add later)
- [ ] Backups via `barmanObjectStore` — deferred (see 5.1.f)
