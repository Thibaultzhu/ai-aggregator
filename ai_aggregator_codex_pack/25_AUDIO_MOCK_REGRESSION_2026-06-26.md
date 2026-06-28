# Audio Mock Regression Results - 2026-06-26

## 1. Scope

目标：将 `/v1/audio/transcriptions` 与 `/v1/audio/speech` 的网关层能力固化为可重复执行的本地回归，覆盖 mock provider 路由、响应形态、request_logs 与 usage_logs 持久化。

新增脚本：

```text
scripts/regression/audio-mock.sh
```

覆盖范围：

```text
Per-provider mock adapter routing
Audio transcription endpoint
Audio speech endpoint
Audio request_logs persistence
Audio usage_logs persistence
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
postgres=aag-postgres
redis=aag-redis
frontend=http://localhost:5175
MOCK_PROVIDER_MODE=false
```

脚本会 upsert 一个本地回归专用 provider/model：

```text
provider_id=mock_audio_regression
adapter_type=mock
model_id=mock-audio-regression
```

然后重启 backend，使 router 加载该 per-provider mock adapter。该方式不影响真实 Bailian provider 配置。

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/audio-mock.sh
```

## 4. Result Summary

```text
Total: 7
Passed: 7
Failed: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-AUDIO-001 | Router | Mock audio provider/model setup | provider/model/binding upserted | Pass |
| TC-REG-AUDIO-002 | Runtime | Backend reload | Health returns `status=ok` after restart | Pass |
| TC-REG-AUDIO-003 | Auth | Regression user registration | HTTP 201, JWT and user returned | Pass |
| TC-REG-AUDIO-004 | Audio STT | `/v1/audio/transcriptions` | HTTP 200, text contains mock transcription | Pass |
| TC-REG-AUDIO-005 | Audio TTS | `/v1/audio/speech` | HTTP 200, body is mock audio bytes | Pass |
| TC-REG-AUDIO-006 | Observability | request_logs | Two audio request logs persisted | Pass |
| TC-REG-AUDIO-007 | Usage | usage_logs | Two audio usage logs persisted | Pass |

## 6. Implementation Notes

Backend changes:

```text
ModelRouter now supports provider adapter_type=mock outside global MOCK_PROVIDER_MODE.
storage.RecordUsage now converts empty api_key_id to NULL, fixing JWT-authenticated usage logging.
```

Important behavior:

```text
scripts/regression/audio-mock.sh restarts the backend container to reload provider routing.
Run this script separately from other HTTP regression scripts to avoid transient connection failures during restart.
```

## 7. Remaining Gap

DashScope/Bailian real audio adapter remains intentionally marked as pending mapping:

```text
DashScopeAdapter.TranscribeAudio currently returns unsupported.
DashScopeAdapter.SynthesizeSpeech currently returns unsupported.
```

The gateway and mock adapter are now regression-covered. Real Bailian audio needs a separate adapter task using the exact Singapore/CN audio API contracts and model mappings.
