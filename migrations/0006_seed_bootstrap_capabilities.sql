-- Migration: 0006_seed_bootstrap_capabilities
-- Description: Bootstrap system capabilities are seeded from config/bootstrap.json by the
--              application when RUN_MIGRATIONS is set (see pkg/db.SeedBootstrap).
--              This file reserves migration order; no schema changes.

SELECT 1;
