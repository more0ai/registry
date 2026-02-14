-- Migration: 0005_create_capability_tenant_rules
-- Description: Tenant-specific capability access rules

CREATE TABLE IF NOT EXISTS capability_tenant_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Reference to capability
    capability_id UUID NOT NULL REFERENCES capabilities(id) ON DELETE CASCADE,

    -- Tenant/scope
    tenant_id UUID,
    env TEXT,
    aud TEXT,

    -- Rules
    rule_type TEXT NOT NULL DEFAULT 'allow',
    allowed_majors INTEGER[] DEFAULT '{}',
    denied_majors INTEGER[] DEFAULT '{}',

    -- Feature gating
    required_features TEXT[] DEFAULT '{}',

    -- Priority (lower = higher priority)
    priority INTEGER NOT NULL DEFAULT 100,

    -- Standard fields
    object TEXT NOT NULL DEFAULT 'capability_tenant_rule',
    status TEXT NOT NULL DEFAULT 'Active',
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL,
    modified TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    modified_by UUID NOT NULL,
    config JSONB DEFAULT '{}',
    ext JSONB DEFAULT '{}',

    -- Constraints
    CONSTRAINT chk_rule_type CHECK (rule_type IN ('allow', 'deny'))
);

CREATE INDEX IF NOT EXISTS idx_capability_tenant_rules_capability_id ON capability_tenant_rules(capability_id);
CREATE INDEX IF NOT EXISTS idx_capability_tenant_rules_tenant_id ON capability_tenant_rules(tenant_id);
CREATE INDEX IF NOT EXISTS idx_capability_tenant_rules_priority ON capability_tenant_rules(priority);

COMMENT ON TABLE capability_tenant_rules IS 'Tenant-specific capability access rules';
COMMENT ON COLUMN capability_tenant_rules.priority IS 'Lower number = higher priority';
