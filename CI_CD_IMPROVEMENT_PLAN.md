# CI/CD + Deploy Improvement Plan

Plan de implementación de las mejoras detectadas en la auditoría y de la migración de deploy a **k3s + ArgoCD + Helm** en una Raspberry Pi 5 (arm64).
Cada tarea tiene un paso de **validación** obligatorio antes de marcarla como completa.

**Leyenda:** `[ ]` pendiente · `[~]` en progreso · `[x]` hecho y validado · `[!]` bloqueado

---

## Fase 0 — Preparación

- [x] **0.1** Crear este plan en el root del proyecto
  - Validación: archivo `CI_CD_IMPROVEMENT_PLAN.md` existe y está commiteado
- [x] **0.2** ~~Crear rama `chore/ci-cd-hardening` para toda la serie~~
  - Superseded: el usuario decidió trabajar directo en `main` (repo throwaway a nivel git, sin prod real).
- [x] **0.3** Snapshot de métricas actuales (duración de CI, CD, coste estimado, RAM/CPU en la Pi)
  - Validación: tabla en este doc al final, sección "Baseline"
  - Datos CI/CD medidos el 2026-04-17 con `gh run list` (n=60 de últimos 60 runs en main). Métricas RPi marcadas "pendiente humano" (requieren acceso físico/SSH a la Pi, que todavía no está provisionada).

---

## Fase 1 — Críticos de CI/CD (bloqueantes de riesgo alto)

### 1.1 Gate CD ← CI
CD debe sólo publicar si CI terminó exitoso.

- [x] **1.1.a** Reemplazar trigger `push: [main]` en `cd.yml` por `workflow_run`
  - Cambio: `on: workflow_run: { workflows: ["CI"], types: [completed], branches: [main] }`
- [x] **1.1.b** Añadir `if: github.event.workflow_run.conclusion == 'success'` a todos los jobs de CD
- [ ] **1.1.c** Re-resolver el SHA con `${{ github.event.workflow_run.head_sha }}` en checkout y tags
- [ ] **Validación 1.1**
  - Abrir PR rojo (ej: romper un test de game-server)
  - Merge con admin → verificar que CD **no** corre
  - Revertir → CD corre y publica

### 1.2 Multi-arquitectura en builds (amd64 + arm64)

El target de prod es **arm64** (Raspberry Pi 5). Mantenemos amd64 en el registry para dev local de otros colaboradores y para compatibilidad con CI runners amd64 (caso Playwright).

- [ ] **1.2.a** Cambiar `platforms: linux/arm64` → `linux/amd64,linux/arm64` en los 2 jobs
- [ ] **1.2.b** Verificar overhead de QEMU en build time; si supera +8min por imagen, evaluar volver a arm64-only y que los devs buildeen local
- [ ] **1.2.c** ArgoCD siempre pulleará arm64 (el nodo es arm64) — no requiere cambios en el manifest
- [ ] **Validación 1.2**
  - `docker manifest inspect ghcr.io/<owner>/tableforge-game-server:<sha>` lista ambos arches
  - `docker pull --platform linux/arm64 …` desde la Pi funciona

### 1.3 Pinear `golangci-lint`

- [ ] **1.3.a** Determinar la versión deseada: `golangci-lint version` en local, o `grep` en `.golangci.yml`
- [ ] **1.3.b** Reemplazar `version: latest` en `ci.yml` por `version: vX.Y.Z`
- [ ] **Validación 1.3**
  - Job `go-lint` verde en PR de prueba
  - Cambiar la versión a una inexistente → job falla con error claro

### 1.4 Reducir fanout de `shared/**`

- [ ] **1.4.a** Analizar qué subcarpetas de `shared/` consume cada servicio (grep de imports)
- [ ] **1.4.b** Reescribir filtros:
  - `shared/proto/**` → sólo consumidores de ese proto
  - `shared/events/**` → sólo pub/sub consumidores
  - `shared/domain/rating/**` → game-server + rating-service
  - `shared/domain/matchmaking/**` → match-service
  - `shared/middleware|telemetry|config|db|redis|errors/**` → todos (común)
