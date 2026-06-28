# User BYOK Self-Service Regression - 2026-06-27

## Scope

本轮补齐用户侧 BYOK self-service 基线，让普通用户可以自己管理 user-scoped provider credentials。

## Implemented

- 新增用户 API：
  - `GET /api/user/providers`
  - `GET /api/user/provider-keys`
  - `POST /api/user/provider-keys`
  - `DELETE /api/user/provider-keys/:id`
- 用户只能创建 `scope=user` 的 provider key。
- 用户只能 list/revoke 自己的 user-scoped provider key。
- provider secret 创建后不返回明文，只返回 mask。
- DB 存储 sealed credential，不存 plaintext。
- 用户操作写入 audit：
  - `provider_key.user_create`
  - `provider_key.user_revoke`
- Settings 页面新增 BYOK Provider Keys 面板：
  - provider 选择
  - key name / secret / region 输入
  - masked key list
  - revoke 操作

## Regression Script

```text
scripts/regression/user-byok-self-service.sh
```

覆盖内容：

| Case | Result |
|---|---:|
| Admin creates enabled provider | Pass |
| Normal user lists enabled providers | Pass |
| Normal user creates BYOK key | Pass |
| Created key is user scoped | Pass |
| API response hides plaintext | Pass |
| DB does not contain plaintext secret | Pass |
| User lists masked BYOK keys | Pass |
| User revokes own key | Pass |
| Revoked key is inactive | Pass |
| Audit events persisted | Pass |

执行结果：

```text
13 pass / 0 fail
```

## Remaining Work

- 真实 OpenAI/Grok/Anthropic/Bailian upstream smoke with valid keys。
- Anthropic tool-use mapping。
- Anthropic native streaming refinement。
