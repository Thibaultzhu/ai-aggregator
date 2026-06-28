# Invoice / PO / Postpaid Regression - 2026-06-27

## Scope

This regression verifies the enterprise billing baseline for invoice draft creation, purchase order metadata, and postpaid payment terms.

## Implemented Components

| Area | Status | Evidence |
|---|---:|---|
| Organization billing terms | Verified | `organizations.payment_terms_days`, `organizations.default_po_number` |
| Invoice schema | Verified | `invoices` table, status constraint, org/workspace indexes |
| Admin invoice API | Verified | `GET /api/admin/invoices`, `POST /api/admin/invoices` |
| Invoice calculation | Verified | Usage charges summed from `billing_transactions` for org/workspace/period |
| PO defaulting | Verified | Invoice uses organization `default_po_number` when request omits `po_number` |
| Due date calculation | Verified | `period_end + payment_terms_days` |
| Audit trail | Verified | `invoice.create` audit event written |
| Admin UI | Verified by build | Admin Workspaces detail includes Invoices / PO panel |

## Test Command

```bash
BASE_URL=http://localhost:8081 bash scripts/regression/invoices-po-postpaid.sh
```

## Test Result

```text
Total: 12
Passed: 12
Failed: 0
```

## Test Cases

| Case | Objective | Expected Result | Actual Result | Status |
|---|---|---|---|---:|
| Apply migration | Ensure invoice/postpaid schema is present | Migration 024 applies idempotently | Applied successfully | Pass |
| Health check | Ensure target backend is running | `/health` returns `status=ok` | Returned ok | Pass |
| Admin registration/login | Prepare authenticated admin flow | Register, promote, login all succeed | JWT returned | Pass |
| Create postpaid org | Verify billing mode and terms API | Organization returns `billing_mode=postpaid`, `payment_terms_days=45` | Returned expected values | Pass |
| Create workspace | Verify invoice can be scoped to workspace | Workspace ID returned | Workspace created | Pass |
| Seed usage charges | Prepare billable usage | Two in-period negative usage charges and one out-of-period charge inserted | Seed completed | Pass |
| Create invoice draft | Verify subtotal/PO/due date | Total `19.75`, default PO copied, due date `2026-08-14` | Invoice `INV-202606-D1D2BB35` returned expected values | Pass |
| List invoice | Verify Admin list API | Created invoice appears in org invoice list | Found by ID | Pass |
| DB persistence | Verify invoice persisted with exact total | `invoices.total_usd = 19.75000000` | Matched | Pass |
| Audit event | Verify governance trail | One `invoice.create` event exists | Count = 1 | Pass |

## Additional Verification

```text
backend go test ./... = Pass
frontend npm run build = Pass
scripts/dev-check.sh = Pass
```

## Current Limitations

- Invoice export/PDF is not implemented yet.
- Invoice status transition workflow is basic; richer approval/send/pay lifecycle remains future work.
- Project-level invoice grouping is not implemented yet.
- Tax calculation is request-provided baseline only; regional tax rules are not implemented.

