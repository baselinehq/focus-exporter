# Confluent Cloud

Exports one FOCUS 1.2 record per cost line item from the Confluent Cloud
[Billing API](https://docs.confluent.io/cloud/current/api.html) (`GET
/billing/v1/costs`). Cost is the real billed `amount` (dollars, after discounts),
so it maps cleanly onto native FOCUS columns with a real `ResourceId`.

Provider name (for `--provider`): `confluent`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `CONFLUENT_CLOUD_API_KEY` | yes | A **Cloud** (org-scoped) API key id - not a cluster/resource key. |
| `CONFLUENT_CLOUD_API_SECRET` | yes | The Cloud API key secret. Treat as a credential; never commit it. |
| `CONFLUENT_ORG_ID` | no | Sets `BillingAccountId` (the billing API doesn't return the org id). |

The key/secret are sent as HTTP Basic auth (`Authorization: Basic
base64(key:secret)`), with the key id as the username.

## Credential setup

Create a **Cloud API key** (Administrative -> not tied to a cluster) with an
org-level billing role (OrganizationAdmin or BillingAdmin):

```bash
confluent api-key create --resource cloud
export CONFLUENT_CLOUD_API_KEY=...
export CONFLUENT_CLOUD_API_SECRET=...
```

A resource-scoped key, or a key without a billing role, returns 403.

## API endpoints used

Base URL: `https://api.confluent.cloud`

| Endpoint | Purpose |
| --- | --- |
| `GET /billing/v1/costs` | Cost line items per day. Requires `start_date` (inclusive) and `end_date` (exclusive), both `YYYY-MM-DD` UTC. |

Paginated by `metadata.next` (a full URL with an embedded `page_token`), followed
to completion.

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost` / `EffectiveCost` / `ListCost` / `ContractedCost` | `amount` (final amount after discounts, dollars) |
| `BillingCurrency` / `PricingCurrency` | `USD` (the API returns no currency field; Confluent bills in USD) |
| `ChargePeriodStart` / `ChargePeriodEnd` | `start_date` / `end_date` |
| `ChargeCategory` | `Usage`, or `Credit` for a negative `amount` / `PROMO_CREDIT` line |
| `ConsumedQuantity` / `ConsumedUnit`, `PricingQuantity` / `PricingUnit` | `quantity` / `unit` |
| `ListUnitPrice` / `ContractedUnitPrice` | `price` (per-unit dollars) |
| `ServiceName` | `Confluent Cloud` |
| `Provider` / `Publisher` / `InvoiceIssuer` | `Confluent` |
| `ServiceCategory` / `ServiceSubcategory` | `Analytics` / `Streaming Analytics` |
| `ResourceId` / `ResourceName` / `ResourceType` | `resource.id` / `resource.display_name` / `product` |
| `SkuId` / `SkuMeter` / `SkuPriceId` | `product` / `line_type` / `product\|line_type` |
| `x_Environment` | `resource.environment.id` |
| `x_NetworkAccessType` | `network_access_type` |
| `x_OriginalAmount` / `x_DiscountAmount` | pre-discount amount / discount, when non-zero |

## Example

```bash
export CONFLUENT_CLOUD_API_KEY=... CONFLUENT_CLOUD_API_SECRET=...
focus-exporter --provider confluent --start 2026-07-01 --end 2026-08-01 --format json
```

## Notes and limitations

- **A window is required.** The API mandates `start_date`/`end_date`, so always
  pass `--start`/`--end` (or `--month`).
- **Granularity is daily.**
- **Currency is assumed USD** - the API returns no currency field.
- **Some lines have no resource** (e.g. `SUPPORT`, `PROMO_CREDIT`); those are
  emitted without `ResourceId`.
- Cost is verified against Confluent's published OpenAPI spec; live verification
  against a real key is pending.
