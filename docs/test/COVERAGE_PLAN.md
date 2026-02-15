# Test Coverage Plan

Comprehensive plan to achieve and maintain high test coverage for the capabilities-registry codebase.

---

## 1. Current State

### 1.1 Package coverage (baseline)

| Package | Coverage | Primary gaps |
|---------|----------|--------------|
| `cmd/registry` | 0% | main, all CLI commands |
| `internal/config` | ~21% | ValidateForServe, ValidateForDB |
| `internal/server` | ~4% | Run(), all HTTP/NATS handlers |
| `pkg/db` | ~4% | pool, repository, ensure, clear, seed, registries, migrations runtime |
| `pkg/registry` | ~12% | Discover, Describe, Upsert, Deprecate, SetDefault, ListMajors, federation, GetBootstrapCapabilities |
| `pkg/dispatcher` | ~16% | All handle* with real registry, error paths |
| `pkg/commsutil` | ~36% | Connect() |
| `pkg/bootstrap` | ~55% | Edge cases, CreateResolvedBootstrap |
| `pkg/events` | ~74% | Maintain; add for new code |
| `pkg/semver` | ~80% | Maintain |

### 1.2 Existing tests (keep and extend)

- **internal/config**: `config_test.go` — LoadConfig defaults/overrides/log levels
- **internal/server**: `server_test.go` — buildOpenAPISpec only
- **pkg/db**: `migrations_test.go` — LoadMigrationFiles, containsInt; `integration_test.go` (build tag)
- **pkg/registry**: `registry_test.go`, `resolve_test.go`, `health_test.go`, `resolve_federation_test.go`, `types_test.go`, `helpers_test.go`
- **pkg/dispatcher**: `dispatcher_test.go`, `dispatch_routing_test.go`
- **pkg/commsutil**: `codec_test.go`, `subjects_test.go`
- **pkg/events**: `publisher_test.go`, `comms_publisher_integration_test.go`
- **pkg/semver**: `parser_test.go`, `resolver_test.go`
- **pkg/bootstrap**: `loader_test.go`
- **tests**: `integration_test.go`, `e2e_test.go` (build tags)

---

## 2. Goals

- **Target**: Unit and integration coverage such that critical paths are tested and refactors are safe.
- **Metrics**: Aim for ≥70% statement coverage on `internal/`, `pkg/` (excluding `cmd/`); `cmd/registry` covered by CLI tests and integration.
- **Quality**: Table-driven tests where applicable; consistent use of package+method log prefixes in test output; no flaky tests.

---

## 3. Prerequisites and patterns

### 3.1 Test infrastructure

- **Database**: Use testcontainers (e.g. `testcontainers-go` with Postgres) or a dedicated test DB (e.g. `DATABASE_URL` pointing to `.../registry_test`) for `pkg/db` and integration tests. Document in README and CI.
- **NATS**: Existing pattern of starting in-memory NATS in tests (e.g. `tests/integration_test.go`). Reuse for handler tests that need NATS.
- **Mocks**: Introduce interfaces where needed so registry and DB can be mocked in server and dispatcher tests (see Phase 2).

### 3.2 Conventions

- Log messages in tests: use `package:test_file` or `package:TestName` prefix per project rules.
- Prefer table-driven tests for multiple inputs (e.g. validation, status codes).
- Use `t.Helper()` in test helpers.
- Integration tests: `//go:build integration`; unit tests run with default `go test ./...`.

### 3.3 Running tests

```powershell
# Unit tests only (no integration)
go test ./...

# With coverage
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out

# Integration tests (requires DATABASE_URL, optional NATS)
go test -tags=integration ./tests/...
```

---

## 4. Phase 1: Config and server handlers (quick wins)

**Goal**: Validate config and exercise HTTP surface without full Run().

### 4.1 internal/config

**File**: `internal/config/config_test.go`

| Test | Description |
|------|--------------|
| `TestValidateForServe_EmptyDatabaseURL` | DATABASE_URL empty → error |
| `TestValidateForServe_ZeroRequestTimeout` | REGISTRY_REQUEST_TIMEOUT ≤ 0 → error |
| `TestValidateForServe_ZeroHealthCheckTimeout` | HEALTH_CHECK_TIMEOUT ≤ 0 → error |
| `TestValidateForServe_Valid` | All required set and positive → nil |
| `TestValidateForDB_EmptyDatabaseURL` | DATABASE_URL empty → error |
| `TestValidateForDB_Valid` | DATABASE_URL set → nil |

