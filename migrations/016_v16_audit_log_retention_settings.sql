-- v16: Audit log retention runtime setting

INSERT INTO system_settings (key, value, value_type, description)
VALUES (
    'audit_log_retention_days',
    '0',
    'int',
    'Days to retain audit log events before retention cleanup. 0 disables audit log retention cleanup.'
)
ON CONFLICT (key) DO NOTHING;
