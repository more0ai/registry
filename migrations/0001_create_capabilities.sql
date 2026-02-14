-- Migration: 0001_create_capabilities
-- Description: Core capabilities table (logical identity)

CREATE TABLE IF NOT EXISTS capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Identity
    app TEXT NOT NULL,
    name TEXT NOT NULL,

    -- Metadata
    description TEXT,
    tags TEXT[] DEFAULT '{}',

    -- Status
    status TEXT NOT NULL DEFAULT 'Active',
    object TEXT NOT NULL DEFAULT 'capability',

    -- Revision tracking (for etag computation)
    revision INTEGER NOT NULL DEFAULT 1,

    -- Audit fields
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified_by UUID NOT NULL,

    -- Extension fields
    config JSONB DEFAULT '{}',
    ext JSONB DEFAULT '{}',

    -- Constraints
    CONSTRAINT uq_capabilities_app_name UNIQUE (app, name)
);

CREATE INDEX IF NOT EXISTS idx_capabilities_app ON capabilities(app);
CREATE INDEX IF NOT EXISTS idx_capabilities_status ON capabilities(status);
CREATE INDEX IF NOT EXISTS idx_capabilities_tags ON capabilities USING GIN(tags);

COMMENT ON TABLE capabilities IS 'Registry of capability logical identities';
COMMENT ON COLUMN capabilities.app IS 'Application namespace (e.g., more0)';
COMMENT ON COLUMN capabilities.name IS 'Capability name within app (e.g., doc.ingest)';
COMMENT ON COLUMN capabilities.revision IS 'Monotonic revision for etag computation';
