#!/usr/bin/env bash
set -euo pipefail

# CI check: verifies that services/ai-gateway/contract/types.api is identical
# to services/gateway/contract/types.api. The ai-gateway's types.api is a copy
# (goctl only supports same-directory imports), so the two files MUST stay in
# sync. This check prevents silent drift.
#
# Usage: bash scripts/check-types-api-sync.sh
# Make:  make check-types-api-sync

cd "$(dirname "$0")/.."

GW_TYPES="services/gateway/contract/types.api"
AI_TYPES="services/ai-gateway/contract/types.api"

for f in "$GW_TYPES" "$AI_TYPES"; do
    if [ ! -f "$f" ]; then
        echo "ERROR: $f not found."
        exit 1
    fi
done

if ! diff -q "$GW_TYPES" "$AI_TYPES" >/dev/null 2>&1; then
    echo "ERROR: $GW_TYPES and $AI_TYPES have drifted out of sync."
    echo ""
    echo "The ai-gateway's types.api is a copy of the gateway's types.api"
    echo "(goctl only supports same-directory imports). They must be identical."
    echo ""
    echo "Diff:"
    diff -u "$GW_TYPES" "$AI_TYPES" || true
    echo ""
    echo "Fix: cp $GW_TYPES $AI_TYPES"
    exit 1
fi

echo "types.api sync check passed: gateway and ai-gateway types.api are identical."
