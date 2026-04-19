#!/usr/bin/env bash
# Re-seal DATABASE_URL and REDIS_URL for the `recess` namespace with the
# current passwords from the data-tier Secrets. Run this after:
#
#   - The CNPG `pg-app` password was rotated (manually or via annotation).
#   - The Redis `redis-auth` SealedSecret was re-rolled.
#   - A fresh cluster bootstrap, once Postgres + Redis are up.
#
# The script reads the source passwords directly from the live cluster,
# URL-encodes them (passwords from `openssl rand -base64` and CNPG can
# contain +, /, = which are reserved URL delimiters), builds the DSNs
# with the cross-namespace DNS, and rewrites the SealedSecret files under
# infra/k8s/secrets/. Commit + push and let ArgoCD sync.
#
# Run from the repo root with KUBECONFIG pointing at the recess cluster.
# Both passwords are PRINTED so you can save them in 1Password before
# they're only accessible inside the cluster again.

set -euo pipefail

: "${KUBECONFIG:?KUBECONFIG must be set (e.g. export KUBECONFIG=~/.kube/config-recess)}"

if [[ ! -d infra/k8s/secrets ]]; then
  echo "run from the repo root (infra/k8s/secrets not found)" >&2
  exit 1
fi

command -v kubeseal >/dev/null || { echo "kubeseal not in PATH" >&2; exit 1; }
command -v python3 >/dev/null || { echo "python3 not in PATH (needed for URL-encoding)" >&2; exit 1; }

urlencode() {
  python3 -c 'import sys,urllib.parse; sys.stdout.write(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

KS_ARGS=(--controller-name=sealed-secrets --controller-namespace=kube-system --format yaml)

PG_PASS=$(kubectl -n recess-data get secret pg-app -o jsonpath='{.data.password}' | base64 -d)
REDIS_PASS=$(kubectl -n recess-data get secret redis-auth -o jsonpath='{.data.redis-password}' | base64 -d)

cat <<EOF

====================================================================
SAVE BOTH PASSWORDS IN 1PASSWORD BEFORE CLOSING THE TERMINAL:

  Postgres user 'recess' (CNPG pg-app):
    $PG_PASS

  Redis password (recess-data/redis-auth):
    $REDIS_PASS

====================================================================

EOF
read -p "Press ENTER once saved in 1Password to continue sealing..."

PG_PASS_ENC=$(urlencode "$PG_PASS")
DB_URL="postgresql://recess:${PG_PASS_ENC}@pg-rw.recess-data.svc.cluster.local:5432/recess?sslmode=require"
kubectl create secret generic db-auth \
  --from-literal=DATABASE_URL="$DB_URL" \
  --namespace=recess --dry-run=client -o yaml \
  | kubeseal "${KS_ARGS[@]}" \
  > infra/k8s/secrets/db-auth.yaml
unset PG_PASS PG_PASS_ENC DB_URL
echo "written: infra/k8s/secrets/db-auth.yaml"

REDIS_PASS_ENC=$(urlencode "$REDIS_PASS")
REDIS_URL="redis://default:${REDIS_PASS_ENC}@redis-master.recess-data.svc.cluster.local:6379/0"
kubectl create secret generic redis-url \
  --from-literal=REDIS_URL="$REDIS_URL" \
  --namespace=recess --dry-run=client -o yaml \
  | kubeseal "${KS_ARGS[@]}" \
  > infra/k8s/secrets/redis-url.yaml
unset REDIS_PASS REDIS_PASS_ENC REDIS_URL
echo "written: infra/k8s/secrets/redis-url.yaml"

cat <<EOF

done. Next:
  git add infra/k8s/secrets/db-auth.yaml infra/k8s/secrets/redis-url.yaml
  git commit -m "chore(k8s): reseal DATABASE_URL and REDIS_URL after password rotation"
  git push
  # ArgoCD's 'secrets' app picks up the change on its next poll (~3 min);
  # force refresh with:
  #   kubectl -n argocd patch app secrets --type merge \\
  #     -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'
  # Then roll the consumers:
  #   kubectl -n recess rollout restart deploy auth-service
EOF
