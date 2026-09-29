# Alertmanager secrets

Drop secret files here on the VM — this directory is gitignored.

| File | Used by | Contents |
|---|---|---|
| `resend_api_key` | `smtp_auth_password_file` in `../alertmanager.yml` | Resend API key, one line |

```bash
mkdir -p deploy/alertmanager/secrets
printf '%s' 're_xxxxxxxxxxxxxxxxxxxxxxxx' \
  > deploy/alertmanager/secrets/resend_api_key
chmod 600 deploy/alertmanager/secrets/resend_api_key
```

Also set the alert addresses in `deploy/.env.prod`:

```bash
ALERT_EMAIL_FROM=alerts@evolella.com   # must be on a Resend-verified domain
ALERT_EMAIL_TO=you@yourmail.com        # where alerts land
```

Then (re)start the monitoring profile:

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/.env.prod \
  --profile monitoring up -d alertmanager
```

Notes:

- File secrets are read lazily (at send time), so a missing/empty
  `resend_api_key` does not crash Alertmanager — sends just fail in logs.
- Resend SMTP: host `smtp.resend.com`, port `587` (STARTTLS), username
  `resend`, password = the API key. If outbound 587 is blocked, port `2587`
  also works.
- To switch channel (Telegram, Discord, PagerDuty…), replace the receiver in
  `../alertmanager.yml` — most receivers support `*_file` secret options that
  read from this directory.
