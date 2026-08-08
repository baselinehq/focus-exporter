# Helicone

Exports FOCUS 1.2 records from the Helicone request-query API. Helicone is an
LLM gateway/observability proxy, so one integration captures spend across every
model and upstream provider you route through it. One record per
**(day, model, upstream provider)**, aggregated from per-request rows.

Provider name (for `--provider`): `helicone`

This adapter uses an HTTP **POST** (the request-query API takes a JSON filter
body), unlike the GET-based adapters.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `HELICONE_API_KEY` | yes | Helicone API key (`sk-helicone-...`), sent as `Authorization: Bearer <key>`. |
| `HELICONE_ORG_ID` | yes | Populates the mandatory `BillingAccountId` (the query API returns no account id). |

## API endpoints used

Base URL: `https://api.helicone.ai`

| Endpoint | Purpose |
| --- | --- |
| `POST /v1/request/query` | Per-request rows over the window. The body carries a `created_at` filter (`gte` start, `lt` end); the adapter pages via `offset`/`limit` and rolls the rows up by (day, model, provider) client-side. Rows outside the window are also dropped client-side as a guard. |

There is no server-side per-model/provider/day aggregation endpoint, so the
adapter paginates and aggregates locally.

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost` / `EffectiveCost` / `ListCost` / `ContractedCost` | sum of `costUSD` (falling back to `cost`), in USD |
| `BillingCurrency` / `PricingCurrency` | `USD` |
| `ChargePeriodStart` / `ChargePeriodEnd` | the day (from `request_created_at`) |
| `ConsumedQuantity` / `ConsumedUnit` | `prompt_tokens + completion_tokens` / `tokens` |
| `ServiceName`, `SkuId`, `ResourceId`/`Name` | `request_model` |
| `Provider` / `Publisher` / `InvoiceIssuer` | `Helicone` |
| `ServiceCategory` / `ServiceSubcategory` | `AI and Machine Learning` / `Generative AI` |
| `SkuMeter` / `SkuPriceId` | `provider` / `request_model\|provider` |
| `x_UpstreamProvider` | `provider` (e.g. OPENAI, ANTHROPIC) |
| `x_PromptTokens` / `x_CompletionTokens` | token counts |
| `BillingAccountId` | `HELICONE_ORG_ID` |

## Example

```bash
export HELICONE_API_KEY=sk-helicone-...
export HELICONE_ORG_ID=your-org
focus-exporter --provider helicone --start 2026-07-01 --end 2026-08-01 --format json
```

## Notes and limitations

- **A window is required.** Pass `--start`/`--end` or `--month`.
- **Cost is per (model, provider), not per token bucket.** Each request carries
  one `costUSD`; the adapter sums it per day/model/provider and does not split
  input vs output pricing (token counts ride as `x_` extensions).
- **Gateway/platform fees are not captured (API gap).** Reported `BilledCost` is
  request spend from the query API; Helicone's own subscription/platform charges
  are not in it and are not fabricated (see the charge-completeness rule in
  AGENTS.md).
- The exact `request/query` filter shape and cost field (`costUSD` vs `cost`) are
  taken from the documented schema; live verification against a real key is
  pending.
