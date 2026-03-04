# Neo - Ethiopian Neobank API

Production-grade banking API built with Go, PostgreSQL, and Formance Ledger (CE).

## Quick Start

```bash
# Copy environment config
cp .env.example .env

# Start infrastructure (Postgres + Formance)
make up

# Run migrations
make migrate

# Start the API server
make api

# Or start both API + UI dev servers
make dev
```

## Configuration

All configuration is via environment variables. See [.env.example](.env.example) for the full list with documentation.

**Key settings:**

| Variable | Required | Description |
|----------|----------|-------------|
| `ENV` | No | `development` (default), `staging`, or `production` |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `FORMANCE_URL` | No | Formance Ledger CE URL |
| `JWT_SECRET` | Prod only | HMAC signing key (empty = dev mode) |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated allowed origins |

## Development

```bash
make help        # Show all available commands
make test        # Run all tests
make lint        # Run go vet + gofmt check
make vulncheck   # Run Go vulnerability scanner
make check       # Full CI check (lint + test + vulncheck + UI)
```

## Architecture

- **`cmd/`** - Entry points (API, workers, gateway, bot)
- **`internal/domain/`** - Domain models and sentinel errors
- **`internal/usecase/`** - Business logic services
- **`internal/repository/`** - PostgreSQL data access
- **`internal/ledger/`** - Formance Ledger client
- **`internal/transport/`** - HTTP handlers and middleware
- **`pkg/`** - Shared utilities (httputil, money, validation, IBAN)

## API Versioning

All endpoints are prefixed with `/v1/`. When breaking changes are needed, a `/v2/` prefix will be introduced while maintaining `/v1/` for backward compatibility.
