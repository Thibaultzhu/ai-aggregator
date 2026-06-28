-- AI Aggregator Platform - v1.1 File Upload Foundation
-- Stores OpenAI-compatible file metadata while file bytes live in the configured
-- local development storage directory.

CREATE TABLE IF NOT EXISTS uploaded_files (
    id              VARCHAR(80) PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id      UUID NULL REFERENCES api_keys(id) ON DELETE SET NULL,
    workspace_id    UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    filename        TEXT NOT NULL,
    purpose         TEXT NOT NULL DEFAULT 'assistants',
    bytes           BIGINT NOT NULL DEFAULT 0,
    mime_type       TEXT NULL,
    storage_path    TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'uploaded'
        CHECK (status IN ('uploaded', 'deleted')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_uploaded_files_user_created
    ON uploaded_files(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_uploaded_files_workspace_created
    ON uploaded_files(workspace_id, created_at DESC);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_uploaded_files_updated') THEN
        CREATE TRIGGER trg_uploaded_files_updated
            BEFORE UPDATE ON uploaded_files
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
