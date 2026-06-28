-- AI Aggregator Platform - File Governance Settings
-- Adds runtime-configurable MIME allowlist for local file uploads.

INSERT INTO system_settings (key, value, value_type, description)
VALUES (
    'allowed_upload_mime_types',
    'text/plain,application/json,application/pdf,text/csv,image/png,image/jpeg,image/webp',
    'csv',
    'Comma-separated upload MIME allowlist. Use * to allow all MIME types.'
)
ON CONFLICT (key) DO NOTHING;