**Acceptance**: All validation branches covered; no regression when changing env vars.

### 4.2 internal/server (handlers only)

**File**: `internal/server/server_test.go` (extend)

Strategy: Construct `Server` with test config and a **mock registry** (or minimal fake that returns fixed data). Do not start NATS or real DB.

| Test | Description |
|------|--------------|
| `TestHandleHome_Success` | GET / → 200, HTML contains health and discover content (mock Discover, Health) |
| `TestHandleHome_DiscoverError` | Mock Discover error → page shows error message, 200 |
| `TestHandleHome_OnlyRoot` | GET /other → 404 |
| `TestHealthHandler_Healthy` | GET /health with healthy mock → 200, JSON status "healthy" |
| `TestHealthHandler_Unhealthy` | GET /health with unhealthy mock → 503, JSON status "unhealthy" |
| `TestReadyHandler` | GET /ready → 200, JSON `{"status":"ready"}` |
| `TestConnectionHandler_GET` | GET /connection → 200, JSON `natsUrl` |
| `TestConnectionHandler_MethodNotAllowed` | POST /connection → 405, Allow: GET |
| `TestHandleCapabilityDetail_NotFound` | GET /capability/nonexistent → 404 (mock Describe NOT_FOUND) |
| `TestHandleCapabilityDetail_Success` | GET /capability/cap.name with mock Describe → 200, HTML |
| `TestHandleCapabilityDetail_OpenAPISpec` | GET /capability/cap.name/openapi.json → 200, valid OpenAPI JSON |
| `TestHandleCapabilityDetail_Docs` | GET /capability/cap.name/docs → 200, HTML with Swagger UI |
| `TestHandleCapabilityDetail_InvalidPath` | GET /capability/ → redirect or 404 as implemented |

**Implementation note**: To avoid starting full server, either:
- Export handler constructors that take a registry interface and config, or
- Use a test-only constructor that accepts mocks. Prefer injecting an interface (e.g. `HealthChecker`, `Discoverer`, `Describer`) so handlers can be tested with a small fake.

**Acceptance**: Handler tests pass without NATS/DB; coverage for server package increases (target ~30%+ for handler code).

---

## 5. Phase 2: Registry package (core logic)

**Goal**: Unit-test all registry operations with a mock repository so business logic and error paths are covered.

### 5.1 Repository interface (optional but recommended)

**Location**: `pkg/registry` or `pkg/db`

- Define an interface (e.g. `RegistryRepo`) that includes the methods used by `Registry` (GetCapability, ListCapabilities, GetVersions, GetDefaultsBatch, UpsertCapability, UpsertVersion, etc.).
- `db.Repository` implements it. Tests use a **mock** or **fake** that returns controlled data or errors.

If introducing an interface is too large a refactor initially, use integration tests with a real DB (Phase 4) and still add unit tests for pure functions (e.g. validation in upsert).

### 5.2 pkg/registry unit tests

**Files**: New or extended `*_test.go` in `pkg/registry`.

| Area | File | Tests |
|------|------|--------|
| **Discover** | `discover_test.go` | RequireRepo; invalid page/limit (defaults, max); status filter; repo ListCapabilities error; empty results; pagination fields |
| **Describe** | `describe_test.go` | RequireRepo; invalid cap ref (ParseCapabilityRef error); capability not found; no versions; version by string match; by major; by latest; repo error paths |
| **Upsert** | `upsert_test.go` | validateUpsertInput: invalid app/name, version bounds, zero methods, too many methods, metadata/schema/examples size limits; RequireRepo; full Upsert with mock repo (optional if interface exists) |
| **Deprecate** | `deprecate_test.go` | RequireRepo; invalid params; version not found; success path with mock |
| **SetDefault** | `set_default_test.go` | RequireRepo; invalid params; success with mock |
| **ListMajors** | `list_majors_test.go` | RequireRepo; invalid cap; no capability; no versions; success with mock |
| **GetBootstrapCapabilities** | `registry_test.go` or `bootstrap_test.go` | Nil repo → empty map; with mock repo: entries, NatsUrl default, includeMethods/includeSchemas |
| **LoadRegistryAliases** | `registry_test.go` or `federation_test.go` | Nil federation pool → default alias; with mock: alias map and default |
| **Federation** | `resolve_federation_test.go` (extend) / `federation_test.go` | Resolve: unknown alias, nil NatsUrl, nil RegistrySubject; LoadRegistryAliases; connection reuse; error on request failure |

