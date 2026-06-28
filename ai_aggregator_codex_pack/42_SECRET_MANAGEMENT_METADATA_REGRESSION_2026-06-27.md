# Secret Management Metadata Regression - 2026-06-27

## Scope

This regression verifies production-oriented secret metadata for provider and tool credentials.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Provider key seal metadata | Verified | `provider_keys.seal_version = local:v1` |
| Provider key usage tracking | Verified | `last_used_at` and `last_used_scope` update when routing uses the key |
| Provider key revoke metadata | Verified | `revoked_at` set on revoke |
| Safe Admin list metadata | Verified | Provider key list returns mask, seal version, last-used metadata, never plaintext |
| Tool credential seal metadata | Verified | `tool_credentials.seal_version`, `rotated_at`, `revoked_at` columns |
| Mock/local parity | Verified | Mock provider routes also touch scoped provider key metadata for local regression |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/secret-management-metadata.sh
```

## Test Result

```text
Total: 19
Passed: 19
Failed: 0
```

## Remaining Enhancements

- Add KMS/Vault envelope provider implementation beyond `local:v1`.
- Add credential rotation API and UI.
- Add credential usage audit event stream beyond last-used metadata.

