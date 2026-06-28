# Invoice Status / Export Regression - 2026-06-27

## Scope

本轮增强 Invoice / PO / postpaid billing 基线：

- Invoice status workflow
- Invoice CSV export
- Invoice PDF export
- Admin Workspaces invoice actions

## Implemented

- 新增 Admin API：
  - `GET /api/admin/invoices/export`
  - `GET /api/admin/invoices/:id/pdf`
  - `PUT /api/admin/invoices/:id/status`
- `status` 支持：
  - `draft`
  - `issued`
  - `paid`
  - `void`
- 非法 status 返回稳定 `invalid_request`。
- status 更新写入 `audit_logs`：
  - `invoice.status_update`
- PDF export 写入 `audit_logs`：
  - `invoice.pdf_export`
- Admin Workspaces 的 Invoices / PO 面板新增：
  - Export CSV
  - Download PDF
  - Issue
  - Mark Paid
  - Void

## Regression Script

```text
scripts/regression/invoice-status-export.sh
```

覆盖内容：

| Case | Result |
|---|---:|
| Apply invoice migration | Pass |
| Create postpaid organization/workspace | Pass |
| Seed billing transactions | Pass |
| Create invoice draft | Pass |
| Update invoice to issued | Pass |
| Update invoice to paid | Pass |
| Reject invalid status | Pass |
| Export invoices CSV | Pass |
| Export invoice PDF | Pass |
| PDF contains invoice number | Pass |
| DB status persistence | Pass |
| Audit persistence | Pass |
| PDF export audit persistence | Pass |

执行结果：

```text
18 pass / 0 fail
```

## Remaining Work

- Monthly statement automation。
- Regional tax rules。
- Project invoice grouping。
