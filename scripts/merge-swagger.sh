#!/usr/bin/env bash
#
# merge-swagger.sh — produce a combined OpenAPI 3.0 spec for the Growth platform.
#
# The platform has two API services behind a single ingress origin:
#   - gateway    (services/gateway/contract) — habits, goals, auth, billing, etc.
#   - ai-gateway (services/ai-gateway/contract) — coaching, weekly reviews, conversations
#
# Both have routes registered manually (multipart/SSE transports) that goctl's
# .api format cannot express. These are documented in
# services/gateway/contract/swagger/custom-transports.yaml, which is a
# hand-maintained OpenAPI 3.0 fragment. Comments in that file mark which
# service serves each path.
#
# This script:
#   1. (optional) regenerates both Swagger 2.0 specs via make swagger-api swagger-ai-api
#   2. converts both Swagger 2.0 specs to OpenAPI 3.0 (swagger2openapi)
#   3. converts the custom-transports.yaml overlay (OpenAPI 3.0 fragment) to JSON
#   4. deep-merges all three: gateway OAS3 + ai-gateway OAS3 + overlay
#      (paths, components.schemas, components.securitySchemes)
#   5. fails on duplicate paths across sources (a path defined in two sources
#      means a route was not cleanly split)
#   6. writes swagger-combined.json (OpenAPI 3.0) in the gateway swagger dir
#
# The combined spec is the one the mobile app feeds to openapi-typescript.
# Clients see one origin via ingress path-prefix routing.
#
# Usage:
#   bash scripts/merge-swagger.sh            # use existing swagger.json files
#   bash scripts/merge-swagger.sh --regen    # run `make swagger-api swagger-ai-api` first
#
# Requirements (resolved on first run via npx, no global install needed):
#   - node + npx
#   - swagger2openapi@7.0.8   (Swagger 2.0 → OpenAPI 3.0 conversion)
#   - js-yaml@4               (YAML → JSON for the overlay)
#   - jq                      (deep merge + duplicate detection)
#
# From the backend root: bash scripts/merge-swagger.sh
# Make target: make swagger-combined

set -euo pipefail

cd "$(dirname "$0")/.."

GW_SWAGGER_DIR="services/gateway/contract/swagger"
AI_SWAGGER_DIR="services/ai-gateway/contract/swagger"
GW_GENERATED="${GW_SWAGGER_DIR}/swagger.json"
AI_GENERATED="${AI_SWAGGER_DIR}/swagger.json"
OVERLAY="${GW_SWAGGER_DIR}/custom-transports.yaml"
COMBINED="${GW_SWAGGER_DIR}/swagger-combined.json"

GW_OAS3_TMP="$(mktemp -t growth-gw-oas3.XXXXXX.json)"
AI_OAS3_TMP="$(mktemp -t growth-ai-oas3.XXXXXX.json)"
OVERLAY_TMP="$(mktemp -t growth-overlay.XXXXXX.json)"
MERGED_TMP="$(mktemp -t growth-merged.XXXXXX.json)"
trap 'rm -f "$GW_OAS3_TMP" "$AI_OAS3_TMP" "$OVERLAY_TMP" "$MERGED_TMP"' EXIT

REGEN=0
if [[ "${1:-}" == "--regen" ]]; then
  REGEN=1
fi

# --- prerequisites ---------------------------------------------------------
for cmd in node npx jq; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERROR: '$cmd' is required but not on PATH." >&2
    exit 1
  fi
done

# --- 1. optionally regenerate the Swagger 2.0 specs ------------------------
if [[ "$REGEN" -eq 1 ]]; then
  echo "Regenerating Swagger 2.0 specs (make swagger-api swagger-ai-api)..."
  make swagger-api swagger-ai-api
fi

if [[ ! -f "$GW_GENERATED" ]]; then
  echo "ERROR: $GW_GENERATED not found. Run 'make swagger-api' first or pass --regen." >&2
  exit 1
fi
if [[ ! -f "$AI_GENERATED" ]]; then
  echo "ERROR: $AI_GENERATED not found. Run 'make swagger-ai-api' first or pass --regen." >&2
  exit 1
fi
if [[ ! -f "$OVERLAY" ]]; then
  echo "ERROR: overlay $OVERLAY not found." >&2
  exit 1
