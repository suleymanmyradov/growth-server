#!/usr/bin/env bash
# One-time hardening: create a non-root `deploy` user for CI SSH deploys.
#
# Today GitHub Actions SSHes in as root (DEPLOY_USER=root). This script creates
# a `deploy` user that can run docker (via the docker group — passwordless
# sudo is NOT granted) and write to the repo checkout.
#
# Run ON the VM as root:   bash deploy/scripts/setup-deploy-user.sh [pubkey-file]
#   pubkey-file: path to the public key to authorize for deploy@ (default:
#                copies the keys already authorized for the invoking user —
#                usually /root/.ssh/authorized_keys or the ubuntu user's).
#
# Afterwards (manual steps):
#   1. Test:  ssh deploy@<vm> 'docker ps'  (should list containers)
#   2. GitHub repo secrets (all three repos): DEPLOY_USER=deploy
#   3. Optionally lock down root SSH: set "PermitRootLogin no" in
#      /etc/ssh/sshd_config and `systemctl reload ssh` — only after the
#      deploy user works!
set -euo pipefail

DEPLOY_USER="${DEPLOY_USER_NAME:-deploy}"
REPO_DIR="${REPO_DIR:-/home/ubuntu/growth-server}"

[ "$(id -u)" -eq 0 ] || { echo "error: run as root" >&2; exit 1; }

if ! id "$DEPLOY_USER" >/dev/null 2>&1; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
  echo "created user $DEPLOY_USER"
fi
usermod -aG docker "$DEPLOY_USER"
echo "$DEPLOY_USER added to docker group (docker socket access, no sudo)"

# SSH authorized_keys for the deploy user.
PUBKEY_FILE="${1:-}"
if [ -z "$PUBKEY_FILE" ]; then
  for cand in "$SUDO_HOME/.ssh/authorized_keys" /root/.ssh/authorized_keys /home/ubuntu/.ssh/authorized_keys; do
    [ -n "${SUDO_HOME:-}" ] && [ -f "$cand" ] && PUBKEY_FILE="$cand" && break
    [ -f "$cand" ] && PUBKEY_FILE="$cand" && break
  done
fi
home_dir="/home/$DEPLOY_USER"
mkdir -p "$home_dir/.ssh"
if [ -n "$PUBKEY_FILE" ] && [ -f "$PUBKEY_FILE" ]; then
  cat "$PUBKEY_FILE" >> "$home_dir/.ssh/authorized_keys"
  sort -u "$home_dir/.ssh/authorized_keys" -o "$home_dir/.ssh/authorized_keys"
  echo "installed $(wc -l < "$home_dir/.ssh/authorized_keys") key(s) from $PUBKEY_FILE"
else
  echo "!! no pubkey found — add the CI deploy key to $home_dir/.ssh/authorized_keys" >&2
fi
chmod 700 "$home_dir/.ssh"; chmod 600 "$home_dir/.ssh/authorized_keys"
chown -R "$DEPLOY_USER:$DEPLOY_USER" "$home_dir/.ssh"

# Repo access: the checkout lives under /home/ubuntu and deploy.sh does
# `git fetch && git reset --hard`, so the deploy user needs write access.
# Group-share it via the docker group (deploy is already a member).
if [ -d "$REPO_DIR" ]; then
  chgrp -R docker "$REPO_DIR"
  chmod -R g+rwX "$REPO_DIR"
  find "$REPO_DIR" -type d -exec chmod g+s {} +   # keep group on new dirs
  su - "$DEPLOY_USER" -c "cd $REPO_DIR && git config --global --add safe.directory $REPO_DIR || true"
  echo "$REPO_DIR group-shared with docker group"
fi

echo "==> done. Test: ssh ${DEPLOY_USER}@<vm> 'docker ps'"
echo "    then set GitHub secret DEPLOY_USER=${DEPLOY_USER} in all three repos."
