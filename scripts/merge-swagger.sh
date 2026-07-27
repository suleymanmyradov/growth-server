#!/usr/bin/env bash
#
# merge-swagger.sh — produce a combined OpenAPI 3.0 spec for the Growth gateway.
#
# The gateway has three routes registered manually in
# services/gateway/growth/growthapi.go (POST /files/upload,
# POST /personalization/transcribe, POST /personalization/voice-turn) that use
# multipart/form-data and text/event-stream transports. goctl's .api format does
# not express these, so they are absent from the generated swagger.json.
#
# This script:
#   1. (optional) regenerates the Swagger 2.0 spec via `make swagger-api`
#   2. converts the generated Swagger 2.0 spec to OpenAPI 3.0 (swagger2openapi)
#   3. converts the custom-transports.yaml overlay (OpenAPI 3.0 fragment) to JSON
#   4. deep-merges the overlay's paths, components.schemas, and
#      components.securitySchemes into the converted spec (jq)
#   5. writes swagger-combined.json (OpenAPI 3.0) next to swagger.json
#
# The combined spec is the one the mobile app feeds to openapi-typescript.
#
# Usage:
#   bash scripts/merge-swagger.sh            # use existing swagger.json
#   bash scripts/merge-swagger.sh --regen    # run `make swagger-api` first
#
# Requirements (resolved on first run via npx, no global install needed):
#   - node + npx
#   - swagger2openapi@7.0.8   (Swagger 2.0 → OpenAPI 3.0 conversion)
#   - js-yaml@4               (YAML → JSON for the overlay)
#   - jq                      (deep merge)
#
# From the backend root: bash scripts/merge-swagger.sh
# Make target: make swagger-combined

set -euo pipefail

cd "$(dirname "$0")/.."

SWAGGER_DIR="services/gateway/contract/swagger"
GENERATED="${SWAGGER_DIR}/swagger.json"
OVERLAY="${SWAGGER_DIR}/custom-transports.yaml"
COMBINED="${SWAGGER_DIR}/swagger-combined.json"
OAS3_TMP="$(mktemp -t growth-oas3.XXXXXX.json)"
OVERLAY_TMP="$(mktemp -t growth-overlay.XXXXXX.json)"
trap 'rm -f "$OAS3_TMP" "$OVERLAY_TMP"' EXIT

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

# --- 1. optionally regenerate the Swagger 2.0 spec -------------------------
if [[ "$REGEN" -eq 1 ]]; then
  echo "Regenerating Swagger 2.0 spec (make swagger-api)..."
  make swagger-api
fi

if [[ ! -f "$GENERATED" ]]; then
  echo "ERROR: $GENERATED not found. Run 'make swagger-api' first or pass --regen." >&2
  exit 1
fi
if [[ ! -f "$OVERLAY" ]]; then
  echo "ERROR: overlay $OVERLAY not found." >&2
  exit 1
fi

# --- 2. convert Swagger 2.0 → OpenAPI 3.0 ---------------------------------
echo "Converting Swagger 2.0 → OpenAPI 3.0 (swagger2openapi)..."
npx --yes swagger2openapi@7.0.8 "$GENERATED" -o "$OAS3_TMP" >/dev/null

# --- 3. convert the YAML overlay → JSON -----------------------------------
echo "Converting custom-transports.yaml → JSON (js-yaml)..."
npx --yes js-yaml@4 "$OVERLAY" > "$OVERLAY_TMP"

# --- 4. deep-merge paths + components -------------------------------------
echo "Merging custom transports into OpenAPI 3.0 spec (jq)..."
jq -n --slurpfile base "$OAS3_TMP" --slurpfile overlay "$OVERLAY_TMP" '
  $base[0] as $base
  | $overlay[0] as $overlay
  | $base
  | .paths = (($base.paths // {}) + ($overlay.paths // {}))
  | .components = (($base.components // {}) + ($overlay.components // {}))
  | .components.schemas = ((($base.components // {}).schemas // {}) + (($overlay.components // {}).schemas // {}))
  | .components.securitySchemes = ((($base.components // {}).securitySchemes // {}) + (($overlay.components // {}).securitySchemes // {}))
' > "$COMBINED"

# --- 5. report ------------------------------------------------------------
PATH_COUNT=$(jq '.paths | length' "$COMBINED")
SCHEMA_COUNT=$(jq '.components.schemas | length' "$COMBINED")
echo "Wrote $COMBINED"
echo "  OpenAPI version: $(jq -r '.openapi' "$COMBINED")"
echo "  paths:           $PATH_COUNT"
echo "  schemas:         $SCHEMA_COUNT"
echo "  custom routes:   $(jq -r '.paths | keys[] | select(test("/files/upload|/personalization/transcribe|/personalization/voice-turn"))' "$COMBINED" | tr '\n' ' ')"
