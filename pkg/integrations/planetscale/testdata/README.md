# PlanetScale testdata

These fixtures are hand-authored to match the documented PlanetScale billing
API response shapes (invoices, invoice line-items, databases), not
live-captured (no service token available in the build env). Field names mirror
those responses: region slug/display_name/provider, invoice
billing_period_start/end, line-item metric_name/subtotal/database_id.
