#!/usr/bin/env bash
# Apply the "main protection" ruleset on this repo via the GitHub API.
# Idempotent: if the ruleset already exists (matched by name) it's updated
# rather than duplicated.
#
# Why a ruleset and not classic branch protection:
#   - Per-actor bypass list (github-actions[bot] needs to push the
#     image.tag bump committed by .github/workflows/release.yml — classic
#     rules can only allow "admin" or "no one", not a specific bot).
#   - Layerable, dry-runnable, audit-logged.
#
# Prereqs:
#   - `gh` CLI authenticated (`gh auth login`) with admin access to the repo.
#   - `jq` installed.
#
# Usage:
#   bash scripts/setup-branch-protection.sh              # applies to BMogetta/recess
#   REPO=other-org/other-repo bash scripts/setup-branch-protection.sh

set -euo pipefail

REPO="${REPO:-BMogetta/recess}"
RULESET_NAME="main protection"

# Required status checks intentionally OFF for the first pass.
#
# Why: ci.yml has paths-ignore so CI skips on values.yaml bumps (release
# commits). GitHub's "required status check" rule blocks merges when a
# required check is "missing" — including the skipped-by-paths-ignore
# case. Enabling required checks plus paths-ignore would either block
# the release.yml auto-bump flow or force us to re-structure CI with an
# always-running "CI Success" job that short-circuits on release commits.
#
# For a homelab, "PR required" is already the 90% win. Add required
# status checks later if it ever matters.

# Note on bypass_actors:
#   We don't configure any. Everyone goes through the PR flow — including
#   release-please (its bot opens PRs, we merge them) and release.yml
#   (needs to open its own PR for the image.tag bump rather than pushing
#   direct — see the matching change in .github/workflows/release.yml).
#
# Rationale for NO bypass:
#   - Integration-type bypass expects a numeric GitHub App install ID that
#     can only be fetched with a GitHub App token (user PATs get 403),
#     so there's no clean CLI path to resolve it.
#   - "Everything goes through PR" is architecturally cleaner and gives
#     one consistent audit trail.
#   - If you ever need an emergency direct push, disable the ruleset with:
#       gh api repos/${REPO}/rulesets/<id> -X PUT -f enforcement=disabled
#     push, then re-enable. One line, no ceremony.

build_payload() {
  jq -n \
    --arg name "$RULESET_NAME" \
    '{
      name: $name,
      target: "branch",
      enforcement: "active",
      bypass_actors: [],
      conditions: {
        ref_name: {
          include: ["~DEFAULT_BRANCH"],
          exclude: []
        }
      },
      rules: [
        { type: "deletion" },
        { type: "non_fast_forward" },
        { type: "required_linear_history" },
        {
          type: "pull_request",
          parameters: {
            required_approving_review_count: 0,
            dismiss_stale_reviews_on_push: false,
            require_code_owner_review: false,
            require_last_push_approval: false,
            required_review_thread_resolution: false
          }
        }
      ]
    }'
}

# Find an existing ruleset by name (returns id or empty).
existing_id=$(gh api "repos/${REPO}/rulesets" --jq ".[] | select(.name == \"${RULESET_NAME}\") | .id" | head -n1)

payload=$(build_payload)

if [[ -n "$existing_id" ]]; then
  echo "Updating ruleset #${existing_id} (${RULESET_NAME})..."
  echo "$payload" | gh api "repos/${REPO}/rulesets/${existing_id}" \
    -X PUT \
    -H "Accept: application/vnd.github+json" \
    --input - \
    > /dev/null
else
  echo "Creating new ruleset '${RULESET_NAME}'..."
  echo "$payload" | gh api "repos/${REPO}/rulesets" \
    -X POST \
    -H "Accept: application/vnd.github+json" \
    --input - \
    > /dev/null
fi

echo
echo "Current rulesets on ${REPO}:"
gh api "repos/${REPO}/rulesets" --jq '.[] | "  - \(.name) (id=\(.id), \(.enforcement))"'
echo
echo "Done."
