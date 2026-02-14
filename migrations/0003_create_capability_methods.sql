-- Migration: 0003_create_capability_methods
-- Description: Methods available on each capability version

CREATE TABLE IF NOT EXISTS capability_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Reference to version
    version_id UUID NOT NULL REFERENCES capability_versions(id) ON DELETE CASCADE,

    -- Method identity
    name TEXT NOT NULL,
    description TEXT,

    -- Schemas (JSON Schema format)
    input_schema JSONB DEFAULT '{}',
    output_schema JSONB DEFAULT '{}',

    -- Method metadata
    tags TEXT[] DEFAULT '{}',
    policies JSONB DEFAULT '{}',
    examples JSONB DEFAULT '[]',

    -- Invoke modes supported
    modes TEXT[] DEFAULT '{sync}',

    -- Standard fields
    object TEXT NOT NULL DEFAULT 'capability_method',
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified_by UUID NOT NULL,
    config JSONB DEFAULT '{}',
    ext JSONB DEFAULT '{}',

    -- Constraints
    CONSTRAINT uq_capability_method UNIQUE (version_id, name)
);

CREATE INDEX IF NOT EXISTS idx_capability_methods_version_id ON capability_methods(version_id);
CREATE INDEX IF NOT EXISTS idx_capability_methods_name ON capability_methods(name);
CREATE INDEX IF NOT EXISTS idx_capability_methods_tags ON capability_methods USING GIN(tags);

COMMENT ON TABLE capability_methods IS 'Methods available on capability versions';
COMMENT ON COLUMN capability_methods.modes IS 'Supported invocation modes: sync, async, stream';
