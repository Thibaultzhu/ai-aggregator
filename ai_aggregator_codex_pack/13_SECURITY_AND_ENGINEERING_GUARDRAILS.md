# Security and Engineering Guardrails

## Secret Handling

- Never commit API keys.
- Never print API keys.
- Never return provider secret fields to frontend.
- Store user API keys as hashes.
- Show user API key only once at creation.
- Provider credentials should be encrypted or stored through a secret manager in production.

## Database Safety

- Use context for all DB calls.
- Use parameterized queries.
- Add indexes for user-facing list APIs.
- Keep migrations idempotent where possible.
- Add rollback notes.

## Logging Safety

- Do not log Authorization headers.
- Do not log API keys.
- Truncate request and response previews.
- Prepare for PII masking in v0.6.
- Respect retention policy once added.

## API Safety

- User endpoints must enforce ownership.
- Admin endpoints must enforce `is_admin=true`.
- Return normalized errors.
- Include request_id in errors.
- Avoid leaking provider-specific internal errors directly to users.

## Routing Safety

- Disabled models must not be served.
- Disabled providers must not be used.
- Unhealthy providers should be skipped if alternatives exist.
- Avoid infinite fallback loops.
- Track fallback_count.

## Billing Safety

- Charge only once per successful completion.
- Do not charge for failed requests unless explicitly designed.
- Record upstream cost separately from charged cost.
- Support zero-cost mock provider for tests.
- Keep balance updates transactional.

## Frontend Safety

- Do not display raw secrets.
- Mask API key after creation.
- Avoid exposing admin UI to non-admin users.
- Handle empty and error states.

## Codex Behavior Constraints

When using Codex:

1. Ask it to inspect current files before coding.
2. Ask it to avoid broad rewrites.
3. Ask it to produce small diffs.
4. Ask it to run tests after changes.
5. Ask it to summarize changed files.
6. Ask it to explain migration effects.
7. Ask it to preserve v0.1 acceptance path.
