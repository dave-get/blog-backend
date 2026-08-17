# Go Blog API

A production-oriented REST backend for a multi-author blog. It supports account registration and login, JWT authentication, post publishing, tags, categories in the data model, comments, ownership checks, and admin overrides.

## Run locally

Prerequisites: Go 1.23+, PostgreSQL 17+, Docker (optional).

```sh
cp .env.example .env
docker compose up --build
```

The API listens on `http://localhost:8080`. Compose applies the SQL migration before starting it. To run without Docker, apply `migrations/000001_init.up.sql`, copy `.env.example` to `.env`, update its values, and run:

```sh
go mod tidy
go run ./cmd/migrate up
go run ./cmd/api
```

## Common commands

```sh
make test          # unit tests and coverage
make test-race     # race detector (requires a C compiler)
make lint          # go vet
make migrate-up    # apply migrations through Docker
make migrate-down  # roll back one migration
make migrate-local-up    # migrate DATABASE_URL from .env
make migrate-local-down  # roll back DATABASE_URL from .env
```

## Project layout

```text
cmd/api/                         application composition and process lifecycle
internal/config/                 environment configuration and validation
internal/domain/                 entities, repository ports, shared domain errors
internal/service/                application use cases and authorization rules
internal/repository/postgres/    PostgreSQL repository adapters
internal/platform/database/      connection pool setup
internal/transport/httpapi/      routes, middleware, request/response mapping
migrations/                      versioned database schema
docs/                            architecture and API reference
```

See [docs/architecture.md](docs/architecture.md) and [docs/api.md](docs/api.md) for the design and endpoint contract.

New to Go backends? Open [docs/go-blog-backend-guide.html](docs/go-blog-backend-guide.html) in a browser for a self-contained beginner handbook covering the codebase, setup, request flow, database design, testing, security, deployment, and a build-it-yourself checklist.

## Security notes

- Passwords are hashed with bcrypt and never serialized.
- JWTs use HS256, an issuer check, expiration, and a minimum 32-character secret.
- JSON request bodies are capped at 1 MiB and reject unknown fields.
- All SQL values are parameterized. Post/tag changes run in transactions.
- Post and comment mutations enforce owner or admin authorization.

For a public deployment, terminate TLS at a trusted proxy, store secrets in a secret manager, restrict CORS at the edge, add per-IP and per-account rate limiting, and rotate signing keys through a versioned key strategy.