- [ ] **1.4.c** Extraer filtros a `.github/paths-filters.yml` (DRY entre ci.yml y cd.yml)
- [ ] **Validación 1.4**
  - PR que sólo toque `shared/domain/rating/` ⇒ corre en game-server + rating-service, no en los otros 6
  - PR que toque `shared/middleware/` ⇒ corre en los 8

### 1.5 Status check "paraguas" para branch protection

- [ ] **1.5.a** Añadir job `ci-success` al final de `ci.yml` que `needs:` todos los jobs y usa `if: always()` + check de results
- [ ] **1.5.b** Configurar branch protection: required check = `CI Success`
- [ ] **Validación 1.5**
  - PR con sólo docs (ningún job corre) ⇒ `CI Success` verde
  - PR que rompe lint ⇒ `CI Success` falla

---

## Fase 2 — Seguridad

### 2.1 `govulncheck` para Go
- [ ] **2.1.a** Job `go-vuln` en `ci.yml` con `govulncheck` pineado
- [ ] **Validación 2.1** — CVE conocida temporal en `go.mod` → rojo; revertir → verde

### 2.2 `npm audit` para frontend
- [ ] **2.2.a** Step `npm audit --audit-level=high --omit=dev` en `frontend-ci`
- [ ] **Validación 2.2** — verde en `main`; local coincide

### 2.3 Trivy sobre imágenes en CD
- [ ] **2.3.a** Step `aquasecurity/trivy-action` post-build, pre-push, `severity: CRITICAL,HIGH`, `exit-code: 1`, `ignore-unfixed: true`
- [ ] **2.3.b** Upload SARIF a GitHub Security
- [ ] **Validación 2.3** — imagen base vieja temporal → rojo; revertir → verde

### 2.4 Gitleaks en CI
- [ ] **2.4.a** Job `gitleaks` sobre PR diff con `fetch-depth: 0`
- [ ] **Validación 2.4** — commit de fake key → rojo; revertir → verde

### 2.5 SBOM + provenance en build-push-action
- [ ] **2.5.a** `provenance: mode=max` y `sbom: true` en ambos jobs de build
- [ ] **Validación 2.5** — `cosign download sbom` devuelve SPDX válido; `cosign verify-attestation --type slsaprovenance` OK

### 2.6 CodeQL
- [ ] **2.6.a** `.github/workflows/codeql.yml` con matrix `[go, javascript-typescript]`, `push` main + `schedule` semanal
- [ ] **Validación 2.6** — Security tab del repo muestra análisis

---

## Fase 3 — Calidad del pipeline

### 3.1 Frontend build real en CI
- [ ] **3.1.a** Reemplazar `tsc --noEmit` por `npm run build` (usa `tsconfig.build.json` + vite)
- [ ] **3.1.b** Añadir `npx biome check ./src` además de `biome lint`
- [ ] **3.1.c** Subir bundle como artefacto (retención 3 días)
- [ ] **Validación 3.1** — PR que rompe `tsconfig.build.json` sin romper el dev ⇒ ahora falla

### 3.2 Schema drift check
- [ ] **3.2.a** Job `schema-drift`: `make gen-types` + `git diff --exit-code frontend/src/lib/schema-generated.zod.ts`
- [ ] **3.2.b** Extender a `make gen-proto` + diff sobre `shared/proto/`
- [ ] **Validación 3.2** — editar JSON schema sin regenerar ⇒ rojo

### 3.3 Coverage agregado
- [ ] **3.3.a** Subir coverage consolidado con `codecov/codecov-action@v4` (o Coveralls)
- [ ] **3.3.b** Definir umbral (ej: 60% proyecto-wide, sin regresión por PR)
- [ ] **Validación 3.3** — badge en README se actualiza; PR que baja coverage bajo umbral ⇒ rojo

