# Provider Onboarding Templates Regression - 2026-06-27

## Scope

This regression verifies packaged provider onboarding templates for OpenAI, Grok, and Anthropic.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Template list API | Verified | `GET /api/admin/provider-templates` |
| Template install API | Verified | `POST /api/admin/provider-templates/:id/install` |
| OpenAI template | Verified | Provider, models, bindings persisted |
| Grok template | Verified | Provider, models, bindings persisted |
| Anthropic template | Verified | Provider, models, bindings persisted |
| Catalog availability | Verified | Installed template models are active in model catalog |
| Audit | Verified | `provider_template.install` events recorded |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/provider-onboarding-templates.sh
```

## Test Result

```text
Total: 19
Passed: 19
Failed: 0
```

## Current Limitations

- OpenAI and Grok use the OpenAI-compatible adapter path.
- Anthropic currently has catalog/credential/onboarding support; a native Anthropic Messages adapter remains a follow-up enhancement.
- Real upstream smoke requires valid provider API keys.