**Acceptance**: Registry package coverage ≥60%; all public methods have at least one success and one error test where applicable.

---

## 6. Phase 3: Database layer (pkg/db)

**Goal**: Unit tests for pool, migrations runtime, ensure, clear, seed, and repository with a real test database.

### 6.1 Test database

- **Option A**: CI and local use `testcontainers-go` to start Postgres; set `DATABASE_URL` for `pkg/db` tests.
- **Option B**: Use a fixed test DB (e.g. `postgres://.../registry_test`). Document in `docs/test/README.md` and `.env.example`.
- Run migrations in test setup so schema exists for repository tests.

### 6.2 pkg/db tests

| File | Tests |
|------|--------|
| **pool_test.go** (new) | NewPool: invalid URL → error; valid URL (test DB) → success, Ping; optional: pool config (MaxConns, MinConns). RunMigrations: empty list → success; invalid SQL → error. MigrationStatus: table exists vs not. MigrationDown: no-op message. |
| **ensure_test.go** (new) | EnsureDatabase: invalid URL → error; empty dbname → error; invalid chars in dbname → error; valid URL with existing DB → success; (optional) create new DB on test Postgres. buildPostgresURL, quoteIdent (unit). |
| **clear_test.go** (new) | ClearRegistry: with empty test DB → success; after inserting data → tables empty. |
| **seed_capability_metadata_test.go** (new) | SeedFromCapabilityMetadataFile: empty path → nil; path traversal (baseDir) → error; file not found → nil (or error per impl); valid JSON file → capability/version/methods/defaults upserted; idempotent run. |
| **registries_test.go** (new) | GetRegistryByAlias: no rows → nil; row exists → entry. GetDefaultRegistry, ListRegistries with test data. |
| **repository_test.go** (new) | With test DB: GetCapability (found/not found), UpsertCapability, ListCapabilities (pagination, filters), GetVersions, GetVersion, UpsertVersion, UpdateVersionStatus, GetMethods, UpsertMethod, GetDefault, SetDefault, GetTenantRules, CheckTenantAccess, IncrementRevision, ListBootstrapEntries. Scan helpers covered indirectly. |

**Acceptance**: pkg/db coverage ≥50%; repository CRUD and seed/clear/ensure have deterministic tests.

---

## 7. Phase 4: Dispatcher and CLI

**Goal**: Dispatch all methods with a mock registry; test CLI commands with test config and DB.

### 7.1 pkg/dispatcher

**File**: `pkg/dispatcher/dispatcher_test.go` (extend)

| Test | Description |
|------|-------------|
| Table: **method + invalid params** | resolve, discover, describe, upsert, setDefaultMajor, deprecate, disable, listMajors with bad JSON or missing params → Ok: false, INVALID_ARGUMENT |
| Table: **method + registry error** | Mock registry returns RegistryError → response Ok: false, correct code |
| Table: **method + success** | Mock registry returns result → Ok: true, Result set |
| **Unknown method** | method "unknown" → METHOD_NOT_FOUND |
| **UserID from context** | req.Ctx.UserID set → passed to Upsert/Deprecate/etc. |

Use a **mock registry** (interface or test double) that implements Resolve, Discover, Describe, Upsert, etc., and returns controlled results or errors.

**Acceptance**: Dispatcher coverage ≥70%; every method and main error path covered.

### 7.2 cmd/registry (CLI)

**File**: `cmd/registry/main_test.go` or `cmd/registry/cli_test.go`