### 3.4 Validación de compose por perfil
- [ ] **3.4.a** `compose-validate` itera sobre profiles: `""`, `app`, `monitoring`, `production`, `test`
- [ ] **Validación 3.4** — sintaxis inválida en `docker-compose.monitoring.yml` → rojo

---

## Fase 4 — DX / Supply-chain / Releases

### 4.1 Dependabot
- [ ] **4.1.a** `.github/dependabot.yml` con ecosystems: `github-actions`, `gomod` (por servicio, grupo), `npm`, `docker`
- [ ] **Validación 4.1** — al menos 1 PR abierto por Dependabot en el primer ciclo

### 4.2 CODEOWNERS
- [ ] **4.2.a** `.github/CODEOWNERS` con reglas mínimas para `.github/workflows/`, `shared/proto/`, `infra/k8s/`, `services/<svc>/`
- [ ] **Validación 4.2** — PR que toca un workflow solicita review al owner

### 4.3 Pull request template
- [ ] **4.3.a** `.github/pull_request_template.md` con: Summary / Breaking changes / Tests / Rollback
- [ ] **Validación 4.3** — `gh pr create` muestra el template

### 4.4 SHA pinning de actions y base images
- [ ] **4.4.a** Reemplazar `@vN` por SHA en todos los workflows
- [ ] **4.4.b** Pinear base images (`golang:1.26-alpine@sha256:…`, `alpine:3.19@sha256:…`, `nginx:alpine@sha256:…`, `node:20-alpine@sha256:…`)
- [ ] **4.4.c** Dependabot las actualiza (reglas `docker` + `github-actions`)
- [ ] **Validación 4.4** — `grep -E "uses: [^@]+@v[0-9]+$" .github/workflows/*.yml` sin resultados

### 4.5 SemVer releases — independientes por deployable

Estrategia: **versión independiente por componente** con `release-please` multi-package.
Formato de tag: `<component>-vX.Y.Z` (ej: `game-server-v1.2.3`).
Imagen publicada: `ghcr.io/<owner>/tableforge-<component>:1.2.3` + `:1.2` + `:latest`.

**Flujo de release → deploy (full auto):**
```
Merge a main → release-please abre PR "chore(release): game-server 1.2.3"
Merge ese PR → tag auto → CD publica imagen arm64+amd64 firmada
Workflow post-tag bumpea image.tag en infra/k8s/game-server/values.yaml
ArgoCD detecta el commit → sync → rolling update en el cluster
```

- [ ] **4.5.a** `release-please-config.json` en modo **manifest** con entry por deployable (8 servicios + frontend)
  - `release-type: go` para servicios, `node` para frontend
- [ ] **4.5.b** `.release-please-manifest.json` con versiones iniciales `0.1.0-alpha.1` por componente
  - Nota: en este repo los tags generados son throwaway (se descarta el `.git` al migrar al repo limpio). El manifest y los workflows sí viajan como archivos finales; en el primer push del repo limpio empiezan a acumular historial real desde cero.
- [ ] **4.5.c** `.github/workflows/release-please.yml` en `push: main`
- [ ] **4.5.d** `.github/workflows/release.yml` en `push: tags: ['*-v*']`:
  - Parsea tag (`${tag%%-v*}` = componente, `${tag##*-v}` = versión)
  - Buildea, publica con tags `vX.Y.Z`, `vX.Y`, `latest`, cosign + SBOM + provenance
  - **Bumpea** `infra/k8s/<component>/values.yaml` (`image.tag: X.Y.Z`) y commitea a main con `[skip ci]`
- [ ] **4.5.e** Conventional commits con scope por componente (`feat(game-server): …`). Documentar en `CLAUDE.md`
- [ ] **4.5.f** Documentar flujo completo en `RELEASING.md`
- [ ] **Validación 4.5**
  - `feat(game-server): …` → PR de release → merge → tag `game-server-v0.2.0` → imagen publicada → `values.yaml` bumpeado → ArgoCD sincroniza → pod nuevo healthy
  - El frontend no se toca en ese ciclo

---

## Fase 5 — Plataforma de deploy: k3s + ArgoCD + Helm

