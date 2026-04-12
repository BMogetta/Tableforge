#!/usr/bin/env bash
# update-e2e-readme.sh — run Playwright e2e tests and update the README table.
#
# Requires: make up-test && make seed-test first.
# Usage: make test-e2e-readme (or run directly after make test)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
README="$REPO_ROOT/README.md"

if [ ! -f "$REPO_ROOT/frontend/tests/e2e/.players.json" ]; then
  echo "Error: run 'make seed-test' first"
  exit 1
fi

echo "Running Playwright tests with JSON reporter..."
cd "$REPO_ROOT/frontend"

# Run tests and capture JSON output. Allow non-zero exit (test failures).
JSON=$(npx playwright test --reporter=json 2>/dev/null || true)

# Collect every spec via recursive descent so specs nested in describe()
# blocks AND top-level specs (e.g. auth.setup.ts) are both counted. Each
# spec is tagged with the spec file (basename, trailing "/suite" stripped).
SPECS=$(echo "$JSON" | jq '
  [ .. | objects | .specs? // empty | .[] |
    {
      file: (.file // .tests[0].location.file | tostring | split("/") | last),
      status: (.tests[0].results[0].status // "skipped"),
      title: .title
    }
  ]
')

# Per-file rows, sorted alphabetically.
TABLE=$(echo "$SPECS" | jq -r '
  group_by(.file) |
  map({
    file: .[0].file,
    total: length,
    passed: ([.[] | select(.status == "passed")] | length),
    failed: ([.[] | select(.status == "failed" or .status == "timedOut")] | length)
  }) |
  sort_by(.file) |
  .[] |
  "| \(.file) | \(.passed)/\(.total) |" +
  if .failed == 0 and .passed == .total then " ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |"
  elif .passed == 0 then " ![fail](https://img.shields.io/badge/-all_fail-red) |"
  else " ![partial](https://img.shields.io/badge/-\(.passed)_of_\(.total)-yellow) |"
  end
')

# Totals row.
TOTALS=$(echo "$SPECS" | jq -r '
  { total: length,
    passed: ([.[] | select(.status == "passed")] | length),
    failed: ([.[] | select(.status == "failed" or .status == "timedOut")] | length)
  } |
  "| **Total** | **\(.passed)/\(.total)** | \(
    if .failed == 0 then "![pass](https://img.shields.io/badge/-all_pass-brightgreen)"
    else "![progress](https://img.shields.io/badge/-\(.passed)_of_\(.total)-yellow)"
    end
  ) |"
')

# Failed specs — emit to stdout (not the README) so the user can retry them.
FAILED=$(echo "$SPECS" | jq -r '
  [.[] | select(.status == "failed" or .status == "timedOut")] |
  if length == 0 then empty
  else
    "Failed specs (rerun with: npx playwright test --grep \"<title>\"):",
    (.[] | "  - \(.file): \(.title)")
  end
')

# Build the section content.
SECTION="## E2E Tests

| Spec | Passed | Status |
|------|--------|--------|
${TABLE}
${TOTALS}

_Last updated: $(date +%Y-%m-%d)_"

# Inject into README between markers.
if grep -q '<!-- e2e:start -->' "$README"; then
  # Replace existing section.
  awk '
    /<!-- e2e:start -->/ { print; found=1; next }
    /<!-- e2e:end -->/   { found=0 }
    found { next }
    { print }
  ' "$README" > "$README.tmp"

  # Insert new content after the start marker.
  awk -v content="$SECTION" '
    /<!-- e2e:start -->/ { print; print content; next }
    { print }
  ' "$README.tmp" > "$README"
  rm "$README.tmp"
else
  # Insert before coverage section.
  awk -v content="<!-- e2e:start -->\n${SECTION}\n\n<!-- e2e:end -->\n" '
    /<!-- coverage:start -->/ { print content }
    { print }
  ' "$README" > "$README.tmp"
  mv "$README.tmp" "$README"
fi

echo ""
echo "README.md updated with e2e results."
echo "$SECTION"
if [ -n "$FAILED" ]; then
  echo ""
  echo "$FAILED"
fi
