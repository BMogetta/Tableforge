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

# Parse results per project.
TABLE=$(echo "$JSON" | jq -r '
  [ .suites[].suites[]? // .suites[] | .specs[]? |
    {
      project: .tests[0].projectName,
      status: (.tests[0].results[0].status // "skipped"),
      ran: ((.tests[0].results | length) > 0)
    }
  ] |
  group_by(.project) |
  map({
    project: .[0].project,
    total: length,
    passed: ([.[] | select(.status == "passed")] | length),
    failed: ([.[] | select(.status == "failed" or .status == "timedOut")] | length),
    ran: ([.[] | select(.ran)] | length)
  }) |
  sort_by(.project) |
  .[] |
  "| \(.project) | \(.passed)/\(.total) |" +
  if .ran == 0 then " ![skip](https://img.shields.io/badge/-skipped-lightgrey) |"
  elif .failed == 0 and .passed == .total then " ![pass](https://img.shields.io/badge/-all_pass-brightgreen) |"
  elif .passed == 0 then " ![fail](https://img.shields.io/badge/-all_fail-red) |"
  else " ![partial](https://img.shields.io/badge/-\(.passed)_of_\(.total)-yellow) |"
  end
')

# Also compute totals.
TOTALS=$(echo "$JSON" | jq -r '
  [ .suites[].suites[]? // .suites[] | .specs[]? |
    { status: .tests[0].results[0].status }
  ] |
  { total: length,
    passed: ([.[] | select(.status == "passed")] | length),
    failed: ([.[] | select(.status == "failed")] | length),
    skipped: ([.[] | select(.status == "skipped" or .status == null)] | length)
  } |
  "| **Total** | **\(.passed)/\(.total)** | \(
    if .failed == 0 then "![pass](https://img.shields.io/badge/-all_pass-brightgreen)"
    else "![progress](https://img.shields.io/badge/-\(.passed)_of_\(.total)-yellow)"
    end
  ) |"
')

# Build the section content.
SECTION="## E2E Tests

| Project | Passed | Status |
|---------|--------|--------|
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