### 5.1 Bootstrap del cluster k3s en la Raspberry Pi 5

- [ ] **5.1.a** Verificar prerrequisitos en la Pi: Raspberry Pi OS 64-bit, `cgroup_memory=1 cgroup_enable=memory` en `/boot/firmware/cmdline.txt`
- [ ] **5.1.b** Instalar k3s (single-node, server): `curl -sfL https://get.k3s.io | sh -` con flags:
  - `--disable=traefik` → **NO**, conservamos el Traefik built-in de k3s (cumple rol de ingress controller)
  - `--write-kubeconfig-mode=644` para poder usar `kubectl` sin sudo
  - `--node-name=tableforge-pi`
- [ ] **5.1.c** Copiar `/etc/rancher/k3s/k3s.yaml` a la workstation como `~/.kube/config-tableforge`, reemplazar `127.0.0.1` por la IP de la Pi
- [ ] **5.1.d** Instalar Helm en la workstation (no en la Pi)
- [ ] **5.1.e** Decidir storage: `local-path-provisioner` (built-in de k3s) — basta para homelab single-node
- [ ] **5.1.f** Configurar backups del nodo: `etcd`/`k3s` state → rsync nocturno del `/var/lib/rancher/k3s` a otro disco
- [ ] **Validación 5.1**
  - `kubectl get nodes` desde la workstation ⇒ Ready
  - `kubectl get pods -A` ⇒ traefik, coredns, local-path-provisioner, metrics-server Running
  - `kubectl top node` reporta métricas

### 5.2 Namespaces y convenciones

- [ ] **5.2.a** Crear namespaces: `tableforge` (apps), `tableforge-data` (pg, redis), `observability`, `argocd`, `cloudflared`
- [ ] **5.2.b** Documentar convención en `infra/k8s/README.md`
- [ ] **Validación 5.2** — `kubectl get ns` lista todos

### 5.3 Instalar ArgoCD

- [ ] **5.3.a** `helm repo add argo https://argoproj.github.io/argo-helm`
- [ ] **5.3.b** Install con `values.yaml` custom:
  - Server exposed vía IngressRoute de Traefik en `argocd.<dominio>`
  - Auth inicial via admin password; habilitar GitHub OAuth después
  - `configs.params.server.insecure: true` (TLS lo termina cloudflared)
  - `redis-ha: false` (single node)
- [ ] **5.3.c** Añadir repo de manifests (este repo) a ArgoCD vía UI o CR `argoproj.io/v1alpha1/Repository`
- [ ] **5.3.d** Crear el **App-of-Apps** root: `infra/k8s/argocd-apps/root.yaml` (ApplicationSet que materializa todas las otras Applications desde `infra/k8s/apps/*.yaml`)
- [ ] **5.3.e** Resource management: limits CPU/memoria pequeños para ArgoCD (es homelab)
- [ ] **Validación 5.3**
  - ArgoCD UI accesible en `https://argocd.<dominio>`
  - Root ApplicationSet verde, sin apps aún
  - Login con admin OK

### 5.4 Helm chart genérico para los 8 servicios Go

Los 8 servicios son todos stateless, comparten el mismo patrón (HTTP + opcional gRPC, JWT auth, OTel, healthcheck). Un único chart parametrizable.

- [ ] **5.4.a** Crear `infra/k8s/charts/go-service/` con templates:
  - `deployment.yaml` (imagen, env vars, resources, probes, OTel env)
  - `service.yaml` (ClusterIP para HTTP + gRPC opcional)
  - `ingressroute.yaml` (Traefik CR) condicional por flag
  - `configmap.yaml` para config no-secreta
  - `externalsecret.yaml` o referencia a `Secret` para credenciales
  - `hpa.yaml` opcional (sobrekill para homelab, pero listo para activar)
  - `servicemonitor.yaml` (Prometheus scrape)
