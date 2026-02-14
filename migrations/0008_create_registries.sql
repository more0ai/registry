-- Migration: 0008_create_registries
-- Description: Registries table - registry endpoint/config per alias (alias-based resolution)

CREATE TABLE IF NOT EXISTS registries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Alias (e.g. main, partner) - used in canonical identity cap:@alias/app/cap@version
    alias TEXT NOT NULL,

    -- Optional: NATS URL for this registry (when forwarding)
    nats_url TEXT,

    -- Optional: registry subject on that NATS (e.g. cap.system.registry.v1)
    registry_subject TEXT,

    -- Whether this is the default alias when client omits @alias
    is_default BOOLEAN NOT NULL DEFAULT false,

    -- Extension/config (JSON)
    config JSONB DEFAULT '{}',

    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_registries_alias UNIQUE (alias)
);

CREATE INDEX IF NOT EXISTS idx_registries_alias ON registries(alias);
CREATE INDEX IF NOT EXISTS idx_registries_is_default ON registries(is_default) WHERE is_default = true;

COMMENT ON TABLE registries IS 'Registry aliases: alias -> endpoint/config for alias-based capability resolution';
