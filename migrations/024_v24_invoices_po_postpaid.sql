-- v1.0 Invoice / PO / postpaid foundation.

ALTER TABLE organizations ADD COLUMN IF NOT EXISTS payment_terms_days INTEGER NOT NULL DEFAULT 30;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS default_po_number TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_number TEXT UNIQUE NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    workspace_id UUID NULL REFERENCES workspaces(id) ON DELETE SET NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'issued', 'paid', 'void')),
    po_number TEXT NOT NULL DEFAULT '',
    subtotal_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    tax_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    total_usd NUMERIC(18,8) NOT NULL DEFAULT 0,
    due_date DATE NULL,
    notes TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invoices_org_created ON invoices(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_workspace_created ON invoices(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_invoices_status_due ON invoices(status, due_date);

DROP TRIGGER IF EXISTS trg_invoices_updated ON invoices;
CREATE TRIGGER trg_invoices_updated
    BEFORE UPDATE ON invoices
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