- [ ] **5.4.b** `values.yaml` con defaults sensatos:
  - `replicaCount: 2` (necesario para zero-downtime)
  - `strategy: RollingUpdate`, `maxSurge: 1`, `maxUnavailable: 0`
  - `readinessProbe` y `livenessProbe` apuntando a `/healthz`
  - `resources.requests: { cpu: 50m, memory: 64Mi }`, `limits: { cpu: 500m, memory: 256Mi }` (ajustar por servicio)
- [ ] **5.4.c** Crear `infra/k8s/apps/<service>/values.yaml` por cada uno de los 8:
  - `game-server`, `auth-service`, `user-service`, `chat-service`, `ws-gateway`, `rating-service`, `notification-service`, `match-service`
  - Overrides: imagen, puertos, resources ajustados, ingress host
- [ ] **5.4.d** `Application` CR en `infra/k8s/apps/<service>.yaml` apuntando al chart + values
- [ ] **Validación 5.4**
  - `helm template infra/k8s/charts/go-service -f infra/k8s/apps/game-server/values.yaml` genera manifests válidos
  - ArgoCD sincroniza los 8 services → pods Ready
  - `curl` a un endpoint de game-server (via Traefik) responde 200

### 5.5 Helm chart del frontend

- [ ] **5.5.a** `infra/k8s/charts/frontend/` — más simple (nginx servir estáticos + IngressRoute)
- [ ] **5.5.b** `values.yaml`: imagen, ingress host principal, replicas: 2
- [ ] **Validación 5.5** — home del frontend accesible vía dominio público

### 5.6 Datos: Postgres como StatefulSet

- [ ] **5.6.a** Usar Helm chart `bitnami/postgresql` o CloudNativePG (CNPG) — recomiendo **CNPG** (operator, backups nativos, más pro)
- [ ] **5.6.b** Instalar operator: `helm install cnpg cnpg/cloudnative-pg -n cnpg-system`
- [ ] **5.6.c** Definir `Cluster` CR:
  - 1 instance (single-node k3s)
  - `postgresql.version: 16`
  - `storage.size: 20Gi` en `local-path`
  - `bootstrap.initdb` con schemas `users`, `ratings`, extensiones que necesites
  - `backup.barmanObjectStore` → S3 / backblaze B2 / drive local (elegir)
- [ ] **5.6.d** Crear `Secret` de creds vía SealedSecret (ver 5.9)
- [ ] **5.6.e** PgBouncer: CNPG incluye soporte built-in (`Pooler` CR) — úsalo en vez del contenedor separado
- [ ] **Validación 5.6**
  - `kubectl cnpg status <cluster>` verde
  - Conexión desde game-server vía ClusterIP `<cluster>-rw.tableforge-data:5432`
  - Migraciones aplicadas correctamente
  - Backup nocturno en el bucket / volumen configurado

### 5.7 Datos: Redis como StatefulSet

- [ ] **5.7.a** Helm chart `bitnami/redis` en modo standalone (no sentinel, no cluster — homelab)
- [ ] **5.7.b** `values.yaml`:
  - `architecture: standalone`
  - `master.persistence.size: 5Gi`
  - `master.configuration` replicando `maxmemory-policy allkeys-lru` actual
  - `auth.enabled: true` + SealedSecret
- [ ] **Validación 5.7** — ping desde game-server vía `<release>-master.tableforge-data:6379` responde PONG

### 5.8 Ingress: Traefik built-in de k3s + cloudflared

- [ ] **5.8.a** Definir `IngressRoute` CRs por servicio (o `Ingress` stock)
- [ ] **5.8.b** Replicar rules de `docker-compose` actual (hosts, middlewares, auth)
- [ ] **5.8.c** Deployment de `cloudflared` en namespace `cloudflared` con el tunnel token como Secret
  - Apunta a `http://traefik.kube-system:80`
- [ ] **5.8.d** Actualizar la config del tunnel en Cloudflare dashboard si el target cambió
- [ ] **Validación 5.8** — todo el tráfico público funciona igual que hoy

### 5.9 Secrets: SealedSecrets

