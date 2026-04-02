#!/usr/bin/env bash
# update-coverage.sh — collect test coverage from Go services + Vitest
# and update README.md with the results.
#
# Usage: make coverage
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
README="$ROOT/README.md"
DATE=$(date +%Y-%m-%d)
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# ── Go coverage ──────────────────────────────────────────────────────────────

echo "Collecting Go coverage..."
GO_CSV="$TMPDIR/go.csv"
: > "$GO_CSV"
HAS_ERRORS=0

for svc_dir in "$ROOT"/services/*/; do
  [ -f "$svc_dir/go.mod" ] || continue
  svc_name=$(basename "$svc_dir")

  output=$(cd "$svc_dir" && go test ./... -v -cover -count=1 -timeout=600s 2>&1) || true

  # Detect panics or build errors that make the results unreliable.
  panics=$(echo "$output" | grep -c "^panic:" || true)
  if [ "$panics" -gt 0 ]; then
    echo "  $svc_name: PANIC — tests crashed, run 'go test ./services/$svc_name/...' to investigate"
    echo "$svc_name,0,0,0.0" >> "$GO_CSV"
    HAS_ERRORS=1
    continue
  fi

  passed=$(echo "$output" | grep -c "^--- PASS" || true)
  failed=$(echo "$output" | grep -c "^--- FAIL" || true)

  # Extract coverage percentages from lines like: "coverage: 56.8% of statements"
  coverages=$(echo "$output" | grep -oP 'coverage: \K[0-9.]+(?=%)' || true)

  avg="0.0"
  if [ -n "$coverages" ]; then
    avg=$(echo "$coverages" | awk '{s+=$1; n++} END {if(n>0) printf "%.1f", s/n; else print "0.0"}')
  fi

  [ "$failed" -gt 0 ] && HAS_ERRORS=1

  echo "$svc_name,$passed,$failed,$avg" >> "$GO_CSV"
  echo "  $svc_name: $passed passed, ${avg}% coverage"
done

# ── Frontend coverage ────────────────────────────────────────────────────────

echo "Collecting frontend coverage..."
FT_CSV="$TMPDIR/ft.csv"
: > "$FT_CSV"

if [ -f "$ROOT/frontend/package.json" ]; then
  # NO_COLOR disables ANSI codes so grep can parse the output
  output=$(cd "$ROOT/frontend" && NO_COLOR=1 npx vitest run --coverage 2>&1) || true

  vt_total=$(echo "$output" | grep -oP 'Tests\s+\K\d+(?=\s+passed)' || echo "0")
  vt_failed=$(echo "$output" | grep -oP '\d+(?=\s+failed)' || echo "0")

  vt_cov="0.0"
  summary="$ROOT/frontend/coverage/coverage-summary.json"
  if [ -f "$summary" ]; then
    vt_cov=$(python3 -c "
import json
d = json.load(open('$summary'))
print(f\"{d['total']['statements']['pct']:.1f}\")
")
  fi

  echo "frontend,$vt_total,$vt_failed,$vt_cov" >> "$FT_CSV"
  echo "  frontend: $vt_total passed, ${vt_cov}% coverage"
fi

# ── Build README block ───────────────────────────────────────────────────────

badge_color() {
  local pct="$1"
  local int=${pct%%.*}
  if [ "$int" -ge 80 ] 2>/dev/null; then echo "brightgreen"
  elif [ "$int" -ge 60 ] 2>/dev/null; then echo "green"
  elif [ "$int" -ge 40 ] 2>/dev/null; then echo "yellow"
  elif [ "$int" -ge 1 ] 2>/dev/null; then echo "red"
  else echo "lightgrey"
  fi
}

make_row() {
  local name="$1" passed="$2" failed="$3" cov="$4"

  local status="$passed passed"
  [ "$failed" != "0" ] && status="$passed passed, $failed failed"

  local color
  color=$(badge_color "$cov")
  local badge="![${cov}%](https://img.shields.io/badge/coverage-${cov}%25-${color})"

  echo "| $name | $status | $badge |"
}

BLOCK="## Test Coverage

| Service | Tests | Coverage |
|---------|-------|----------|
"

while IFS=, read -r name passed failed cov; do
  BLOCK+="$(make_row "$name" "$passed" "$failed" "$cov")"$'\n'
done < "$GO_CSV"

while IFS=, read -r name passed failed cov; do
  BLOCK+="$(make_row "$name (vitest)" "$passed" "$failed" "$cov")"$'\n'
done < "$FT_CSV"

BLOCK+=$'\n'"_Last updated: ${DATE}_"

# ── Update README ────────────────────────────────────────────────────────────

if [ ! -f "$README" ]; then
  cat > "$README" << 'HEADER'
# Recess

Multiplayer board game platform.

HEADER
  echo "<!-- coverage:start -->" >> "$README"
  echo "<!-- coverage:end -->" >> "$README"
fi

python3 -c "
import sys
readme = open('$README').read()
start = readme.find('<!-- coverage:start -->')
end = readme.find('<!-- coverage:end -->')
if start == -1 or end == -1:
    print('ERROR: coverage markers not found in README.md', file=sys.stderr)
    sys.exit(1)
block = sys.stdin.read()
new = readme[:start] + '<!-- coverage:start -->\n' + block + '\n<!-- coverage:end -->' + readme[end + len('<!-- coverage:end -->'):]
open('$README', 'w').write(new)
" <<< "$BLOCK"

echo ""
echo "README.md updated."

if [ "$HAS_ERRORS" -gt 0 ]; then
  echo ""
  echo "⚠ Some services had panics or test failures — check output above."
  exit 1
fi
