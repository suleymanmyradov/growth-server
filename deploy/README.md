# Deploying evolella.com backend — AWS EC2 (free plan)

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