- [ ] **5.9.a** Instalar controller: `helm install sealed-secrets sealed-secrets/sealed-secrets -n kube-system`
- [ ] **5.9.b** Instalar CLI `kubeseal` en la workstation
- [ ] **5.9.c** Convertir todos los secrets (Postgres, Redis, JWT, GitHub OAuth, CLOUDFLARE_TUNNEL_TOKEN, GHCR pull) a SealedSecrets commiteados en `infra/k8s/secrets/`
- [ ] **5.9.d** Image pull secret para GHCR: docker-registry secret sealado, referenciado en cada Deployment
- [ ] **5.9.e** Documentar rotación en `infra/k8s/README.md`
- [ ] **Validación 5.9**
  - `kubectl get secrets` muestra los Secrets descifrados
  - Re-clonar el repo en otra máquina: los SealedSecrets siguen siendo seguros (no descifrables sin el controller)

### 5.10 Migraciones de DB (safe para zero-downtime)

Problema: hoy game-server aplica migraciones al arrancar. Con 2 réplicas + rolling update, race condition y migraciones no-backwards-compatible rompen.

- [ ] **5.10.a** Extraer migraciones a un **Kubernetes Job** separado que corre **pre-deploy**
  - ArgoCD hook: `argocd.argoproj.io/hook: PreSync`
  - Job usa la misma imagen del servicio pero con `--migrate-only` flag
- [ ] **5.10.b** Documentar en `shared/db/migrations/README.md` (política de backwards-compat queda para definir cuando haya prod real — las migraciones actuales se aplanarán antes)
- [ ] **Validación 5.10**
  - Deploy con una migración nueva → Job corre → completa → nuevos pods arrancan con schema actualizado

### 5.11 Health/readiness/liveness en los servicios

- [ ] **5.11.a** Auditar cada servicio: deben exponer `/healthz` (liveness, barato) y `/readyz` (readiness, verifica DB + Redis)
- [ ] **5.11.b** Agregar probes si faltan (ajustar código Go)
- [ ] **5.11.c** Configurar probes en el Helm chart (`initialDelaySeconds`, `periodSeconds`, `failureThreshold` sensatos)
- [ ] **Validación 5.11**
  - `kubectl describe pod` muestra probes configuradas
  - Kill de Redis temporalmente ⇒ pods marcados NotReady, Traefik les quita tráfico, recovery automático

### 5.12 Rolling update zero-downtime verificado

- [ ] **5.12.a** Configurar en el chart: `maxSurge: 1, maxUnavailable: 0, minReadySeconds: 10`
- [ ] **5.12.b** Script de smoke test: loop `curl` contra un endpoint crítico durante el deploy y verifica 0 errores
- [ ] **Validación 5.12**
  - Correr smoke loop (1 req/s)
  - Bumpear tag en values.yaml de un servicio
  - ArgoCD sync → rolling update
  - 0 errores (ni 5xx ni conexiones rechazadas) durante toda la transición

### 5.13 Conectar Fase 4.5 al cluster

El workflow `release.yml` (4.5.d) bumpea `infra/k8s/apps/<service>/values.yaml`, ArgoCD lo detecta.

- [ ] **5.13.a** ArgoCD `Application` configurado con `syncPolicy.automated: { prune: true, selfHeal: true }`
- [ ] **5.13.b** Verificar permisos del bot account de GitHub Actions para commitear bumps
- [ ] **5.13.c** Notificaciones ArgoCD → Discord/Slack/email en deploy success/failure
- [ ] **Validación 5.13**
  - End-to-end: `feat(game-server): …` → release PR → merge → tag → CD → bump → ArgoCD sync → rolling update → notificación recibida
  - Tiempo total "commit a prod" medido en baseline

### 5.14 Rollback workflow

