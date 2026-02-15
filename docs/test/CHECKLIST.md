# Test coverage implementation checklist

Use this checklist to track progress. Details and test case lists are in [COVERAGE_PLAN.md](./COVERAGE_PLAN.md).

## Phase 1: Config and server handlers

- [x] **internal/config**
  - [x] ValidateForServe: empty DATABASE_URL, zero timeouts, valid
  - [x] ValidateForDB: empty DATABASE_URL, valid
- [x] **internal/server**
  - [x] Handler tests: home, health, ready, connection, capability detail (success, not found, openapi, docs)
  - [x] Mock registry (registryForServer interface) for handler tests

## Phase 2: Registry package

- [x] **Discover** (discover_test.go): requireRepo
- [x] **Describe** (describe_test.go): requireRepo
- [x] **Upsert** (upsert_test.go): validateUpsertInput cases, requireRepo
- [x] **Deprecate / SetDefault / ListMajors**: requireRepo (deprecate_test, set_default_test, list_majors_test)
- [x] **GetBootstrapCapabilities / LoadRegistryAliases**: nil repo (registry_test.go)
- [ ] **Federation** (federation_test.go / resolve_federation_test.go): alias errors, resolve, LoadRegistryAliases (existing tests; extend as needed)

## Phase 3: Database layer (pkg/db)

- [ ] Test DB setup (testcontainers or registry_test) for full repository tests
- [x] pool_test.go: NewPool invalid/empty URL
- [x] ensure_test.go: URL validation, quoteIdent, buildPostgresURL, empty dbname, invalid dbname chars
- [ ] clear_test.go: ClearRegistry (requires test DB)
- [x] seed_capability_metadata_test.go: empty path, path traversal rejected
- [ ] registries_test.go: GetRegistryByAlias, etc. (requires test DB)
- [ ] repository_test.go: CRUD with test DB

## Phase 4: Dispatcher and CLI

- [x] **pkg/dispatcher**: Dispatch with nil-repo registry (resolve, discover, describe, listMajors → INTERNAL_ERROR); health → Ok with unhealthy; invalid params → INVALID_ARGUMENT
- [x] **cmd/registry**: usage string tests (main_test.go)

## Phase 5: Integration and CI

- [ ] Integration tests documented and runnable with DATABASE_URL
- [ ] CI runs unit tests and (optionally) integration tests
- [ ] Coverage gate or report (e.g. threshold or badge)

## Phase 6: Remaining and maintenance

- [x] pkg/commsutil: Connect_test (invalid URL)
- [ ] pkg/bootstrap: loader edge cases, CreateResolvedBootstrap if needed
- [ ] Keep pkg/events and pkg/semver coverage on new changes
