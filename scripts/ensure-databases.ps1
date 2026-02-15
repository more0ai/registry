# Ensure registry and registry_test databases exist on Postgres (e.g. platform instance).
# Uses POSTGRES_* env vars if set; otherwise defaults for local platform.
# Run from registry directory: .\scripts\ensure-databases.ps1

$ErrorActionPreference = "Stop"
$user = if ($env:POSTGRES_USER) { $env:POSTGRES_USER } else { "morezero" }
$pgHost = if ($env:POSTGRES_HOST) { $env:POSTGRES_HOST } else { "localhost" }
$port = if ($env:POSTGRES_PORT) { $env:POSTGRES_PORT } else { "5432" }
if ($env:POSTGRES_PASSWORD) { $env:PGPASSWORD = $env:POSTGRES_PASSWORD }

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sqlPath = Join-Path $scriptDir "ensure-databases.sql"

& psql -U $user -d postgres -h $pgHost -p $port -f $sqlPath
if ($LASTEXITCODE -ne 0) {
    Write-Error "ensure-databases.ps1: psql failed. Ensure Postgres is running (e.g. platform) and $user can create databases."
    exit $LASTEXITCODE
}
Write-Host "Registry databases (registry, registry_test) are ready."
