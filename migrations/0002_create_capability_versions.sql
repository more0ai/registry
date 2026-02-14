-- Migration: 0002_create_capability_versions
-- Description: Versioned capability implementations

CREATE TABLE IF NOT EXISTS capability_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Reference to capability
    capability_id UUID NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,

    -- SemVer components
    major INTEGER NOT NULL,
    minor INTEGER NOT NULL DEFAULT 0,
    patch INTEGER NOT NULL DEFAULT 0,
    prerelease TEXT,
    build_metadata TEXT,

    -- Computed full version string for display
    version_string TEXT GENERATED ALWAYS AS (
        major::TEXT || '.' || minor::TEXT || '.' || patch::TEXT ||
        COALESCE('-' || prerelease, '') ||
        COALESCE('+' || build_metadata, '')
    ) STORED,

    -- Status
    status TEXT NOT NULL DEFAULT 'active',
    deprecation_reason TEXT,
    deprecated_at TIMESTAMP WITH TIME ZONE,
    disabled_at TIMESTAMP WITH TIME ZONE,

    -- Metadata
    description TEXT,
    changelog TEXT,
    metadata JSONB DEFAULT '{}',

    -- Standard fields
    object TEXT NOT NULL DEFAULT 'capability_version',
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified_by UUID NOT NULL,
    config JSONB DEFAULT '{}',
    ext JSONB DEFAULT '{}',

    -- Constraints
    CONSTRAINT uq_capability_version UNIQUE (capability_id, major, minor, patch, prerelease),
    CONSTRAINT chk_version_status CHECK (status IN ('active', 'deprecated', 'disabled'))
);

CREATE INDEX IF NOT EXISTS idx_capability_versions_capability_id ON capability_versions(capability_id);
CREATE INDEX IF NOT EXISTS idx_capability_versions_major ON capability_versions(capability_id, major);
CREATE INDEX IF NOT EXISTS idx_capability_versions_status ON capability_versions(status);
CREATE INDEX IF NOT EXISTS idx_capability_versions_semver ON capability_versions(capability_id, major, minor, patch);

COMMENT ON TABLE capability_versions IS 'Versioned implementations of capabilities';
COMMENT ON COLUMN capability_versions.status IS 'active=usable, deprecated=warning, disabled=blocked';
