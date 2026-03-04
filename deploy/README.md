# Deploying the Neobank API

This directory contains the Dockerfile and docker-compose setup. The API can be deployed to **Railway** (or any Docker-friendly platform) by building the `api` target and configuring the following.

## Required services

The API depends on three backing services. Provision them on Railway (or externally) before deploying:

| Service    | Purpose              | Required at startup |
| ---------- | -------------------- | ---------------------- |
| PostgreSQL | State DB (users, KYC, transactions, etc.) | Yes |
| Formance   | Ledger (double-entry accounting)          | Yes |
| Redis      | Rate limiting and cache                   | Yes |

## Environment variables

The app reads config from environment variables with the **`NEO_`** prefix (e.g. `NEO_DATABASE_URL`). Railway injects **`PORT`** at runtime; the API uses it automatically.

### Minimal production set

Set these in your Railway project (or equivalent):

| Variable                       | Description |
| ------------------------------ | ----------- |
| `NEO_DATABASE_URL`             | Postgres connection string (e.g. from Railway Postgres plugin). |
| `NEO_FORMANCE_URL`             | Formance ledger base URL. |
| `NEO_FORMANCE_LEDGER_NAME`     | Ledger name (default: `neobank`). |
| `NEO_FORMANCE_ACCOUNT_PREFIX`  | Account prefix (default: `neo`). |
| `NEO_REDIS_URL`                | Redis URL (e.g. from Railway Redis plugin). |
| `NEO_JWT_SIGNING_KEY`          | **Required in production** — JWT signing secret for user auth. |
| `NEO_APP_INFO_ENVIRONMENT`     | Set to `production` for production behavior. |

### Optional

- `NEO_ADMIN_JWT_SECRET` — if you use the admin API.
- `NEO_CORS_SETTINGS_ALLOWED_ORIGINS` — allowed CORS origins (comma-separated).
- `NEO_RATE_LIMIT_RPS` / `NEO_RATE_LIMIT_BURST` — rate limit tuning.
- EthSwitch, Fayda, Wise, Telegram — set if you use those integrations.

## Migrations

The API **does not** run migrations on startup. Run them before or when deploying.

### Railway (automatic)

