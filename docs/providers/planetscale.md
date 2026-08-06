# PlanetScale

Exports one FOCUS 1.2 record per **(invoice month, database, billing metric)**
from the PlanetScale billing API. Cost is the real billed `subtotal` from the
invoice - not an estimate - so credits, prorations, and plan minimums are
reflected exactly as PlanetScale charged them.

Provider name (for `--provider`): `planetscale`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `PLANETSCALE_ORG` | yes | Organization slug (e.g. `your-org`). This is the `name` field of the organization, visible in the dashboard URL and in `GET /v1/organizations`. |
| `PLANETSCALE_SERVICE_TOKEN_ID` | yes | The service token's ID (the short public identifier, not the secret). |
| `PLANETSCALE_SERVICE_TOKEN` | yes | The service token secret (`pscale_tkn_...`). Treat as a credential; never commit it. |

The token is sent as `Authorization: <id>:<token>` on every request.

## Credential setup

PlanetScale [service tokens](https://api-docs.planetscale.com/reference/service-tokens)
are the recommended machine credential (they are not tied to a user account).

1. In the PlanetScale dashboard, go to **Settings -> Service tokens** for the
   organization, or use the CLI:
   ```bash
   pscale service-token create --org your-org
   ```
   This prints the token **ID** and the **token secret**. The secret is shown
   once - store it securely.
2. Grant the token the permissions it needs (see below):
   ```bash
   pscale service-token add-access <token-id> \
     read_organizations read_databases \
     --org your-org
   ```
3. Export the three variables:
   ```bash
   export PLANETSCALE_ORG=your-org
   export PLANETSCALE_SERVICE_TOKEN_ID=<token-id>
   export PLANETSCALE_SERVICE_TOKEN=pscale_tkn_...
   ```

### Minimum permissions

The exporter only reads. It needs organization-level read access to invoices
and databases:

- `read_organizations` - list the org and read its invoices and line items.
- `read_databases` - list databases (used to resolve region and plan for each
  line item).

No write, branch, or connection-string access is required. Scope the token to
read-only.

## API endpoints used

Base URL: `https://api.planetscale.com`

| Endpoint | Purpose |
| --- | --- |
| `GET /v1/organizations/{org}/invoices` | List invoices; each carries `billing_period_start` / `billing_period_end`. |
| `GET /v1/organizations/{org}/invoices/{id}/line-items` | Per-invoice line items: `metric_name`, `subtotal`, `database_id`, `database_name`. |
| `GET /v1/organizations/{org}/databases` | Resolve `region` (slug + display name + cloud) and `plan` for each database. |

All are paginated (`page`, `per_page=100`) and followed to completion. Only
invoices whose billing period overlaps the requested `--start`/`--end` window
are fetched for line items.

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost`, `EffectiveCost`, `ListCost` | line-item `subtotal` (there is no separate negotiated rate on the invoice) |
| `BillingCurrency` | `USD` |
| `ChargePeriodStart` / `ChargePeriodEnd` | invoice `billing_period_start` / `billing_period_end` |
| `BillingPeriodStart` / `BillingPeriodEnd` | same as charge period |
| `ChargeCategory` | `Usage`, or `Credit` when the line item has a negative `subtotal` (prorated discounts, refunds) |
| `ChargeDescription` | line-item `description` (falls back to `metric_name` when absent) |
| `ServiceName`, `Provider` | `PlanetScale` |
| `ServiceCategory` | `Databases` |
| `ServiceSubcategory` | `Managed Database` |
| `ResourceId` / `ResourceName` | `database_id` / `database_name` |
| `ResourceType` | `Database` |
| `RegionId` / `RegionName` | database `region.slug` / `region.display_name` (via the databases join) |
| `SkuId` | database `plan` |
| `SkuMeter` | `metric_name` |
| `SkuPriceId` | `plan\|metric_name` (or `metric_name` alone when the database is not in the join) |
| `x_InfraProvider` | database `region.provider` (`aws` / `gcp`) - the sole vendor-specific extension |

## Example

```bash
export PLANETSCALE_ORG=your-org
export PLANETSCALE_SERVICE_TOKEN_ID=xxxxxxxxxxxx
export PLANETSCALE_SERVICE_TOKEN=pscale_tkn_...

focus-exporter --provider planetscale --month 2026-07 --format json
```

Representative record:

```json
{
  "BilledCost": "30.0",
  "EffectiveCost": "30.0",
  "ListCost": "30.0",
  "BillingCurrency": "USD",
  "ChargePeriodStart": "2026-07-01T00:00:00Z",
  "ChargePeriodEnd": "2026-08-01T00:00:00Z",
  "Provider": "PlanetScale",
  "ServiceName": "PlanetScale",
  "ServiceCategory": "Databases",
  "ServiceSubcategory": "Managed Database",
  "ChargeCategory": "Usage",
  "ResourceId": "7p5sqtyldrf6",
  "ResourceName": "pscale-1",
  "ResourceType": "Database",
  "RegionId": "us-east",
  "RegionName": "AWS us-east-1",
  "SkuId": "scaler_pro",
  "SkuMeter": "PS_10_AWS_ARM",
  "ChargeDescription": "PS-10-AWS-ARM for branch 'main'",
  "SkuPriceId": "scaler_pro|PS_10_AWS_ARM",
  "x_InfraProvider": "AWS"
}
```

## Notes and limitations

- **Granularity is monthly.** The invoices API exposes only monthly line items,
  so `ChargePeriod` equals `BillingPeriod` (the calendar month). There is no
  finer charge granularity from this source.
- **Currency is USD.** PlanetScale invoices are billed in USD.
- **Credits and prorations** appear as their own line items with a negative
  `subtotal` (e.g. PlanetScale bills the full plan up front, then credits back
  the unused portion when a branch is deleted mid-period). They are surfaced
  as-is with negative `BilledCost` and `ChargeCategory = "Credit"`, and their
  `ChargeDescription` carries the provider's own text ("Prorated ... discount").
- **Unmatched databases** (a line item whose `database_id` is not in the
  databases list, e.g. a deleted database) are still emitted, with region and
  plan left empty rather than dropped.
- A single invoice that fails to fetch or has an unparseable billing period is
  logged and skipped; it never aborts the whole export.
