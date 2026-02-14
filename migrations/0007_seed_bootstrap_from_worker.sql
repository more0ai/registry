-- Migration: 0007_seed_bootstrap_from_worker
-- Description: Seeds database with all capabilities from apps/capabilities-worker/bootstrap.json.
--              Includes system, tool, workflow, agent, model, and prompt capabilities.
--              Idempotent: uses ON CONFLICT DO NOTHING / DO UPDATE and WHERE NOT EXISTS.

-- System user UUID for created_by/modified_by
DO $$
DECLARE
    v_system_user UUID := '00000000-0000-0000-0000-000000000001';
    v_cap_id UUID;
    v_ver_id UUID;
BEGIN

    -- ============================================================
    -- 1. TOOL CAPABILITIES
    -- ============================================================

    -- tool.search
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('tool', 'search', 'Search tool capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'invoke', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'validate', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- tool.calculator
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('tool', 'calculator', 'Calculator tool capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'invoke', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- tool.codegen
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('tool', 'codegen', 'Code generation tool capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'invoke', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'stream', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- ============================================================
    -- 2. WORKFLOW CAPABILITIES
    -- ============================================================

    -- workflow.ingest
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('workflow', 'ingest', 'Document/workflow ingest workflow', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'start', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'status', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'cancel', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- workflow.approval
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('workflow', 'approval', 'Approval workflow capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'start', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'approve', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'reject', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'status', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- ============================================================
    -- 3. AGENT CAPABILITIES
    -- ============================================================

    -- agent.assistant
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('agent', 'assistant', 'Assistant agent capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'chat', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'stream', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'tools', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- agent.summarizer
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('agent', 'summarizer', 'Summarizer agent capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'summarize', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'stream', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- ============================================================
    -- 4. MODEL CAPABILITIES
    -- ============================================================

    -- model.completion
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('model', 'completion', 'LLM completion model capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'complete', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'stream', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'list', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- model.embedding
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('model', 'embedding', 'Embedding model capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'embed', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'list', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- ============================================================
    -- 5. PROMPT CAPABILITIES
    -- ============================================================

    -- prompt.template
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('prompt', 'template', 'Prompt template capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'render', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'list', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'validate', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    -- prompt.chat
    INSERT INTO capabilities (app, name, description, tags, status, created_by, modified_by)
    VALUES ('prompt', 'chat', 'Chat prompt / conversation prompt capability', '{}', 'Active', v_system_user, v_system_user)
    ON CONFLICT (app, name) DO UPDATE SET
        description = COALESCE(EXCLUDED.description, capabilities.description),
        modified = NOW(), modified_by = EXCLUDED.modified_by
    RETURNING id INTO v_cap_id;

    INSERT INTO capability_versions (capability_id, major, minor, patch, status, created_by, modified_by)
    SELECT v_cap_id, 1, 0, 0, 'active', v_system_user, v_system_user
    WHERE NOT EXISTS (
        SELECT 1 FROM capability_versions
        WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL
    );

    SELECT id INTO v_ver_id FROM capability_versions
    WHERE capability_id = v_cap_id AND major = 1 AND minor = 0 AND patch = 0 AND prerelease IS NULL LIMIT 1;

    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'render', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'list', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;
    INSERT INTO capability_methods (version_id, name, created_by, modified_by)
    VALUES (v_ver_id, 'describe', v_system_user, v_system_user) ON CONFLICT (version_id, name) DO NOTHING;

    INSERT INTO capability_defaults (capability_id, default_major, env, created_by, modified_by)
    VALUES (v_cap_id, 1, 'production', v_system_user, v_system_user)
    ON CONFLICT (capability_id, env) DO NOTHING;

    RAISE NOTICE '0007_seed_bootstrap_from_worker - seeded 11 non-system capabilities (tools, workflows, agents, models, prompts)';
END $$;
