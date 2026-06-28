# DashScope Audio Chain Regression - 2026-06-27

## Scope

This regression verifies the Alibaba Cloud Bailian / DashScope audio provider path from Aggregator API to provider adapter.

Covered chain:

- Provider credential resolution from `provider_keys`
- DashScope adapter health check through `/compatible-mode/v1/models`
- TTS API: `POST /v1/audio/speech`
- ASR API: `POST /v1/audio/transcriptions`
- Native DashScope audio endpoint construction
- Bearer credential forwarding
- `request_logs` credential scope attribution
- `usage_logs` audio persistence

## Implementation Completed

- `backend/internal/provider/dashscope.go`
  - Implemented `TranscribeAudio`.
  - Implemented `SynthesizeSpeech`.
  - Added shared native URL builder for image, video, task polling, ASR, and TTS.
  - Added audio content-type fallback by requested format.
- `frontend/src/lib/api.ts`
  - Added `SpeechRequest`.
  - Added `createSpeech()` returning a `Blob`.
- `frontend/src/pages/Playground.tsx`
  - Enabled Audio tab generation.
  - Added DashScope voice/format controls.
  - Calls `/v1/audio/speech` and renders an audio player.
  - Shows correct audio curl example.
- `scripts/regression/dashscope-audio-chain.sh`
  - Adds fake DashScope upstream.
  - Creates temporary DashScope provider, model binding, and sealed provider key.
  - Verifies TTS, ASR, provider key forwarding, request logs, and usage logs.

## Test Command

```bash
source scripts/dev-env.sh && scripts/regression/dashscope-audio-chain.sh
```

## Test Results

| ID | Target | Expected | Result |
|---|---|---|---|
| TC-DASHSCOPE-AUDIO-001 | Health | Backend health returns ok | Pass |
| TC-DASHSCOPE-AUDIO-002 | Provider setup | Temporary DashScope provider/model/key are created | Pass |
| TC-DASHSCOPE-AUDIO-003 | Backend reload | Backend restarts and loads registry | Pass |
| TC-DASHSCOPE-AUDIO-004 | User auth | Test user registers and receives token | Pass |
| TC-DASHSCOPE-AUDIO-005 | Routing | Temporary audio model is visible/routable | Pass |
| TC-DASHSCOPE-AUDIO-006 | TTS | `/v1/audio/speech` returns provider audio bytes | Pass |
| TC-DASHSCOPE-AUDIO-007 | ASR | `/v1/audio/transcriptions` returns transcription text | Pass |
| TC-DASHSCOPE-AUDIO-008 | Native endpoints | Adapter calls fake DashScope models/TTS/ASR endpoints | Pass |
| TC-DASHSCOPE-AUDIO-009 | Credential forwarding | Provider key is forwarded as Bearer token | Pass |
| TC-DASHSCOPE-AUDIO-010 | Request logs | Audio request logs include `credential_scope=platform` | Pass |
| TC-DASHSCOPE-AUDIO-011 | Usage logs | Audio usage logs persist with modality `audio` | Pass |

Summary:

```text
Total: 11
Passed: 11
Failed: 0
```

## Additional Verification

```bash
source ../scripts/dev-env.sh && go test ./...
npm run build
source scripts/dev-env.sh && docker compose build backend && docker compose up -d backend
docker cp frontend/src/. aag-frontend:/app/src && docker restart aag-frontend
```

Results:

```text
go test ./... = Pass
npm run build = Pass
docker compose build backend = Pass
backend restarted = Pass
frontend restarted = Pass
```

## Known Limitations

- Playground currently exposes TTS playback. ASR is API-first through multipart upload.
- The regression uses a fake DashScope upstream to avoid real external cost and to verify deterministic paths.
- Live Bailian ASR may require object-storage file URL handoff instead of direct multipart upload depending on account/model policy. If so, add a file-to-OSS staging component before the native ASR request.