| Test | Description |
|------|-------------|
| **Main_Help** | os.Args = ["registry", "help"] → exit 0, usage printed (or "-h"/"--help") |
| **Main_UnknownCommand** | os.Args = ["registry", "unknown"] → exit 1, stderr message |
| **Main_MigrateMissingSubcommand** | os.Args = ["registry", "migrate"] → exit 1 |
| **Main_MigrateUnknownSubcommand** | os.Args = ["registry", "migrate", "foo"] → exit 1 |
| **RunMigrateUp** | With test DATABASE_URL and migration path → migrations run (or skip if no DB) |
| **RunMigrateStatus** | With test DB → prints status |
| **RunMigrateDown** | With test DB → prints "not supported" message |
| **RunClear** | With test DB → ClearRegistry called / tables empty |
| **RunSeed** | With test DB and bootstrap path → SeedFromCapabilityMetadataFile called / data present |

CLI tests can set `os.Args` and `os.Stderr`/`os.Stdout`; use `config.LoadConfig()` with env vars set so DATABASE_URL and MIGRATION_PATH point to test resources. For commands that need DB, use the same test DB as pkg/db.

**Acceptance**: All subcommands have at least one test; main exit paths covered.

---

## 8. Phase 5: Integration and E2E

**Goal**: Keep integration and e2e tests stable and run them in CI.

### 8.1 tests/integration_test.go

- Ensure test runs with `-tags=integration` and DATABASE_URL (and optional NATS port).
- Cover: resolve, discover, describe, upsert, setDefaultMajor, deprecate, health (via NATS and/or HTTP if server is started).
- Document in `docs/test/README.md`: how to run, required env, scripts (e.g. `scripts/ensure-databases.ps1`).

### 8.2 tests/e2e_test.go

- If present: full server start (NATS + DB + HTTP), client requests to health/ready/connection and registry subject.
- Ensure CI runs e2e when tag is set (e.g. `go test -tags=integration,e2e ./tests/...`).

### 8.3 CI

- **Unit**: `go test ./...` (excluding or including `cmd/registry` as desired).
- **Coverage**: `go test ./... -coverprofile=coverage.out -covermode=atomic`; fail or warn if coverage drops below threshold (e.g. 50% per package or overall).
- **Integration**: `go test -tags=integration ./tests/...` with DATABASE_URL (and NATS) configured.

---

## 9. Phase 6: Remaining packages and maintenance

### 9.1 pkg/commsutil

**File**: `pkg/commsutil/connect_test.go` (new)

- Connect with invalid URL (e.g. "invalid://") → error.
- Optional: Connect with real NATS (e.g. start embedded server) → success, Close.

### 9.2 pkg/bootstrap

- Extend `loader_test.go`: more file shapes, missing file, invalid JSON.
- Add tests for `CreateResolvedBootstrap` (subject resolution, aliases) if used by server and not covered.

### 9.3 pkg/events and pkg/semver

- Keep existing coverage; add tests for any new code or branches.
- No major new files unless new behavior is added.

### 9.4 internal/server Run()

- **Option A**: Large integration test that starts server with test NATS + test DB, hits health/ready/connection and NATS subject, then shuts down (signal or timeout). Expensive but validates full stack.
- **Option B**: Leave Run() to e2e and manual testing; rely on handler unit tests and integration tests for components.

Recommendation: Phase 1–4 give most value; add Run() integration in Phase 5 or 6 if needed.

---

## 10. Checklist summary

Use this as a high-level checklist; detailed steps are in the phases above.

- [ ] **Phase 1**: Config validation tests; server handler tests with mock registry
- [ ] **Phase 2**: Registry Discover, Describe, Upsert (validation + paths), Deprecate, SetDefault, ListMajors, GetBootstrapCapabilities, LoadRegistryAliases; federation tests
- [ ] **Phase 3**: db pool, ensure, clear, seed, registries, repository tests (with test DB)
- [ ] **Phase 4**: Dispatcher tests with mock registry; cmd/registry CLI tests
- [ ] **Phase 5**: Integration and e2e documented and run in CI; coverage gate
- [ ] **Phase 6**: commsutil Connect, bootstrap edge cases; maintain events/semver

---

## 11. Document history

| Date | Change |
|------|--------|
| Initial | Created from code review and coverage analysis. |
