# Deployment runbook (single VPS)

Target: one Linux VPS (2 vCPU / 4 GB RAM / 40 GB SSD is plenty for the MVP) with Docker. AWS EC2, Lightsail, Hetzner and DigitalOcean all work. PostgreSQL runs in the same compose stack. Managed PostgreSQL is optional.

> **Cost:** choose a provider and check its current pricing before deciding. No figures are given here on purpose. The stack needs one small VM, about 20 GB of off-site backup storage, a domain, and transactional email (free tiers are usually enough at MVP volume).

## 1. Prepare the server

```bash
# as root on a fresh Ubuntu LTS
adduser deploy && usermod -aG sudo deploy
# install Docker Engine + compose plugin (docs.docker.com/engine/install/ubuntu)
usermod -aG docker deploy
ufw allow OpenSSH && ufw allow 80 && ufw allow 443 && ufw enable
# disable SSH password login (PasswordAuthentication no) once your key works
```

Point DNS `A` records for `directory.example.com` (and `www.`) at the server.

## 2. Deploy

```bash
sudo mkdir -p /opt/bharatmfg && sudo chown deploy /opt/bharatmfg
git clone <your-repo> /opt/bharatmfg && cd /opt/bharatmfg
cp infra/deploy/.env.production.example infra/deploy/.env
chmod 600 infra/deploy/.env
# fill in DOMAIN, ACME_EMAIL, POSTGRES_PASSWORD (openssl rand -base64 24),
# DATA_ENCRYPTION_KEY (openssl rand -base64 32), SMTP_*, MAIL_FROM, PRIVACY_EMAIL
docker compose -f infra/deploy/compose.prod.yaml --env-file infra/deploy/.env up -d --build
docker compose -f infra/deploy/compose.prod.yaml --env-file infra/deploy/.env exec -e ADMIN_PASSWORD='…' api \
  /api create-admin --email you@yourdomain --role owner
```

Caddy obtains TLS certificates automatically. Migrations run when the API starts.

**Store `DATA_ENCRYPTION_KEY` in a password manager now.** If it is lost, encrypted contacts and submitter emails cannot be recovered, and backups do not contain it.

## 3. Verify

```bash
curl -fsS https://directory.example.com/healthz
curl -s https://directory.example.com/robots.txt        # "Disallow: /" until indexing is enabled
curl -sI https://directory.example.com/admin | grep -i x-robots-tag
```

Sign in at `/login`, submit a test registration, check that the email arrives, then reject it as spam.

## 4. Backups

```bash
sudo mkdir -p /var/backups/bharatmfg && sudo chown deploy /var/backups/bharatmfg
crontab -e
# 30 2 * * * /opt/bharatmfg/scripts/backup.sh >> /var/log/bharatmfg-backup.log 2>&1
```

For off-site copies, install `rclone`, configure a remote (e.g. Cloudflare R2, Backblaze B2 or S3) and set `BACKUP_REMOTE`. Run `scripts/restore.sh <latest dump>` once now as a drill.

## 5. Updates

```bash
cd /opt/bharatmfg && git pull
docker compose -f infra/deploy/compose.prod.yaml --env-file infra/deploy/.env up -d --build
```

Take a backup first if the release contains migrations (`db/migrations/` changed).

## Managed PostgreSQL (optional)

Set `DATABASE_URL=postgres://…?sslmode=require` in `infra/deploy/.env` and delete the `postgres` service from `compose.prod.yaml`. The provider must support the `pg_trgm`, `unaccent` and `citext` extensions. Backups then follow the provider's snapshots, but keep `scripts/backup.sh`-style logical dumps as well.

## Going public (indexing)

Only after the [launch checklist](../launch-checklist.md) is complete: set `NEXT_PUBLIC_ALLOW_INDEXING=true`, redeploy `web`, and submit `https://<domain>/sitemap.xml` in Google Search Console and Bing Webmaster Tools.