- [ ] **5.14.a** Documentar procedimiento: rollback desde UI ArgoCD (History → Rollback) revierte el cluster pero **no el git**
- [ ] **5.14.b** Procedimiento correcto: `git revert` del commit de bump → merge → ArgoCD sincroniza al tag previo
- [ ] **5.14.c** Agregar alias Makefile: `make rollback SVC=game-server VERSION=1.2.2` que hace el git revert/bump
- [ ] **Validación 5.14** — deploy de una versión mala a propósito → `make rollback` → cluster vuelve a la versión previa en <2min

---

## Fase 6 — Observability en el cluster

Migrar Tempo, Loki, Prometheus, Grafana, OTel Collector y Alertmanager de docker-compose al cluster k3s.

### 6.1 kube-prometheus-stack

- [ ] **6.1.a** `helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -n observability`
- [ ] **6.1.b** `values.yaml`:
  - Grafana con ingress `grafana.<dominio>`, admin creds via SealedSecret
  - Prometheus `retention: 15d`, storage 20Gi
  - Alertmanager con receivers replicando `infra/alertmanager/` actual
- [ ] **6.1.c** Importar dashboards existentes desde `infra/grafana/` como `ConfigMap` con label `grafana_dashboard: "1"`
- [ ] **Validación 6.1** — Grafana accesible, dashboards poblados, alerts funcionando

### 6.2 Loki + Promtail

- [ ] **6.2.a** `helm install loki grafana/loki-stack -n observability` (incluye Promtail)
- [ ] **6.2.b** Storage: filesystem local, 10Gi, retención 7d
- [ ] **6.2.c** Promtail DaemonSet scrapea logs de todos los pods con labels de Kubernetes
- [ ] **Validación 6.2** — logs de los 8 servicios visibles en Grafana Explore via Loki datasource

### 6.3 Tempo

- [ ] **6.3.a** `helm install tempo grafana/tempo -n observability`
- [ ] **6.3.b** OTLP receiver en `:4317` (gRPC) y `:4318` (HTTP)
- [ ] **6.3.c** Storage local, retención 3d
- [ ] **Validación 6.3** — trazas end-to-end visibles, link desde Loki a Tempo funcionando

### 6.4 OTel Collector

- [ ] **6.4.a** Deployment (no DaemonSet) con config desde `infra/collector-config.yaml`
- [ ] **6.4.b** Exporters: Tempo (traces), Prometheus (metrics), Loki (logs)
- [ ] **6.4.c** Servicios Go configurados para mandar a `otel-collector.observability:4317`
- [ ] **Validación 6.4** — traces, metrics, logs llegan a sus destinos finales

### 6.5 ServiceMonitors para los servicios Go

- [ ] **6.5.a** El chart `go-service` ya crea `ServiceMonitor` cuando `.Values.metrics.enabled: true`
- [ ] **6.5.b** Activar en los 8 servicios
- [ ] **Validación 6.5** — Prometheus targets muestra los 8 servicios como UP

### 6.6 Dashboards de aplicación

- [ ] **6.6.a** Dashboard "TableForge Overview" con golden signals por servicio
- [ ] **6.6.b** Dashboard "Bot Analytics" migrado (ver memory reference)
- [ ] **6.6.c** Dashboard "SLO burn rate" con alertas
- [ ] **Validación 6.6** — dashboards render correcto en Grafana, drill-down funciona

---

## Fase 7 — E2E

### 7.1 Trigger por paths críticos
- [ ] **7.1.a** Ampliar `e2e.yml` con `paths:` además de `labeled`:
  - `services/{game-server,match-service,ws-gateway}/**`
  - `frontend/src/features/{game,lobby,room}/**`
  - `shared/proto/{game,lobby}/**`
- [ ] **Validación 7.1** — PR que toca game-server sin label ⇒ E2E corre; PR de docs sin label ⇒ no corre

### 7.2 Traces/videos/screenshots de Playwright
- [ ] **7.2.a** `playwright.config.ts`: `trace: 'retain-on-failure'`, `video: 'retain-on-failure'`, `screenshot: 'only-on-failure'`
- [ ] **7.2.b** `upload-artifact` de `frontend/test-results/` retention 14
- [ ] **Validación 7.2** — forzar fail local → artefactos visibles en Actions