fi

# --- 2. convert both Swagger 2.0 specs → OpenAPI 3.0 -----------------------
echo "Converting gateway Swagger 2.0 → OpenAPI 3.0 (swagger2openapi)..."
npx --yes swagger2openapi@7.0.8 "$GW_GENERATED" -o "$GW_OAS3_TMP" >/dev/null

echo "Converting ai-gateway Swagger 2.0 → OpenAPI 3.0 (swagger2openapi)..."
npx --yes swagger2openapi@7.0.8 "$AI_GENERATED" -o "$AI_OAS3_TMP" >/dev/null

# --- 3. convert the YAML overlay → JSON -----------------------------------
echo "Converting custom-transports.yaml → JSON (js-yaml)..."
npx --yes js-yaml@4 "$OVERLAY" > "$OVERLAY_TMP"

# --- 4. check for duplicate paths across sources ---------------------------
echo "Checking for duplicate paths across gateway + ai-gateway + overlay..."
DUPLICATES=$(jq -n --slurpfile gw "$GW_OAS3_TMP" --slurpfile ai "$AI_OAS3_TMP" --slurpfile ov "$OVERLAY_TMP" '
  ($gw[0].paths // {}) as $gw_paths
  | ($ai[0].paths // {}) as $ai_paths
  | ($ov[0].paths // {}) as $ov_paths
  | ($gw_paths | keys) as $gw_keys
  | ($ai_paths | keys) as $ai_keys
  | ($ov_paths | keys) as $ov_keys
  | ($gw_keys + $ai_keys + $ov_keys | group_by(.) | map(select(length > 1) | .[0]))
')
if [[ "$DUPLICATES" != "[]" ]]; then
  echo "ERROR: duplicate paths found across sources: $DUPLICATES" >&2
  echo "Each path must be defined in exactly one source (gateway, ai-gateway, or overlay)." >&2
  exit 1
fi

# --- 5. deep-merge paths + components -------------------------------------
echo "Merging gateway + ai-gateway + custom transports (jq)..."
jq -n --slurpfile gw "$GW_OAS3_TMP" --slurpfile ai "$AI_OAS3_TMP" --slurpfile ov "$OVERLAY_TMP" '
  $gw[0] as $gw
  | $ai[0] as $ai
  | $ov[0] as $ov
  | $gw
  | .paths = (($gw.paths // {}) + ($ai.paths // {}) + ($ov.paths // {}))
  | .components = (($gw.components // {}) + ($ai.components // {}) + ($ov.components // {}))
  | .components.schemas = ((($gw.components // {}).schemas // {}) + (($ai.components // {}).schemas // {}) + (($ov.components // {}).schemas // {}))
  | .components.requestBodies = ((($gw.components // {}).requestBodies // {}) + (($ai.components // {}).requestBodies // {}) + (($ov.components // {}).requestBodies // {}))
  | .components.responses = ((($gw.components // {}).responses // {}) + (($ai.components // {}).responses // {}) + (($ov.components // {}).responses // {}))
  | .components.parameters = ((($gw.components // {}).parameters // {}) + (($ai.components // {}).parameters // {}) + (($ov.components // {}).parameters // {}))
  | .components.headers = ((($gw.components // {}).headers // {}) + (($ai.components // {}).headers // {}) + (($ov.components // {}).headers // {}))
  | .components.securitySchemes = ((($gw.components // {}).securitySchemes // {}) + (($ai.components // {}).securitySchemes // {}) + (($ov.components // {}).securitySchemes // {}))
' > "$COMBINED"

# --- 6. report ------------------------------------------------------------
PATH_COUNT=$(jq '.paths | length' "$COMBINED")
SCHEMA_COUNT=$(jq '.components.schemas | length' "$COMBINED")
echo "Wrote $COMBINED"
echo "  OpenAPI version: $(jq -r '.openapi' "$COMBINED")"
echo "  paths:           $PATH_COUNT"
echo "  schemas:         $SCHEMA_COUNT"
echo "  custom routes:   $(jq -r '.paths | keys[] | select(test("/files/upload|/personalization/transcribe|/personalization/voice-turn"))' "$COMBINED" | tr '\n' ' ')"
