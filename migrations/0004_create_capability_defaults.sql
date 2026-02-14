-- Migration: 0004_create_capability_defaults
-- Description: Default major version per capability per environment

CREATE TABLE IF NOT EXISTS capability_defaults (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Reference to capability
    capability_id UUID NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,

    -- Default configuration
    default_major INTEGER NOT NULL,
    env TEXT NOT NULL DEFAULT 'production',

    -- Standard fields
    object TEXT NOT NULL DEFAULT 'capability_default',
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified_by UUID NOT NULL,
    config JSONB DEFAULT '{}',
    ext JSONB DEFAULT '{}',

    -- Constraints
    CONSTRAINT uq_capability_default_env UNIQUE (capability_id, env)
);

CREATE INDEX IF NOT EXISTS idx_capability_defaults_capability_id ON capability_defaults(capability_id);

COMMENT ON TABLE capability_defaults IS 'Default major version per capability per environment';
COMMENT ON COLUMN capability_defaults.env IS 'Environment: production, staging, development';