### 7.3 Summary en el PR
- [ ] **7.3.a** `daun/playwright-report-summary@v3` postea tabla passed/failed
- [ ] **Validación 7.3** — comentario aparece en el PR

### 7.4 Secrets vía `secrets.*`
- [ ] **7.4.a** Mover `JWT_SECRET` a `secrets.CI_JWT_SECRET`
- [ ] **Validación 7.4** — workflow sigue pasando

---

## Fase 8 — Menores / Nice-to-have

- [ ] **8.1** Runtime distroless para servicios Go (`gcr.io/distroless/static-debian12:nonroot`); reemplazar wget por probe TCP
- [ ] **8.2** Retention de coverage 7 → 30 días
- [ ] **8.3** Composite action local `.github/actions/detect-changes` para dedup de paths-filter
- [ ] **8.4** README badges: CI, CD, coverage, license, k8s version
- [ ] **8.5** Argo Rollouts para canary/blue-green (después de que la Fase 5 esté estable)
- [ ] **8.6** cert-manager si dejás cloudflared y exponés Traefik directo

---

## Baseline (medido 2026-04-17)

Fuente CI/CD: `gh run list --limit 60` sobre `main`. Últimos 30 días: 91 runs totales. 0 PRs (workflow actual es push directo a `main`). Repo público → minutos GitHub Actions gratuitos ilimitados.

| Métrica | Valor | Nota |
|---|---|---|
| CI duración p50 (push a main) | ~280s (4m40s) | sample n=30 |
| CI duración p95 | ~306s (5m6s) | sample n=30 |
| CI duración máx observada | 315s | |
| CD duración p50 | ~42s | casi todos los runs son skip (detect-changes sin matches) |
| CD duración p95 | ~63s | incluye intentos de build frontend que fallan rápido |
| CD "main → publicado" exitoso | N/A | últimos 30 runs CD son failure o skip; no hay imágenes publicadas recientes |
| Deploy duración (tag → pod healthy) | N/A | Fase 4.5/5 todavía no implementadas |
| Runs/mes GitHub Actions | ~90 | últimos 30 días |
| # PRs/semana | 0 | workflow actual es push directo a main |
| RPi RAM idle (compose actual) | _pendiente humano_ | medir con `free -h` en la Pi |
| RPi RAM con k3s + ArgoCD | _pendiente humano_ | medir post Fase 5.3 |
| RPi CPU idle | _pendiente humano_ | medir con `top`/`vmstat` en la Pi |

## Notas / decisiones

- **2026-04-17** — Estrategia de versionado: **independiente por deployable** (Opción B).
  Razones: microservicios ya aislados por `paths-filter`; contratos versionados en proto; rollback granular necesario; frontend y backend evolucionan a ritmos distintos.
- **2026-04-17** — Plataforma de deploy: **k3s + ArgoCD + Helm** en la Raspberry Pi 5.
  Razones: proyecto de práctica DevOps, UI de ArgoCD para rollback/selección de versión, zero-downtime real, Kubernetes transferible a prod real. Alternativas descartadas: Docker Swarm (moribundo), Watchtower (sólo latest), compose vanilla (sin rollout manager).
- **2026-04-17** — Postgres + Redis **al cluster** (CNPG para Postgres).
- **2026-04-17** — Observability **al cluster** en Fase 6 (después de apps stateless).
- **2026-04-17** — Manifests en `/infra/k8s/` del mismo repo (no config-repo separado por ahora).
- **2026-04-17** — cloudflared se mantiene como ingress público (cero cambios en DNS).
- **2026-04-17** — Full auto: cada tag semver deploya sin approval manual.
- **2026-04-17** — Este repo es throwaway a nivel git: al terminar este plan + el de backend, se borra `.git` y se hace `git init` nuevo con commit inicial en `0.1.0-alpha.1`. Todos los archivos son finales; los commits intermedios no importan. No hay prod ni data real todavía, así que migraciones, backups y políticas de compatibilidad se validan en cluster vacío.
