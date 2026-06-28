# Anthropic Native Adapter Regression - 2026-06-27

## Scope

This regression verifies native Anthropic Messages API support behind the OpenAI-compatible AI Aggregator gateway.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Native adapter | Verified | `backend/internal/provider/anthropic.go` |
| Router support | Verified | `adapter_type=anthropic` |
| Provider template | Verified | Anthropic template installs `adapter_type=anthropic` |
| Request mapping | Verified | `/v1/chat/completions` maps to Anthropic `/v1/messages` |
| Headers | Verified | `x-api-key`, `anthropic-version: 2023-06-01` |
| System message mapping | Verified | OpenAI `system` message maps to Anthropic `system` field |
| Response mapping | Verified | Anthropic message response maps back to OpenAI-compatible chat response |
| Usage logging | Verified | Anthropic `input_tokens` / `output_tokens` persisted as prompt/completion tokens |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/anthropic-native-adapter.sh
```

## Test Result

```text
Total: 13
Passed: 13
Failed: 0
```

## Current Limitations

- Streaming is currently synthesized from a non-streaming native call for API compatibility.
- Tool-use mapping is not implemented yet.
- Real upstream smoke requires a valid Anthropic API key.

