# OpenRouter

Exports FOCUS 1.2 records from the OpenRouter [Activity API](https://openrouter.ai/docs/api/api-reference/analytics/get-user-activity)
(`GET /api/v1/activity`). OpenRouter is an LLM gateway, so one integration
captures spend across **every model and upstream provider** you route through it.
One record per **(day, model, upstream provider)**, with real credit spend
(`usage`, in USD) as the billed cost.

Provider name (for `--provider`): `openrouter`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `OPENROUTER_MANAGEMENT_KEY` | yes | A **management** key (from openrouter.ai/settings/management-keys). A normal `sk-or-v1-` inference key returns `403 - Only management keys can perform this operation`. |
| `OPENROUTER_ORG_ID` | no | Sets `BillingAccountId` (the activity API returns no account id). |

The key is sent as `Authorization: Bearer <key>`.

## API endpoints used

Base URL: `https://openrouter.ai`

| Endpoint | Purpose |
| --- | --- |
| `GET /api/v1/activity?date=YYYY-MM-DD` | Per-model, per-provider usage and cost for a single UTC day. |

The adapter iterates the requested window day by day. **Only the last ~30
completed UTC days are available**; days outside that window return no data.

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost` / `EffectiveCost` / `ListCost` / `ContractedCost` | `usage` (credit spend, USD) |
| `BillingCurrency` / `PricingCurrency` | `USD` (credits are USD) |
| `ChargePeriodStart` / `ChargePeriodEnd` | the day |
| `ConsumedQuantity` / `ConsumedUnit` | `prompt_tokens + completion_tokens` / `tokens` |
| `ServiceName`, `SkuId`, `ResourceId`/`Name` | `model` |
| `Provider` / `Publisher` / `InvoiceIssuer` | `OpenRouter` |
| `ServiceCategory` / `ServiceSubcategory` | `AI and Machine Learning` / `Generative AI` |
| `SkuMeter` / `SkuPriceId` | upstream `provider_name` / `model\|provider_name` |
| `x_UpstreamProvider` | `provider_name` (e.g. OpenAI, Anthropic) |
| `x_PromptTokens` / `x_CompletionTokens` / `x_ReasoningTokens` | token counts |
| `x_ModelRequests` | `requests` |
| `x_ModelPermaslug` | `model_permaslug` |
| `x_ByokUsage` | `byok_usage_inference` (see below) |

## Example

```bash
export OPENROUTER_MANAGEMENT_KEY=sk-or-...
focus-exporter --provider openrouter --start 2026-07-01 --end 2026-08-01 --format json
```

## Notes and limitations

- **A window is required** and capped to the last ~30 UTC days by the API. Pass
  `--start`/`--end` (or `--month`); older days return nothing.
- **BYOK spend is kept out of `BilledCost`.** `byok_usage_inference` is spend that
  routed through your own upstream provider keys - OpenRouter did not bill it, and
  it would double-count against your direct provider bill. It is carried as
  `x_ByokUsage` for visibility only.
- **Cost is per (model, provider), not per token bucket.** OpenRouter reports one
  `usage` figure per model/provider per day, so cost is not split into input vs
  output; the token breakdown rides as `x_` extensions.
- Wait ~30 minutes past a UTC day boundary before pulling that day (late-finishing
  requests are attributed by start time). A day outside the 30-day window (or the
  current incomplete day) returns 400; the adapter logs and skips it rather than
  failing the run.
- Verified end-to-end against a live management key (real per-model, per-provider
  spend across OpenAI / Azure / Amazon Bedrock upstreams).
