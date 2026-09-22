#!/usr/bin/env bash
# Generates an ES256 (ECDSA P-256) keypair for JWT signing and prints it in
# the single-line env format used by deploy/.env.prod and the etc yamls
# (literal \n escapes — the services normalize them at load).
#
# Usage:
#   ./scripts/gen-jwt-keys.sh            # user-token pair (JWT_*)
#   ./scripts/gen-jwt-keys.sh ADMIN_JWT  # admin-token pair (ADMIN_JWT_*)
set -euo pipefail

PREFIX="${1:-JWT}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

openssl ecparam -genkey -name prime256v1 -noout -out "$tmp/ec.key" 2>/dev/null
openssl pkcs8 -topk8 -nocrypt -in "$tmp/ec.key" -out "$tmp/private.pem" 2>/dev/null
openssl ec -in "$tmp/ec.key" -pubout -out "$tmp/public.pem" 2>/dev/null

esc() { awk 'NR>1{printf "\\n"}{printf "%s",$0}END{printf "\\n"}' "$1"; }

echo "# ${PREFIX} keypair (ES256 / P-256) — generated $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "# Keep the private key only on token-issuing services (auth / adminway)."
echo "${PREFIX}_PRIVATE_KEY=\"$(esc "$tmp/private.pem")\""
echo "${PREFIX}_PUBLIC_KEY=\"$(esc "$tmp/public.pem")\""
