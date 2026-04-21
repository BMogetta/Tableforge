#!/usr/bin/env bash
# Apply the "main protection" ruleset on this repo via the GitHub API.
# Idempotent: if the ruleset already exists (matched by name) it's updated
# rather than duplicated.
#
# Why a ruleset and not classic branch protection:
#   - Layerable, dry-runnable, audit-logged.
#   - First-class support for required signatures + per-check requirements
#     alongside the PR rule.
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

# Rules applied:
#   - deletion / non_fast_forward / required_linear_history
#   - pull_request (required — 0 approvals needed, same as before P1.5)
#   - required_signatures (GPG / web-flow signed commits only)
#   - required_status_checks: CI Success + both CodeQL matrix legs
#
# Why required checks are safe now: recess no longer hosts cluster
# manifests (P1.4). Release.yml pushes image.tag bumps to recess-deploy,
# not here, so there's no auto-bump path that would hit these checks.
# CI Success is an aggregator job that always runs and passes on
# release-please PRs (their changes are in paths-ignore scope for push
# but the aggregator still runs on the PR).

# Note on bypass_actors:
#   Empty list — no bot, no admin bypass. Every change goes through PR.
#   If you ever need an emergency direct push, disable the ruleset with:
#     gh api repos/${REPO}/rulesets/<id> -X PUT -f enforcement=disabled
#   push, then re-enable. One line, no ceremony.

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
        { type: "required_signatures" },
        {
          type: "pull_request",
          parameters: {
            required_approving_review_count: 0,
            dismiss_stale_reviews_on_push: false,
            require_code_owner_review: false,
            require_last_push_approval: false,
            required_review_thread_resolution: false
          }
        },
        {
          type: "required_status_checks",
          parameters: {
            strict_required_status_checks_policy: false,
            required_status_checks: [
              { context: "CI Success" },
              { context: "CodeQL: go" },
              { context: "CodeQL: javascript-typescript" }
            ]
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
