# Deploying evolella.com backend

Current target: **UpCloud `STARTER-4xCPU-8GB`** (4 vCPU, 8 GB RAM, x86_64) in
`us-east-1` zone `us-nyc1` (New York), public IP `194.113.74.65`.
The AWS EC2 instructions below are kept as an alternative — the stack itself is
provider-neutral Docker Compose; only VM provisioning differs.

## CI/CD (current — how deploys actually happen)

GitHub Actions builds every image and deploys automatically; the VM is a git
clone of this repo (not rsync'd) and only pulls prebuilt images.

- **PRs** → `test` job: `go build`, `go test`, golangci-lint, `make check-ownership`.
- **Push to `main`** → test → build every service image (matrix, GHA-cached) →
  push to GHCR (`ghcr.io/suleymanmyradov/growth-server-<svc>:{sha-<sha>,latest}`)
  → SSH deploy: `deploy/deploy.sh` pulls images, runs migrations as a one-shot
  container, recreates changed services, and health-checks the public domains.
  Fails red (and leaves the old containers running) if any check fails.
- Frontends deploy the same way from their own repos (`self-dev`,
  `growth-admin-front`): image build → pull → recreate only that service.
- The VM at `/home/ubuntu/growth-server` is a **git clone of origin/main**;
  `deploy.sh` does `git fetch && git reset --hard origin/main` each deploy.
  `deploy/.env.prod` and `logs/` are gitignored and survive resets.

Required repo secrets (all three repos): `DEPLOY_SSH_KEY` (dedicated deploy
key, `~/.ssh/growth-deploy-key` locally, pubkey in the VM's
`authorized_keys`), `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_KNOWN_HOSTS`.

**Rollback** — every build is tagged with its commit SHA:

```bash
ssh root@194.113.74.65
cd /home/ubuntu/growth-server
BACKEND_TAG=sha-<old-sha> docker compose -f deploy/docker-compose.prod.yml \
  --env-file deploy/.env.prod --profile admin up -d --no-deps <services>
# frontends: FRONTEND_TAG=sha-<sha> ... up -d --no-deps frontend
#            ADMIN_TAG=sha-<sha>   ... up -d --no-deps admin-frontend
```

**Manual deploy** (if CI is down): `git pull` on the VM, then run the steps of
`deploy/deploy.sh` by hand (pull → `run --rm -T migrate` → `up -d`).

Notes:
- GHCR images are private; the free tier includes 500 MB of package storage.
  The workflows prune old versions (keep 3). If pushes ever fail on quota,
  flip the packages public in the GitHub UI (Packages → Package settings →
  Danger Zone) — the repos are public, so public images leak nothing new.
  Visibility cannot be changed via the REST API.
- Builds run on GitHub runners (GHA-cached, ~2 min per service, all in
  parallel); the VM only pulls. The `build:` blocks in the compose file are
  kept as an emergency fallback for building on the VM.

## UpCloud deployment (current)

Prereqs: `upctl` CLI (`brew tap UpCloudLtd/tap && brew install upcloud-cli`),
logged in with `upctl login`. SSH key: `~/.ssh/upcloud-growth` (ed25519).

```bash
# 1. Create VM + 30 GB Docker data disk (trial accounts cannot edit the
#    firewall via API, but all inbound ports are open by default in trial mode)
upctl server create --title growth-prod --hostname growth-prod \
  --zone us-nyc1 --plan STARTER-4xCPU-8GB \
  --ssh-keys ~/.ssh/upcloud-growth.pub --enable-firewall --wait
upctl storage create --zone us-nyc1 --title growth-docker --size 30
upctl server storage attach <SERVER-UUID> --storage <STORAGE-UUID>

# 2. Format + mount the Docker disk (UpCloud template has no /home/ubuntu)
ssh -i ~/.ssh/upcloud-growth root@<IP>
mkfs.ext4 -F /dev/vdb && mkdir -p /var/lib/docker && mount /dev/vdb /var/lib/docker
echo '/dev/vdb /var/lib/docker ext4 defaults,nofail 0 2' >> /etc/fstab
mkdir -p /home/ubuntu/growth-server /home/ubuntu/self-dev /home/ubuntu/growth-admin-front /home/ubuntu/env

# 3. Sync code + secrets (same layout as AWS; user is root, not ubuntu)
rsync -az --exclude .git --exclude bin --exclude tmp backend/   root@<IP>:/home/ubuntu/growth-server/
rsync -az --exclude .git --exclude node_modules --exclude .next --exclude .next-docs --exclude .turbo frontend/ root@<IP>:/home/ubuntu/self-dev/
rsync -az --exclude .git --exclude node_modules --exclude .next --exclude .next-docs --exclude .turbo admin-frontend/ root@<IP>:/home/ubuntu/growth-admin-front/
scp deploy/.env.prod root@<IP>:/home/ubuntu/env/growth.env   # chmod 600
scp deploy/aws/bootstrap.sh root@<IP>:/home/ubuntu/bootstrap.sh

# 4. Bootstrap (same script as AWS — installs Docker, builds 13 images
#    sequentially, starts the stack, sets MinIO bucket policy)
ssh root@<IP> 'bash /home/ubuntu/bootstrap.sh'

# 5. Migrations (same script; runs on the compose network as root)
scp deploy/../../run-migrations.sh root@<IP>:/home/ubuntu/  # or recreate inline
ssh root@<IP> 'bash /home/ubuntu/run-migrations.sh'

# 6. Admin panel lives behind the `admin` compose profile
ssh root@<IP> 'cd /home/ubuntu/growth-server/deploy && docker compose -f docker-compose.prod.yml --env-file .env.prod --profile admin up -d admin-frontend'
```

DNS: Cloudflare A records `api` / `app` / `admin` → `194.113.74.65`, **DNS only**
(gray cloud). Caddy obtains Let's Encrypt certs on first request; if a cert was
attempted before DNS propagated, `docker restart deploy-caddy-1` forces retry.

UpCloud notes:
- The env file **must** define `APP_DOMAIN` and `ADMIN_DOMAIN` (plus
  `API_DOMAIN`) — Caddy's site blocks are keyed on them; missing values make
  Caddy exit with "server block without any key".
- Trial accounts: resource limits shown by `upctl account show` (6 cores /
  12 GB cap — the 4 vCPU plan fits).
- Cost: $24/mo VM + ~$3.60/mo disk ≈ $28/mo; zero-cost egress.

---

## AWS EC2 deployment (alternative)

Target: **AWS EC2 `m7i-flex.large`** (2 vCPU, 8 GB RAM, x86_64) in `us-east-1`,
Elastic IP `100.63.36.119`. This is the largest instance type the AWS free plan
allows — non-free-tier-eligible types are blocked at the API level on new accounts.

## Architecture

```
Internet → Caddy (:80/:443, auto-TLS via Let's Encrypt)
             ├── /api/v1/conversations, /memory/facts,        → ai-gateway (:8889)
             │   /weekly-reviews/generate*, /personalization/coaching,
             │   /personalization/onboarding-habits,
             │   /personalization/transcribe, /personalization/voice-turn
             ├── /files/*                                     → minio (:9000)
             └── /api/v1/* (everything else)                  → gateway (:8888)

app.evolella.com   → frontend (:3000, Next.js)
admin.evolella.com → admin-frontend (:3001, Next.js) → adminway (:8890)

gateway    → auth, client, search, notifications, filemanager (gRPC)
ai-gateway → auth, client, ai-coach, search (gRPC)
adminway   → client, filemanager, search, auth (gRPC) + Postgres (direct)

Stateful:  PostgreSQL, Redis, Redpanda (Kafka), Meilisearch, MinIO
Consumers: ai-coach-consumer, search-sync, analytics-consumer (optional)
```

## Memory budget (8 GB instance)

| Service          | Limit  |
|------------------|--------|
| PostgreSQL       | 2 GB   |
| Redpanda         | 1.5 GB |
| Meilisearch      | 768 MB |
| Redis            | 384 MB |
| MinIO            | 384 MB |
| auth / client / ai-coach / ai-coach-consumer | 256 MB each |
| gateway / ai-gateway / search / search-sync / filemanager / notifications / adminway / Caddy | 128 MB each |
| frontend         | 512 MB |
| admin-frontend   | 384 MB |

Go services typically idle far below their limits. A 4 GB swapfile is configured
on the VM as headroom.

---

## Deploying from scratch

### 1. Provision

```bash
# Prereqs: AWS CLI v2, credentials in env (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY)
set -a; source ~/.growth-aws-creds; set +a
bash deploy/aws/provision.sh
```

Creates: keypair `growth-deploy` (~/.ssh/growth-deploy.pem), security group
`growth-prod-sg` (22/80/443), the `m7i-flex.large` instance (Ubuntu 24.04, 60 GB
gp3), and an Elastic IP. Writes the IP to `deploy/aws/instance-ip.txt`.

> Free-plan accounts can only launch free-tier-eligible instance types
> (t3.micro/small, t4g.micro/small, c7i-flex.large, m7i-flex.large).

### 2. Sync code + secrets

```bash
rsync -az --exclude .git --exclude node_modules --exclude .next \
  backend/   ubuntu@<IP>:growth-server/
rsync -az --exclude node_modules --exclude .next frontend/ ubuntu@<IP>:self-dev/
rsync -az --exclude node_modules --exclude .next admin-frontend/ ubuntu@<IP>:growth-admin-front/
scp deploy/.env.prod ubuntu@<IP>:env/growth.env   # chmod 600
scp deploy/aws/bootstrap.sh ubuntu@<IP>:bootstrap.sh
```

`.env.prod` is built from `.env.prod.test` with fresh generated secrets
(`secrets.token_hex(32)`) plus the real external keys (Gemini, Resend, Google,
Stripe). It is never committed.

### 3. Bootstrap the VM

```bash
ssh ubuntu@<IP>
sudo bash bootstrap.sh
```

Installs Docker, tunes the kernel (vm.max_map_count for Redpanda/Meilisearch),
adds a 4 GB swapfile, builds all images **sequentially** (parallel builds thrash
2 vCPUs — load 40+), starts the stack, and sets the MinIO bucket to
anonymous-download so public object URLs work.

A full build takes ~40 min on 2 vCPU. Build only what changed afterwards.

### 4. Migrations

```bash
sudo bash /tmp/run-migrations.sh
```
(One-off `migrate/migrate` container on the compose network; reads the password
from `.env.prod`. Recreate the script if missing — see git history or rewrite:
`migrate -path=/migrations -database "postgres://growthmind:$PGPASS@postgres:5432/growthmind?sslmode=disable" up`.)

### 5. DNS (Cloudflare, DNS-only / gray cloud)

| Name    | Value          | Proxy     |
|---------|----------------|-----------|
| `api`   | Elastic IP     | DNS only  |
| `app`   | Elastic IP     | DNS only  |
| `admin` | Elastic IP     | DNS only  |

Caddy issues Let's Encrypt certificates automatically and retries until DNS
propagates. Force a retry with `docker compose restart caddy`.

### 6. Admin panel

```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod --profile admin up -d
```

## Operations

### Logs / status / restart
```bash
cd ~/growth-server/deploy
docker compose -f docker-compose.prod.yml --env-file .env.prod --project-directory . ps
docker compose -f docker-compose.prod.yml --env-file .env.prod --project-directory . logs -f gateway
docker compose -f docker-compose.prod.yml --env-file .env.prod --project-directory . restart gateway
```

### Rebuild after code changes
Rsync changed files, then rebuild **only the changed service** (a full rebuild
takes ~40 min on 2 vCPU):
```bash
docker compose -f docker-compose.prod.yml --env-file .env.prod --project-directory . build frontend
docker compose -f docker-compose.prod.yml --env-file .env.prod --project-directory . up -d frontend
```

### PostgreSQL backup
```bash
docker exec deploy-postgres-1 pg_dump -U growthmind growthmind | gzip > backup_$(date +%Y%m%d).sql.gz
```
(Cron this — see beforeprod.md #18 for the full backup/DR gap.)

### Secrets
All secrets live in `deploy/.env.prod` on the VM (chmod 600, gitignored).
Deploy config templates in `deploy/config/*.yaml` reference them as `${VAR}`;
go-zero expands them at startup via `conf.UseEnv()` (opt-in — every service's
`conf.MustLoad` call must pass it). After editing `.env.prod`:
`docker compose ... up -d` recreates the affected containers.

Still to fill in: `RESEND_API_KEY`, `GOOGLE_CLIENT_SECRET` (done 2026-09-09),
`STRIPE_SECRET_KEY`/`STRIPE_WEBHOOK_SECRET` (see beforeprod.md #2/#3/#6).

### Billing guardrails
- Free plan: $100 credits, expires 2027-03-08 or when credits run out — no
  charges before that; the account just stops.
- CloudWatch alarm `evolella-estimated-spend-80` at $80.
- t4g.large (~$49/mo) becomes available if the account upgrades to a paid plan —
  the Dockerfile builds whatever arch it runs on; switching is
  stop → change type → start.

## Troubleshooting

### SSH times out / banner exchange fails
The 2-vCPU box starves under parallel Docker builds (load 40+). Builds are
sequential in `bootstrap.sh` for this reason. Check `uptime` — load > 10 means
something is thrashing.

### "no space left on device" during builds
Grow the EBS volume (online): `aws ec2 modify-volume --size 60`, then on the VM
`sudo growpart /dev/nvme0n1 1 && sudo resize2fs /dev/nvme0n1p1`.

### Caddy can't get a TLS certificate
- DNS must resolve to the Elastic IP (DNS-only records) — Let's Encrypt fails
  with NXDOMAIN until it does. Caddy retries automatically (up to 6 h between
  attempts); `docker compose restart caddy` forces an immediate retry.
- Check: `docker compose logs caddy | grep certificate`

### Meilisearch "unhealthy" but running
The healthcheck must use `http://127.0.0.1:7700` — `localhost` may resolve to
IPv6 `::1` and Meilisearch binds IPv4 only.

### Service panics on `${VAR}` in config
A service's `conf.MustLoad` is missing the `conf.UseEnv()` option — all 13
services have it; new services must opt in too.

### search-sync fatal-exits on embedders
Hybrid search requires Meilisearch's vector store (experimental in v1.11) plus
an embedder endpoint (Ollama). For launch it runs keyword-only: empty
`Embedder.URL` in `deploy/config/search-sync.yaml` skips embedder setup.
