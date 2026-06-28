# Scoped BYOK Regression - 2026-06-27

## Scope

This regression verifies user/workspace/platform scoped provider credentials for BYOK.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Scoped provider key schema | Verified | `provider_keys.scope`, `user_id`, `workspace_id`, `key_mask` |
| Scoped Admin API | Verified | `POST /api/admin/providers/:id/keys` accepts `platform`, `user`, `workspace` scope |
| Masked list response | Verified | `GET /api/admin/providers/:id/keys` returns masks and never plaintext |
| Runtime priority | Verified | Workspace key takes priority over user key, then platform key |
| Admin UI | Verified by build | Provider Credentials form supports scope/user/workspace fields |
| Audit | Verified | `provider_key.create` events recorded |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/scoped-byok.sh
```

## Test Result

```text
Total: 19
Passed: 19
Failed: 0
```

## Remaining Enhancements

- End-user self-service BYOK page has been added later; see `46_USER_BYOK_SELF_SERVICE_REGRESSION_2026-06-27.md`.
- Add credential validation button against each real upstream provider.
- Add request log columns for `credential_scope` / provider key id without exposing secrets.
