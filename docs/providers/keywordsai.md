# KeywordsAI

Exports FOCUS 1.2 records from the KeywordsAI (now respan.ai) request-logs API.
KeywordsAI is an LLM gateway/observability layer, so one integration captures
spend across every model and upstream provider you route through it. One record
per **(day, model, upstream provider)**, aggregated from per-request logs.

Provider name (for `--provider`): `keywordsai`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `KEYWORDSAI_API_KEY` | yes | API key, sent as `Authorization: Bearer <key>`. |
| `KEYWORDSAI_ORG_ID` | yes | Populates the mandatory `BillingAccountId` (the logs API returns no account id). |

## API endpoints used

Base URL: `https://api.keywordsai.co`

| Endpoint | Purpose |
| --- | --- |
| `GET /api/request-logs/list/?start_time&end_time&page_size` | Per-request logs over the window. The adapter follows `next` until the last page and rolls the rows up by (day, model, provider) client-side. |

There is no server-side per-model/provider/day aggregation endpoint, so the
adapter paginates the log list and aggregates locally.

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost` / `EffectiveCost` / `ListCost` / `ContractedCost` | sum of `cost` (USD) over the bucket |
| `BillingCurrency` / `PricingCurrency` | `USD` (the logs API returns no currency) |
| `ChargePeriodStart` / `ChargePeriodEnd` | the day (from `timestamp`) |
| `ConsumedQuantity` / `ConsumedUnit` | `prompt_tokens + completion_tokens` / `tokens` |
| `ServiceName`, `SkuId`, `ResourceId`/`Name` | `model` |
| `Provider` / `Publisher` / `InvoiceIssuer` | `KeywordsAI` |
| `ServiceCategory` / `ServiceSubcategory` | `AI and Machine Learning` / `Generative AI` |
| `SkuMeter` / `SkuPriceId` | `provider_id` / `model\|provider_id` |
| `x_UpstreamProvider` | `provider_id` |
| `x_PromptTokens` / `x_CompletionTokens` | token counts |
| `BillingAccountId` | `KEYWORDSAI_ORG_ID` |

## Example

```bash
export KEYWORDSAI_API_KEY=...
export KEYWORDSAI_ORG_ID=your-org
focus-exporter --provider keywordsai --start 2026-07-01 --end 2026-08-01 --format json
```

## Notes and limitations

- **A window is required.** Pass `--start`/`--end` or `--month`; the list API
  otherwise defaults to the last hour.
- **Cost is per (model, provider), not per token bucket.** The logs carry one
  `cost` per request; the adapter sums it per day/model/provider and does not
  split input vs output pricing (token counts ride as `x_` extensions).
- **Gateway credit fees are not captured (API gap).** Reported `BilledCost` is
  request spend from the logs. KeywordsAI's own platform/credit charges are not
  in the logs endpoint, so - as with OpenRouter - they are not reported here; if
  a billing/transactions API is added they map to `ChargeCategory = "Purchase"`
  (see the charge-completeness rule in AGENTS.md).
