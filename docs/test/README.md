# Test documentation

This directory holds the test strategy and coverage plan for the capabilities-registry project.

## Contents

| Document | Purpose |
|----------|---------|
| [COVERAGE_PLAN.md](./COVERAGE_PLAN.md) | Comprehensive plan to complete test coverage: current state, phases, test cases, and checklists. |

## Quick reference

### Run unit tests

```powershell
go test ./...
```

### Run tests with coverage

```powershell
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -func coverage.out
go tool cover -html coverage.out
```

### Run integration tests

Requires `DATABASE_URL` (e.g. to a `registry_test` database). Create test DBs with `scripts/ensure-databases.ps1` if available.

```powershell
$env:DATABASE_URL = "postgres://user:pass@localhost:5432/registry_test?sslmode=disable"
go test -tags=integration ./tests/...
```

### Coverage goals

- **Target**: ≥70% statement coverage for `internal/` and `pkg/` (see [COVERAGE_PLAN.md](./COVERAGE_PLAN.md) for per-package targets).
- **CI**: Run `go test ./...` and optionally fail or warn when coverage drops below threshold.

## Implementing the plan

Follow [COVERAGE_PLAN.md](./COVERAGE_PLAN.md) phase by phase:

1. **Phase 1**: Config validation + server handler tests.
2. **Phase 2**: Registry unit tests (Discover, Describe, Upsert, etc.) with mocks.
3. **Phase 3**: Database layer tests (pool, ensure, clear, seed, repository) with test DB.
4. **Phase 4**: Dispatcher and CLI tests.
5. **Phase 5**: Integration/e2e in CI and coverage gate.
6. **Phase 6**: commsutil, bootstrap, and ongoing maintenance.

Each phase includes concrete test names and acceptance criteria in the plan.
