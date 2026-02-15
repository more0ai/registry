# Capabilities Registry

The **Capabilities Registry** is a Go service that implements the **Capability Registry**: it stores and resolves capability metadata (app, name, version, subject, methods) and exposes a request/reply API over NATS. Clients use it to **resolve** capability names to NATS subjects and to **discover** / **describe** capabilities. The registry can run **standalone** (binary or Docker) or be used from Node/TypeScript apps by importing the **capabilities-client** package (in-process client).

**Licensing:** This project is source-available under the **Business Source License 1.1 (BSL 1.1)**. Production use may require a commercial license from more0ai. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

---

## Table of contents

1. [Quickstart (Docker Compose)](#quickstart-docker-compose)  
2. [Testing](#testing)  
3. [Setup and usage](#1-setup-and-usage)  
   - [Database setup](#database-setup)  
   - [CLI commands (migrate, serve)](#cli-commands)  
   - [Standalone (binary)](#standalone-binary)  
   - [Standalone (Docker / Compose)](#standalone-docker--compose)  
   - [Using from Node/TypeScript](#using-from-nodetypescript-import-packages-in-process-client)  
   - [Environment variables](#environment-variables)  
4. [APIs to interact with the server](#2-apis-to-interact-with-the-server)  
   - [NATS registry API (request/reply)](#nats-registry-api-requestreply)  
   - [HTTP endpoints](#http-endpoints)  
5. [Production and versioning](#production-and-versioning)  
6. [How it works](#3-how-it-works)  
   - [Component overview](#component-overview)  
   - [Component diagrams](#component-diagrams)  
   - [Data flow](#data-flow)  
   - [Bootstrap and subjects](#bootstrap-and-subjects)

---

## Quickstart (Docker Compose)

**Prerequisites:** Docker and Docker Compose.

Run the registry with Postgres and NATS in one command:

```powershell
cp .env.example .env
docker compose up -d
```

Run migrations (required once before the registry is fully usable):

```powershell
docker compose run --rm registry registry migrate up
```

Check health:

```powershell
curl http://localhost:8080/healthz
```

Or on Windows: `Invoke-WebRequest -Uri http://localhost:8080/healthz`. You should get a JSON response with `"status":"healthy"` when the database is reachable.

**Note:** The default credentials in `.env.example` are for local development only. Do not use them in production.

---

## Testing

The registry has **unit**, **integration**, and **e2e** tests. The app uses **platform** resources (Postgres); see `platform/compose.yaml`.

- **Unit and e2e (no DB):** Run from the registry directory:
  ```powershell
  go test ./...
  ```
  This runs all unit tests and e2e tests that use an embedded NATS server (no PostgreSQL required).

- **Integration tests (with DB), automated:** With **platform** running, use the script to set up the test DB, run tests, and tear it down:
  ```powershell
  .\scripts\run-integration-tests.ps1
  ```
  The script ensures `registry_test` exists on platform Postgres (via docker exec), runs `go test -tags=integration ./...`, then drops `registry_test` when done. No manual DB setup or `psql` on the host required.

  Integration tests:
  - **`pkg/db`** – Repository against a real database (migrations are applied in test).
  - **`tests`** – Full flow: NATS + dispatcher + registry + DB (upsert, resolve, discover, describe, health, listMajors).

---

## 1. Setup and usage

### Prerequisites

- **PostgreSQL** – database for capability metadata (versions, methods, defaults, tenant rules).
- **NATS** – a standalone NATS server (the registry connects as a client; Compose starts one by default).
- **Go 1.22+** – only if building/running the binary locally.

### Database setup

The server stores all capability metadata in **PostgreSQL**. You must have a running Postgres instance and a dedicated database (and user) for the capabilities registry.

#### 1. Create the database and user

Create a database and a user with permissions to create tables and read/write data. Example (run as a superuser in `psql` or your SQL client):

```sql
CREATE USER morezero WITH PASSWORD 'morezero_secret';
CREATE DATABASE morezero OWNER morezero;
-- If you use schema search path or extensions:
-- GRANT ALL ON SCHEMA public TO morezero;
```

On Windows (PowerShell) you can connect with:

```powershell
# If psql is on PATH (e.g. from Postgres install)
psql -U postgres -c "CREATE USER morezero WITH PASSWORD 'morezero_secret';"
psql -U postgres -c "CREATE DATABASE morezero OWNER morezero;"
```

Use your own password and database name; then set `DATABASE_URL` accordingly (see [Environment variables](#environment-variables)).

**When using platform Postgres:** the user and server already exist. The registry **creates its database automatically** on startup if it does not exist (so you can start the app without running a script). To create both `registry` and `registry_test` manually (e.g. for a one-off integration test run without the script), run `.\scripts\ensure-databases.ps1` or pipe `scripts/ensure-databases.sql` into the platform Postgres container.

#### 2. Run migrations

Migrations create and update the schema (tables and indexes). You can run them in either of two ways:

- **CLI (recommended for one-off or CI):** `registry migrate up` (or `capabilities-registry migrate up` when built from repo) — applies migrations and exits. Uses `DATABASE_URL` and `MIGRATION_PATH` from the environment.
- **At server startup:** set `RUN_MIGRATIONS=true` and `MIGRATION_PATH` (default: `migrations`; in Docker use `/app/migrations`).

Migration files in `migrations/` are applied in alphabetical order. They create:

- `capabilities` – logical capability identity (app, name, description, tags, status)
- `capability_versions` – versioned implementations (major/minor/patch, status, subject)
- `capability_methods` – method definitions per version (name, schemas, modes)
- `capability_defaults` – default major version per capability (optional env)
- `capability_tenant_rules` – tenant-specific overrides (optional)

Example (run migrations via CLI, then start server with seed):

```powershell
$env:DATABASE_URL = "postgres://morezero:morezero_secret@localhost:5432/morezero?sslmode=disable"
$env:MIGRATION_PATH = "migrations"
.\capabilities-registry.exe migrate

$env:REGISTRY_BOOTSTRAP_FILE = "config\bootstrap.json"
$env:RUN_MIGRATIONS = "true"
.\capabilities-registry.exe
```

The second run seeds bootstrap; after that use `RUN_MIGRATIONS=false` (or omit) so the server does not re-run migrations on every start. Re-run `capabilities-registry migrate` (or start with `RUN_MIGRATIONS=true`) only when you add or change migration files.

#### 3. Seed bootstrap capabilities (optional)

When `RUN_MIGRATIONS=true`, the server also runs **bootstrap seeding**: it reads `REGISTRY_BOOTSTRAP_FILE` (e.g. `config/bootstrap.json`) and upserts the capabilities defined there (e.g. `system.registry`, `tool.search`) into the database. This populates the registry so `resolve` and `discover` return data. If you skip seeding, the registry will be empty until you register capabilities via the `upsert` API.

Summary:

| Step | Action |
|------|--------|
| 1 | Create PostgreSQL database and user; set `DATABASE_URL` |
| 2 | Set `RUN_MIGRATIONS=true` and `MIGRATION_PATH`; start server once to apply migrations |
| 3 | Bootstrap seed runs automatically when `RUN_MIGRATIONS=true` and `REGISTRY_BOOTSTRAP_FILE` is set |
| 4 | For later runs, set `RUN_MIGRATIONS=false` (or omit) unless you added new migrations |

### CLI commands

The binary (named `registry` in the Docker image) supports subcommands:

| Command | Description |
|---------|-------------|
| `serve` (default) | Start the registry (NATS, HTTP, registry API). |
| `migrate up` | Run database migrations. Uses `DATABASE_URL` and `MIGRATION_PATH`. |
| `migrate status` | Show whether migrations have been applied. |
| `migrate down` | Optional; current migrations are forward-only (no-op with message). |
| `clear` | Truncate all registry tables; schema is preserved. |
| `seed [file]` | Load capabilities from bootstrap JSON. Uses `DATABASE_URL`. |
| `help` | Print usage. |

**Migration workflow:** Run `registry migrate up` once (or when you add new migrations). Do not rely on auto-running migrations at server startup in production unless you use a single instance or a safe leader/lock strategy; prefer a one-off migrate job.

Examples:

```powershell
# Run migrations (set DATABASE_URL and optionally MIGRATION_PATH)
$env:DATABASE_URL = "postgres://morezero:morezero_secret@localhost:5432/morezero?sslmode=disable"
.\capabilities-registry.exe migrate up
.\capabilities-registry.exe migrate status

# Clear registry (data only)
.\capabilities-registry.exe clear

# Seed from default (env or config\bootstrap.json)
$env:REGISTRY_BOOTSTRAP_FILE = "config\bootstrap.json"
.\capabilities-registry.exe seed

# Seed from a specific file (overrides env)
.\capabilities-registry.exe seed path\to\my-bootstrap.json

# Show help
.\capabilities-registry.exe help
```

### Standalone (binary)

1. **Build** (from repo root or `services/capabilities-registry`):

   ```powershell
   cd services\capabilities-registry
   go build -o capabilities-registry.exe .\cmd\capabilities-registry\
   ```

2. **Configure** via environment. See [Environment variables](#environment-variables) for required and optional variables.

3. **Run**:

   ```powershell
   .\run.ps1
   ```

   Or set env and run manually, e.g.:

   ```powershell
   $env:DATABASE_URL = "postgres://user:pass@localhost:5432/morezero?sslmode=disable"
   $env:REGISTRY_BOOTSTRAP_FILE = "config\bootstrap.json"
   $env:RUN_MIGRATIONS = "true"
   .\capabilities-registry.exe
   ```

   On first run with `RUN_MIGRATIONS=true`, the server runs migrations and seeds bootstrap capabilities from `REGISTRY_BOOTSTRAP_FILE`.

### Standalone (Docker / Compose)

- **Build and run** with the included Compose file. It expects a network `more0ai-infra-network` (create it by starting platform first) and uses `POSTGRES_*` / `COMMS_*` / `HTTP_PORT` style env vars (see `compose.yaml`).

  ```powershell
  docker compose -f compose.yaml up -d
  ```

- **Ports**: HTTP (default 8090). Compose also starts a standalone NATS service (client 4222, monitor 8222).
- **Database**: Point `DATABASE_URL` at your Postgres (e.g. `host.docker.internal` for host Postgres).
- Set `RUN_MIGRATIONS=true` and `MIGRATION_PATH=/app/migrations` for first run in container.

### Using from Node/TypeScript (import packages, in-process client)

Your app runs in its own process and talks to the **capabilities registry** over NATS using the **capabilities-client** package. The registry can be the standalone Go binary (or Docker); the client is “in-process” in your Node/TS app.

1. **Install** (in your app or a workspace package):

   ```bash
   pnpm add @morezero/capabilities-client @morezero/capabilities-core
   ```

   Optionally: `@morezero/registry-types` for wire types, `@morezero/capabilities-registry` for schemas and capability base classes if you implement capability handlers.

2. **Configure and initialize** the client (NATS URL must match the registry’s COMMS – e.g. `nats://127.0.0.1:4222` for a local NATS server):

   ```ts
   import { CapabilityClient, defaultCapabilityClientConfig } from "@morezero/capabilities-client";

   const client = new CapabilityClient({
     config: {
       ...defaultCapabilityClientConfig,
       natsUrl: "nats://127.0.0.1:4222",
       registrySubject: "cap.system.registry.v1",  // must match server's registry subject (from bootstrap)
       bootstrapFile: "./config/bootstrap.json",   // optional; for bootstrap-backed resolution
     },
   });

   await client.initialize();
   ```

3. **Use registry APIs** (these call the server over NATS):

   ```ts
   // Resolve capability name → subject
   const resolved = await client.resolve("tool.search");
   // resolved.subject, resolved.resolvedVersion, etc.

   // Discover capabilities
   const discovered = await client.discover({ app: "morezero", tags: ["search"] });

   // Describe a capability
   const described = await client.describe("tool.search");
   ```

4. **Invoke a capability** (resolve → NATS request to resolved subject):

   ```ts
   const result = await client.invoke("tool.search", {
     method: "invoke",
     payload: { query: "test" },
     ctx: { tenantId: "my-tenant" },
   });
   ```

5. **Shutdown**:

   ```ts
   await client.close();
   ```

Ensure the **capabilities registry** is running and that `registrySubject` (and optionally bootstrap) matches the server’s configuration (e.g. `cap.system.registry.v1` from `config/bootstrap.json`).

### Environment variables

All configuration is read from environment variables (no config file besides bootstrap JSON). The server uses default values where shown; override as needed for your environment.

#### Required

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string. The server cannot run without a valid database. Example: `postgres://user:password@host:5432/dbname?sslmode=disable`. |

When `RUN_MIGRATIONS=true`, the server seeds the database from bootstrap. You can set `REGISTRY_BOOTSTRAP_FILE` to point to your bootstrap JSON, or the loader will try `config/bootstrap.json`, then `bootstrap.json`, then built-in defaults.

#### Optional

**Database**

| Variable | Default | Description |
|----------|---------|-------------|
| `RUN_MIGRATIONS` | `false` | Set to `true` to run SQL migrations from `MIGRATION_PATH` at startup and then seed from bootstrap. Use on first run or after adding migrations. |
| `MIGRATION_PATH` | `migrations` | Directory containing `.sql` migration files (relative to working directory). In Docker, use `/app/migrations`. |

**COMMS (NATS)**

| Variable | Default | Description |
|----------|---------|-------------|
| `COMMS_URL` | `nats://127.0.0.1:4222` | NATS server URL. The registry connects to this standalone NATS (no embedded server). |
| `SERVICE_NAME` | `capabilities-registry` | Client name for the NATS connection (identifies this server in NATS). |

**Registry**

| Variable | Default | Description |
|----------|---------|-------------|
| `REGISTRY_SUBJECT` | (from bootstrap) | NATS subject the server subscribes to for registry requests. Empty = use subject for `system.registry` from bootstrap (e.g. `cap.system.registry.v1`). |
| `REGISTRY_CHANGE_EVENT_SUBJECT` | `registry.changed` | Global subject for publishing registry change events (used by clients for cache invalidation). |
| `REGISTRY_BOOTSTRAP_FILE` | (none) | Path to bootstrap JSON. Used at startup to resolve registry subject and (when `RUN_MIGRATIONS=true`) to seed capabilities. Bootstrap loader also tries `config/bootstrap.json`, `bootstrap.json` and built-in defaults if unset. |
| `REGISTRY_REQUEST_TIMEOUT` | `25s` | Maximum duration for handling a single registry request. |

**HTTP**

| Variable | Default | Description |
|----------|---------|-------------|
| `REGISTRY_HTTP_ADDR` | (none) | Listen address for the HTTP server (e.g. `0.0.0.0:8080`). If unset, `HTTP_PORT` is used. |
| `HTTP_PORT` | `8080` | Port for the HTTP server when `REGISTRY_HTTP_ADDR` is not set (health, ready, registry UI, capability docs). |
| `HEALTH_CHECK_TIMEOUT` | `5s` | Timeout for health checks (e.g. DB ping) when serving `/health` and `/healthz`. |

**Logging**

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, or `error`. |

---

## 2. APIs to interact with the server

Interaction is via **NATS request/reply** (registry API) and **HTTP** (health and UI).

### NATS registry API (request/reply)

The server subscribes to a **registry subject** (e.g. `cap.system.registry.v1` from bootstrap or `REGISTRY_SUBJECT`). Clients send JSON **requests** and receive JSON **responses**.

**Request envelope** (aligns with `pkg/dispatcher/envelope.go` and `@morezero/registry-types/wire`):

```json
{
  "id": "<uuid>",
  "type": "invoke",
  "cap": "system.registry",
  "method": "<methodName>",
  "params": { ... },
  "ctx": {
    "tenantId": "...",
    "userId": "...",
    "requestId": "...",
    "timeoutMs": 30000
  }
}
```

**Response envelope**:

```json
{
  "id": "<same-as-request>",
  "ok": true,
  "result": { ... }
}
```

or on error:

```json
{
  "id": "<same-as-request>",
  "ok": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "...",
    "details": {},
    "retryable": false
  }
}
```

**Registry methods** (handled by the server):

| Method | Description | Params (key fields) | Result type |
|--------|-------------|----------------------|-------------|
| `resolve` | Resolve capability name (and optional version) to subject and metadata | `cap`, `ver?`, `ctx?`, `includeMethods?`, `includeSchemas?` | `ResolveOutput` (subject, major, resolvedVersion, status, ttlSeconds, etag, methods?, schemas?) |
| `discover` | List capabilities with optional filters and pagination | `app?`, `tags?`, `query?`, `status?`, `supportsMethod?`, `page?`, `limit?` | `DiscoverOutput` (capabilities[], pagination) |
| `describe` | Full description of a capability (methods, schemas) | `cap`, `major?`, `version?` | `DescribeOutput` |
| `upsert` | Create or update a capability version | `app`, `name`, `version`, `methods`, etc. | `UpsertOutput` |
| `setDefaultMajor` | Set default major version for a capability | `cap`, `major`, `env?` | `SetDefaultMajorOutput` |
| `deprecate` | Mark version(s) deprecated | `cap`, `version?`, `major?`, `reason` | `DeprecateOutput` |
| `disable` | Disable version(s) | `cap`, `version?`, `major?`, `reason` | `DisableOutput` |
| `listMajors` | List major versions for a capability | `cap`, `includeInactive?` | `ListMajorsOutput` |
| `health` | Health check (DB, COMMS) | (none) | `HealthOutput` |

Input/output shapes match the Go `pkg/registry` types and `@morezero/registry-types` (e.g. `registry-methods`, `wire`). Example raw NATS request (CLI):

```bash
nats req cap.system.registry.v1 '{"id":"test","type":"invoke","cap":"system.registry","method":"health","params":{},"ctx":{"tenantId":"system"}}'
```

### HTTP endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Registry home page: health, stats, list of capabilities (HTML) |
| `GET /health` | JSON health (status, checks.database, timestamp). Returns 503 if unhealthy |
| `GET /healthz` | Same as `/health` (for readiness probes, e.g. Kubernetes) |
| `GET /ready` | Simple readiness JSON `{"status":"ready"}` |
| `GET /capability/<cap>` | Capability detail page (describe output, HTML) |
| `GET /capability/<cap>/openapi.json` | OpenAPI 3.0 spec for the capability’s methods |
| `GET /capability/<cap>/docs` | Swagger UI for the capability API |

---

## Production and versioning

### Running in production

- **Docker image:** `ghcr.io/more0ai/registry:<version>` (e.g. `ghcr.io/more0ai/registry:1.2.3`). Tags match Git tags without the `v` prefix (`v1.2.3` → `1.2.3`).
- **Example run:**

  ```bash
  docker run -d --name registry \
    -e DATABASE_URL="postgres://user:pass@host:5432/dbname?sslmode=require" \
    -e COMMS_URL="nats://nats-host:4222" \
    -e REGISTRY_HTTP_ADDR="0.0.0.0:8080" \
    -p 8080:8080 \
    ghcr.io/more0ai/registry:1.2.3
  ```

- **Migrations:** Run as a one-off job before or during deployment (e.g. `docker run --rm -e DATABASE_URL=... ghcr.io/more0ai/registry:1.2.3 registry migrate up`). Do not rely on `RUN_MIGRATIONS=true` at server startup in multi-instance setups unless you use a migration lock/leader election.
- **Kubernetes / ECS:** Use the image as a deployment; add a `livenessProbe` / `readinessProbe` on `GET /healthz` (or `/health`). Run migrations as a Job or init container. Provide `DATABASE_URL` and `COMMS_URL` via secrets or env.

### Versioning

- **Docker tags** are derived from Git tags: pushing tag `v1.2.3` publishes `ghcr.io/more0ai/registry:1.2.3`. We do not push a `latest` tag by default.
- **Releases:** GitHub Actions build and push the image to GHCR on every `v*` tag and attach binaries (linux/amd64, linux/arm64, darwin/arm64) to the GitHub Release.

---

## 3. How it works

### Component overview

- **Config** – Loads from env (database, COMMS, bootstrap path, migrations, HTTP, timeouts, logging).
- **Bootstrap** – Loads bootstrap JSON to get registry subject and system capability definitions; used for seeding and for resolving the registry subject when `REGISTRY_SUBJECT` is not set.
- **COMMS** – Connects to **standalone NATS** via `COMMS_URL`. The server connects as a client and subscribes to the registry subject.
- **Database** – PostgreSQL: capabilities, versions, methods, defaults, tenant rules. Migrations and optional bootstrap seed run at startup when `RUN_MIGRATIONS=true`.
- **Registry** – Core logic: resolve, discover, describe, upsert, setDefaultMajor, deprecate, disable, listMajors, health. Uses DB and optional **events publisher** for change notifications.
- **Dispatcher** – Translates incoming NATS messages into registry method calls and serializes responses (request/reply).
- **Events** – On registry mutations, publishes to a global change subject and a granular subject (e.g. `registry.changed.<app>.<capability>`) so clients can invalidate caches.
- **HTTP server** – Health, ready, and simple HTML/OpenAPI/Swagger UI for the registry and per-capability docs.

### Component diagrams

**High-level: capabilities registry and clients**

```text
┌─────────────────────────────────────────────────────────────────────────┐
│                     Capabilities Registry (Go process)                   │
│  ┌──────────────┐   ┌─────────────┐   ┌───────────┐   ┌───────────────┐  │
│  │ Config       │   │ Bootstrap   │   │ COMMS     │   │ HTTP Server   │  │
│  │ (env)        │   │ (JSON)      │   │ (NATS)    │   │ /health, /    │  │
│  └──────┬───────┘   └──────┬──────┘   └─────┬─────┘   └───────────────┘  │
│         │                  │                │                             │
│         ▼                  ▼                ▼                             │
│  ┌──────────────┐   ┌─────────────┐   ┌─────────────────────────────┐   │
│  │ Migrations   │   │ Registry     │   │ Dispatcher                  │   │
│  │ + DB seed    │   │ (resolve,    │   │ (NATS sub → Registry calls) │   │
│  └──────┬───────┘   │  discover,   │   └─────────────────────────────┘   │
│         │           │  describe,   │                ▲                     │
│         ▼           │  upsert, …)  │                │ request/reply      │
│  ┌──────────────┐   └──────┬───────┘                │                     │
│  │ PostgreSQL   │◄─────────┘                        │                     │
│  └──────────────┘   ┌──────▼───────┐   ┌────────────┴──────────────┐     │
│                     │ Events       │   │ Standalone NATS             │     │
│                     │ (change      │   │ (COMMS_URL)                 │     │
│                     │  publish)    │   └────────────────────────────┘     │
│                     └─────────────┘                                        │
└─────────────────────────────────────────────────────────────────────────┘
         │                    │                              ▲
         │                    │ change events                 │ NATS
         │                    ▼                              │ (registry + capability
         │             ┌─────────────┐                        │  subjects)
         │             │ NATS        │                        │
         │             │ (publish)   │                        │
         │             └──────┬──────┘                        │
         │                    │                              │
         ▼                    ▼                              │
┌─────────────────────────────────────────────────────────────────────────┐
│  Node/TS app (in-process client)                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │ @morezero/capabilities-client                                        │ │
│  │  resolve(), discover(), describe(), invoke() → NATS request/reply   │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

**Request path: registry call over NATS**

```text
Client (e.g. capabilities-client)          Capabilities Registry
         │                                            │
         │  NATS request                               │
         │  subject: cap.system.registry.v1          │
         │  body: { method, params, ctx }             │
         │ ─────────────────────────────────────────► │
         │                                            │ Dispatcher
         │                                            │   → Registry (resolve/discover/…)
         │                                            │   → DB / bootstrap
         │  NATS response                             │
         │  body: { ok, result } or { ok: false, error }│
         │ ◄───────────────────────────────────────── │
         │                                            │
```

### Data flow

1. **Startup**: Load config → load bootstrap → start or connect COMMS → connect DB → run migrations (optional) → seed bootstrap (optional) → create Registry and Dispatcher → subscribe to registry subject → start HTTP server.
2. **Registry request**: NATS message on registry subject → Dispatcher decodes request → calls Registry method (resolve, discover, describe, upsert, …) → Registry uses DB (and bootstrap for system capabilities) → Dispatcher encodes response → NATS reply.
3. **Mutations** (upsert, setDefaultMajor, deprecate, disable): Registry updates DB and publishes change events so clients can invalidate resolution/discovery caches.
4. **Shutdown**: Unsubscribe, drain NATS, close DB.

### Bootstrap and subjects

- **Bootstrap file** (`config/bootstrap.json` by default) defines:
  - **System capabilities**, including `system.registry` with its NATS **subject** (e.g. `cap.system.registry.v1`).
  - Other capability subjects (e.g. `cap.tool.search.v1`) and aliases.
- The server uses the **registry subject** from bootstrap for `system.registry` unless `REGISTRY_SUBJECT` is set. Clients must use the **same subject** (e.g. `cap.system.registry.v1`) in their config (`registrySubject`) so their requests reach this server.
- Capability subjects follow a convention (e.g. `cap.<app>.<name>.v<major>`); the registry **resolve** method returns the subject for a given capability/version so callers can then send invoke requests to that subject (handled by workers or other services, not by this server).

---

## Related packages and repos

- **`@morezero/capabilities-client`** – TypeScript client (resolve, discover, describe, invoke pipeline over NATS).
- **`@morezero/capabilities-core`** – Shared types, pipeline, middleware.
- **`@morezero/capabilities-registry`** – Schemas and capability base classes for implementing handlers (e.g. in workers).
- **`@morezero/registry-types`** – Wire and registry method types (request/response, resolve, discover, describe, etc.).
- **`apps/capabilities-worker`** – Example worker that subscribes to capability subjects and runs handlers; uses bootstrap to resolve capability names to subjects.
- **`Docs/registry/`** – Deeper design and data model documentation.
