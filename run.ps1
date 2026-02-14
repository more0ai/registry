# Run capabilities-registry directly (no Docker).
# Requires: PostgreSQL at DATABASE_URL and a standalone NATS server at COMMS_URL. Build first: go build -o capabilities-registry.exe .\cmd\capabilities-registry\
#
# Optional env (defaults shown):
#   COMMS_URL=nats://127.0.0.1:4222  - standalone NATS server URL
#   HTTP_PORT=8080                   - health/docs HTTP port
#   REGISTRY_BOOTSTRAP_FILE=config\bootstrap.json
#   RUN_MIGRATIONS=false   - set true on first run or after schema changes

$env:DATABASE_URL = if ($env:DATABASE_URL) { $env:DATABASE_URL } else { "postgres://morezero:morezero_secret@localhost:5432/morezero?sslmode=disable" }
$env:REGISTRY_BOOTSTRAP_FILE = if ($env:REGISTRY_BOOTSTRAP_FILE) { $env:REGISTRY_BOOTSTRAP_FILE } else { "config\bootstrap.json" }
$env:COMMS_URL = if ($env:COMMS_URL) { $env:COMMS_URL } else { "nats://127.0.0.1:4222" }
$env:HTTP_PORT = if ($env:HTTP_PORT) { $env:HTTP_PORT } else { "8080" }
$env:RUN_MIGRATIONS = if ($env:RUN_MIGRATIONS) { $env:RUN_MIGRATIONS } else { "false" }

& "$PSScriptRoot\capabilities-registry.exe"