On **Railway**, migrations run automatically in the **pre-deploy** phase before each deployment. The root [Dockerfile](../Dockerfile) includes the [golang-migrate](https://github.com/golang-migrate/migrate) CLI and the `migrations/` directory; [railway.toml](../railway.toml) sets the pre-deploy command:

```bash
migrate -path /migrations -database "$NEO_DATABASE_URL" up
```

Ensure **neo-api** has either `NEO_DATABASE_URL` set (e.g. via `${{Postgres.DATABASE_URL}}`) or all of `NEO_DATABASE_HOST`, `NEO_DATABASE_PORT`, `NEO_DATABASE_USERNAME`, `NEO_DATABASE_PASSWORD`, `NEO_DATABASE_DATABASE`—the pre-deploy command uses the URL if present and valid, otherwise builds it from those variables (so migrations work even when the URL reference is not expanded in the pre-deploy phase). If the pre-deploy command fails (e.g. DB unreachable or migration error), Railway aborts the deployment and the previous version keeps running.

### Other platforms / local

1. **Pre-deploy or release command** (if your platform supports it): run the same command as above (path may be `migrations` when run from repo root).
2. **One-off job** — run once against the production DB from a job or your machine (with appropriate access).

## Railway multi-service setup

Deploy the API and all required backing services (Postgres, Redis, Formance) in one Railway project. Add services in this order so you can wire variables from one service to another.

### 1. PostgreSQL (state DB)

- In the project: **New** → **Database** → **PostgreSQL** (or **Add Plugin** → PostgreSQL).
- Railway creates a Postgres service and exposes `DATABASE_URL` (and often `PGHOST`, `PGPORT`, etc.). Note the service name (e.g. `Postgres`).

### 2. Redis

- **New** → **Database** → **Redis** (or **Add Plugin** → Redis).
- Railway exposes `REDIS_URL` (or similar). Note the service name (e.g. `Redis`).

### 3. Formance Postgres (ledger DB)

Formance uses a separate Postgres instance. Add a service that runs the official Postgres image:

- **New** → **Empty Service** (or **Deploy from image**).
- Set **Source** to **Docker image**: `postgres:16-alpine`.
- In **Variables**, add:
  - `POSTGRES_USER` = `formance`
  - `POSTGRES_PASSWORD` = a strong secret
  - `POSTGRES_DB` = `formance`
- No public domain needed; the API and Formance Ledger will use **private networking**.
- Note the service name (e.g. `formance-postgres`). You will need its **private hostname** and port `5432` for the next step.

### 4. Formance Ledger

- **New** → **Empty Service** → **Deploy from image**: `ghcr.io/formancehq/ledger:v2.2.1`.
- **Start command** (if required): `serve --auto-upgrade`.
- In **Variables**, set:
  - `POSTGRES_URI` = `postgres://formance:YOUR_FORMANCE_PASSWORD@${{formance-postgres.RAILWAY_PRIVATE_DOMAIN}}:5432/formance?sslmode=disable`  
    (Replace `YOUR_FORMANCE_PASSWORD` and `formance-postgres` with your Formance Postgres service name if different.)
- Formance listens on port **3068**. If Railway assigns a different `PORT`, set the API’s `NEO_FORMANCE_URL` to use that port (see step 5). Optionally give this service a **public domain** if you need to call the ledger from outside Railway.

### 5. API (this repo)

- **New** → **GitHub Repo** (or **Empty Service** + connect repo). Select this repository.
- **Root Directory**: leave as repo root. The repo’s **railway.toml** sets Dockerfile path and health check.
- In the service **Settings** → **Build**: set **Docker build target** to `api` (the Dockerfile is multi-stage).
- In **Variables**, set (use **Reference** to pull from other services where possible):

| Variable | Value |
| -------- | ----- |
| `NEO_DATABASE_URL` | `${{Postgres.DATABASE_URL}}` (adjust `Postgres` to your state DB service name) |
| `NEO_REDIS_URL` | `${{Redis.REDIS_URL}}` or `${{Redis.URL}}` (adjust `Redis` to your Redis service name) |
| `NEO_FORMANCE_URL` | `http://${{formance-ledger.RAILWAY_PRIVATE_DOMAIN}}:3068` (adjust service name; use the port Formance actually listens on if different) |
| `NEO_FORMANCE_LEDGER_NAME` | `neobank` |
| `NEO_FORMANCE_ACCOUNT_PREFIX` | `neo` |
| `NEO_JWT_SIGNING_KEY` | Your production JWT secret |
| `NEO_APP_INFO_ENVIRONMENT` | `production` |

- **PORT** is set by Railway automatically; the API uses it.
- Add a **public domain** in the API service so you can reach it over HTTPS.

### Variable reference syntax

Railway lets you reference other services with `${{ServiceName.VARIABLE}}`. Use the exact service names you see in the project (e.g. `Postgres`, `Redis`, `formance-postgres`, `formance-ledger`). For private hostnames use `${{ServiceName.RAILWAY_PRIVATE_DOMAIN}}`.

### After deploy: migrations

On Railway, migrations run automatically before each deploy (see **Migrations** above). For other setups, run DB migrations against the **state** Postgres (the one backing the API), not Formance Postgres—e.g. one-off or release command: `migrate -path migrations -database "$NEO_DATABASE_URL" up`.

## Railway build settings (API only)

- **Build**: Repo root is the context. The root **railway.toml** sets Dockerfile path to **`Dockerfile`** (root). The **api** stage includes the API binary, the migrate CLI, and **/migrations** for the pre-deploy command.
- **Pre-deploy**: `railway.toml` sets **preDeployCommand** to run migrations before the API starts; requires `NEO_DATABASE_URL`.
- **Health check**: `railway.toml` can set **healthcheckPath** to **`/healthz`** when enabled.

## Local stack (docker-compose)

For local development:

```bash
docker compose -f deploy/docker-compose.yml up -d
```

This starts Postgres, Formance (and its Postgres), Redis, and the API. The compose file uses unprefixed env vars for convenience; for production (Railway), use the `NEO_*` variables above.
