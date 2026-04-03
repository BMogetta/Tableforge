#!/usr/bin/env bash
# Generates TypeScript interfaces from JSON Schema definitions.
# Processes defs/ first (shared types), then endpoint schemas.
# Output: frontend/src/lib/schema-generated.ts
set -euo pipefail

SCHEMA_DIR="shared/schemas"
OUT_FILE="frontend/src/lib/schema-generated.ts"

header='/* eslint-disable */
// @ts-nocheck
/*
 * ---------------------------------------------------------------
 * ## THIS FILE WAS GENERATED FROM JSON SCHEMAS                  ##
 * ## DO NOT MODIFY BY HAND — edit shared/schemas/*.json instead ##
 * ---------------------------------------------------------------
 */
'

echo "$header" > "$OUT_FILE"

# Shared type definitions (defs/)
for schema in "$SCHEMA_DIR"/defs/*.json; do
  [ -f "$schema" ] || continue
  npx --yes json-schema-to-typescript \
    -i "$schema" \
    --cwd "$SCHEMA_DIR" \
    --no-additionalProperties \
    --no-bannerComment \
    2>/dev/null >> "$OUT_FILE"
  echo "" >> "$OUT_FILE"
done

# Endpoint schemas (root level) — $ref resolved via --cwd
for schema in "$SCHEMA_DIR"/*.json; do
  [ -f "$schema" ] || continue
  npx --yes json-schema-to-typescript \
    -i "$schema" \
    --cwd "$SCHEMA_DIR" \
    --no-additionalProperties \
    --no-bannerComment \
    --no-declareExternallyReferenced \
    2>/dev/null >> "$OUT_FILE"
  echo "" >> "$OUT_FILE"
done

echo "Generated $OUT_FILE"
