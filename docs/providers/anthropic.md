# Anthropic

Exports one FOCUS 1.2 record per **(day, model, token bucket)** from the
Anthropic Admin API, joining the [Cost report](https://platform.claude.com/docs/en/api/admin-api/usage-cost/get-cost-report)
(real billed cost) with the [Messages usage report](https://platform.claude.com/docs/en/api/admin-api/usage-cost/get-messages-usage-report)
(token counts). Cost is the real invoiced amount, not an estimate.

Provider name (for `--provider`): `anthropic`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `ANTHROPIC_ADMIN_KEY` | yes | An Admin API key (`sk-ant-admin...`). Only an organization admin can create one. Treat it as a credential; never commit it. |
| `ANTHROPIC_ORG_ID` | no | Overrides `BillingAccountId`. Normally the org id and name are resolved automatically from `GET /v1/organizations/me`; set this only to force a different value. |

The key is sent as the `x-api-key` header, with `anthropic-version: 2023-06-01`.

## Credential setup

Admin API keys are created in the Anthropic Console under **Settings ->
Admin keys** by an organization admin. They are distinct from regular API keys
and grant read access to organization-wide usage and cost.

This adapter supports **Claude Console** organizations, which expose the Usage
and Cost Admin API. Claude Enterprise (claude.ai) organizations are **not**
supported: they use the separate Analytics API and an Analytics API key, and an
admin key will not authenticate against those endpoints.

```bash
export ANTHROPIC_ADMIN_KEY=sk-ant-admin...
```

## API endpoints used

Base URL: `https://api.anthropic.com`

| Endpoint | Purpose |
| --- | --- |
| `GET /v1/organizations/me` | Organization id and name, for `BillingAccountId` / `BillingAccountName`. |
| `GET /v1/organizations/cost_report` | Billed cost per day, grouped by `description` (carries `model`, `token_type`, `amount`, `service_tier`, `context_window`). |
| `GET /v1/organizations/usage_report/messages` | Token counts per day, grouped by `model`. |

Both use `bucket_width=1d`, are bounded by `starting_at`/`ending_at`, and are
followed to completion via `has_more` / `next_page`.

## FOCUS mapping

Cost `amount` is a decimal string in **lowest currency units (cents)**; it is
divided by 100 into major units. `token_type` maps to a FOCUS token bucket, and
the two cache-creation token types are summed into one `cache_creation` bucket:

| Anthropic `token_type` | FOCUS bucket (`SkuMeter`) |
| --- | --- |
| `uncached_input_tokens` | `input` |
| `output_tokens` | `output` |
| `cache_read_input_tokens` | `cache_read` |
| `cache_creation.ephemeral_1h_input_tokens` + `cache_creation.ephemeral_5m_input_tokens` | `cache_creation` (summed) |

| FOCUS column | Source |
| --- | --- |
| `BilledCost`, `EffectiveCost`, `ListCost` | cost-report `amount` / 100 |
| `BillingCurrency` | `USD` |
| `ChargePeriodStart` / `ChargePeriodEnd` | the day bucket `[start, start+24h)` |
| `ConsumedQuantity` / `ConsumedUnit` | usage-report token count for that (model, bucket) / `tokens` |
| `ServiceName`, `SkuId` | model (e.g. `claude-opus-4-6`) |
| `Provider` | `Anthropic` |
| `ServiceCategory` / `ServiceSubcategory` | `AI and Machine Learning` / `Generative AI` |
| `ResourceType` | `Model` |
| `SkuMeter` | token bucket |
| `SkuPriceId` | `model\|bucket` |
| `SkuPriceDetails` | JSON of `token_type` / `service_tier` / `context_window` |
| `ContractedCost` | same as `EffectiveCost` (no separate negotiated rate) |
| `Publisher`, `InvoiceIssuer` | `Anthropic` |
| `ChargeFrequency` | `Usage-Based` |
| `PricingCategory`, `PricingCurrency` | `Standard`, `USD` |
| `PricingQuantity` / `PricingUnit` | tokens / 1e6, `1M tokens` |
| `ListUnitPrice` / `ContractedUnitPrice` | cost / (tokens / 1e6), the per-MTok rate |
| `BillingAccountId` / `BillingAccountName` | org id / name from `GET /v1/organizations/me` (override the id with `ANTHROPIC_ORG_ID`) |
| `x_TokenType` | token bucket |
| `x_ServiceTier` / `x_ContextWindow` | cost-row dimensions, when present |

## Example

```bash
export ANTHROPIC_ADMIN_KEY=sk-ant-admin...
focus-exporter --provider anthropic --start 2025-08-01 --end 2025-08-31 --format json
```

Representative record:

```json
{
  "BilledCost": "3",
  "EffectiveCost": "3",
  "ListCost": "3",
  "BillingCurrency": "USD",
  "ChargePeriodStart": "2025-08-01T00:00:00Z",
  "ChargePeriodEnd": "2025-08-02T00:00:00Z",
  "Provider": "Anthropic",
  "ServiceName": "claude-opus-4-6",
  "ServiceCategory": "AI and Machine Learning",
  "ServiceSubcategory": "Generative AI",
  "ChargeCategory": "Usage",
  "ResourceType": "Model",
  "ConsumedQuantity": "1500",
  "ConsumedUnit": "tokens",
  "SkuId": "claude-opus-4-6",
  "SkuMeter": "input",
  "SkuPriceId": "claude-opus-4-6|input",
  "x_TokenType": "input",
  "x_ServiceTier": "standard",
  "x_ContextWindow": "0-200k"
}
```

## Notes and limitations

- **Granularity is daily.** The cost report only supports `bucket_width=1d`, so
  `ChargePeriod` is a calendar day.
- **A window is required.** Unlike PlanetScale, the Anthropic API requires an
  explicit `starting_at`; always pass `--start`/`--end` (or `--month`).
- **Cost is rolled up per (model, token bucket) per day.** When a model's cost
  splits across service tiers or context windows within a day, the amounts are
  summed and one representative tier/context window is reported in the
  extensions.
- **Non-token costs** (web search, code execution, session usage) are surfaced
  as their own records keyed by `cost_type`, with the provider's description and
  no token quantity.
- **Cost without matching usage** (or usage without cost) is still emitted, not
  dropped. In particular, Priority Tier (`service_tier=priority`) usage is
  reported by the usage API but **not** by the cost API, so those records are
  emitted with `BilledCost` unset. An unset cost is not a zero-cost result.
- **`inference_geo` is not emitted.** The cost report cannot group by it and the
  usage report is grouped by model, so the API returns it as null; the column is
  omitted rather than always-empty.
