-- v15: File retention runtime setting

INSERT INTO system_settings (key, value, value_type, description)
VALUES (
    'file_retention_days',
    '0',
    'int',
    'Days to retain uploaded files before retention cleanup. 0 disables automatic retention cleanup.'
)
ON CONFLICT (key) DO NOTHING;
