# UI Page Specification

## User Console

### Dashboard

Purpose: show high-level usage and reliability.

Widgets:

- Total requests
- Total tokens
- Total spend
- Error rate
- Average latency
- Top models by usage
- Recent errors

### API Keys

Functions:

- Create key
- Optionally bind key to workspace_id
- Show key once
- Revoke key
- View created date and last used date
- Future: scope, quota, rotation

### Models

Functions:

- List models
- Show price
- Show context window
- Show capabilities
- Show provider availability
- Open model detail page

### Playground

Functions:

- Select model
- Enter system/user messages
- Toggle streaming if supported
- Send request
- Show response
- Show token usage
- Show estimated cost
- Show request_id

### Billing

Functions:

- Show balance
- Show transactions
- Show usage charges
- Show credit grants
- Future: invoice and payment method

### Request Logs

Functions:

- List requests
- Filter by model/provider/status/date
- Open detail drawer
- Show request_id
- Show latency
- Show token usage
- Show charged cost
- Show upstream cost if permitted
- Show error details
- Show fallback count

### Provider Status

Functions:

- Show provider list
- Show health status
- Show last check
- Show latency
- Show error message
- Show manual check button for admin

Current implementation:

```text
Path: /admin/provider-status
File: frontend/src/pages/Admin.tsx
API: GET /api/admin/provider-health
Manual action: POST /api/admin/providers/:id/health-check
```

Displayed fields:

```text
provider display name
provider_id
adapter_type
status
latency_ms
checked_at
error_message
manual Check button
```

### Admin Models

Current implementation:

```text
Path: /admin/models
File: frontend/src/pages/Admin.tsx
API:
  GET /api/admin/models
  GET /api/admin/models/:id
  PUT /api/admin/models/:id
```

Functions:

- List models from real Admin API
- Show model id, display name, modality, input/output price, status
- Select a model and edit display name, input price, output price, status
- Show provider bindings for the selected model

Notes:

```text
Provider create/update and model-provider binding create/update/delete are implemented in backend.
The current UI exposes model pricing/status editing and binding visibility; deeper provider/binding edit forms can be added in v0.3 admin expansion.
```

### Admin Workspaces

Current implementation:

```text
Path: /admin/workspaces
File: frontend/src/pages/Admin.tsx
API:
  GET /api/admin/organizations
  POST /api/admin/organizations
  GET /api/admin/workspaces
  POST /api/admin/workspaces
  GET /api/admin/workspaces/:id/members
  POST /api/admin/workspaces/:id/members
  GET /api/admin/workspaces/:id/usage
```

Functions:

- Create organization
- Create workspace inside first available organization
- List workspaces
- Select workspace
- View workspace usage summary
- Add workspace member by user_id
- View workspace members

Remaining v0.3 UI work:

```text
Budget / Quota configuration UI
RBAC role editor
Workspace-level request log and cost detail
```

Access:

```text
Requires admin JWT. Non-admin users receive admin access error state.
```

## Admin Console

### Admin Models

Functions:

- Create model
- Edit display name
- Edit context window
- Edit pricing
- Enable/disable model
- Delete or archive model

### Admin Providers

Functions:

- Create provider
- Edit provider type
- Edit base URL
- Update secret without displaying existing secret
- Enable/disable provider
- Trigger health check

### Admin Model Providers

Functions:

- Map model to provider
- Configure provider_model_name
- Set priority
- Enable/disable mapping
- Set streaming support

### Admin Usage / Margin

Future functions:

- Revenue
- Upstream cost
- Gross margin
- Usage by model
- Usage by provider

## Marketplace Pages

### Model Catalog

Functions:

- Browse models
- Filter by capability
- Sort by price/context/latency/quality score when available

### Model Detail

Functions:

- Description
- Use cases
- Pricing
- Context window
- Supported modalities
- Provider availability
- Example API call

### Model Comparison

Functions:

- Compare 2-5 models
- Price
- Context
- Provider
- Capabilities
- Latency and quality score when available

## Workflow / Agent Pages — Future

### Workflow Builder

- Visual steps
- Model node
- Tool node
- RAG node
- Condition node
- Human approval node

### Agent Runs

- Trace viewer
- Step cost
- Tool calls
- Final output
- Evaluation score
