# Run integration tests with automated setup and teardown.
# - Ensures platform Postgres has registry_test (creates if missing via docker exec).
# - Runs go test -tags=integration ./...
# - Drops registry_test when done so the test run is self-contained.
# Requires: platform running (e.g. platform/start.ps1), docker, go.
# Run from registry directory: .\scripts\run-integration-tests.ps1

$ErrorActionPreference = "Stop"

$container = if ($env:PLATFORM_POSTGRES_CONTAINER) { $env:PLATFORM_POSTGRES_CONTAINER } else { "more0ai-postgres" }

# Default DATABASE_URL for integration tests if not set
if (-not $env:DATABASE_URL) {
    $user = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { "morezero" }
    $pass = if ($env:POSTGRES_PASSWORD) { $env:POSTGRES_PASSWORD } else { "morezero" }
    $host = if ($env:POSTGRES_HOST) { $env:POSTGRES_HOST } else { "localhost" }
    $port = if ($env:POSTGRES_PORT) { $env:POSTGRES_PORT } else { "5432" }
    $env:DATABASE_URL = "postgres://${user}:${pass}@${host}:${port}/registry_test?sslmode=disable"
}

# Resolve script dir and repo root (registry)
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$registryRoot = Split-Path -Parent $scriptDir
$ensureSql = Join-Path $scriptDir "ensure-databases.sql"

# Check platform Postgres container is running
$running = docker ps -q -f "name=$container" 2>$null
if (-not $running) {
    Write-Error "run-integration-tests.ps1: Postgres container '$container' not running. Start platform first (e.g. platform/start.ps1)."
    exit 1
}

# Ensure registry and registry_test exist (idempotent)
Write-Host "Ensuring registry_test database exists..."
Get-Content $ensureSql -Raw | docker exec -i $container psql -U morezero -d postgres -f -
if ($LASTEXITCODE -ne 0) {
    Write-Error "run-integration-tests.ps1: failed to ensure databases."
    exit $LASTEXITCODE
}

$testsPassed = $false
try {
    Push-Location $registryRoot
    go test -tags=integration ./... -count=1
    $testsPassed = ($LASTEXITCODE -eq 0)
} finally {
    Pop-Location
    # Teardown: drop test DB so next run is clean
    Write-Host "Dropping registry_test database..."
    docker exec -i $container psql -U morezero -d postgres -c "DROP DATABASE IF EXISTS registry_test;" 2>&1 | Out-Null
}

if (-not $testsPassed) { exit 1 }
exit 0
