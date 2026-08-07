# OpenAI

Exports FOCUS 1.2 records from the OpenAI [Usage and Costs Admin API](https://platform.openai.com/docs/api-reference/usage).
OpenAI reports token usage and cost at **different grains**, so this adapter
emits two kinds of record:

- **Token records** - one per (day, model, token bucket) from
  `usage/completions`, carrying `ConsumedQuantity` (tokens) but **no cost**.
- **Cost records** - one per (day, cost line item) from `costs`, carrying real
  billed `BilledCost` in dollars but no token quantity.

They are not joined because the cost endpoint only groups by `line_item` (a
coarse product bucket like `"gpt-4o, input"` or `"Image models"`), not by model.

Provider name (for `--provider`): `openai`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `OPENAI_ADMIN_KEY` | yes | An Admin API key (`sk-admin-...`) or a key with the `api.usage.read` scope. Only an org owner can create an admin key / grant the scope. |
| `OPENAI_ORG_ID` | no | Sets `BillingAccountId` on every record (OpenAI's usage/cost endpoints don't return the org id). |

The key is sent as `Authorization: Bearer <key>`.

## Credential setup

The Usage and Cost endpoints require the `api.usage.read` scope. Create an
**Admin key** in the OpenAI dashboard (Settings -> Organization -> Admin keys),
or grant an existing service-account/API key the **Usage read** role. A regular
or service-account key without that scope returns `403 Missing scopes:
api.usage.read`.

```bash
export OPENAI_ADMIN_KEY=sk-admin-...
```

## API endpoints used

Base URL: `https://api.openai.com`

| Endpoint | Purpose |
| --- | --- |
| `GET /v1/organization/usage/completions` | Token counts per day, grouped by `model`. |
| `GET /v1/organization/costs` | Billed cost per day, grouped by `line_item`. |

Both take `start_time` (unix seconds, required) / `end_time`, `bucket_width=1d`,
and are followed to completion via `has_more` / `next_page`.

## FOCUS mapping

Token buckets from the usage report:

| OpenAI usage field | FOCUS bucket (`SkuMeter`) |
| --- | --- |
| `input_uncached_tokens` (or `input_tokens - input_cached_tokens`) | `input` |
| `input_cached_tokens` | `cache_read` |
| `output_tokens` | `output` |

| FOCUS column | Source |
| --- | --- |
| `BilledCost` (cost records only) | `costs` `amount.value` (already in dollars) |
| `BillingCurrency` | `amount.currency`, uppercased (`usd` -> `USD`) |
| `ConsumedQuantity` / `ConsumedUnit` (token records only) | usage token count / `tokens` |
| `ServiceName`, `SkuId` (token) | model (e.g. `gpt-4o`) |
| `ServiceName`, `SkuMeter` (cost) | line item (e.g. `gpt-4o, input`) |
| `Provider`, `Publisher`, `InvoiceIssuer` | `OpenAI` |
| `ServiceCategory` / `ServiceSubcategory` | `AI and Machine Learning` / `Generative AI` |
| `SkuPriceId` (token) | `model\|bucket` |
| `x_TokenType` | token bucket |
| `x_ModelRequests` | `num_model_requests` |
| `BillingAccountId` | `OPENAI_ORG_ID`, when set |

## Example

```bash
export OPENAI_ADMIN_KEY=sk-admin-...
focus-exporter --provider openai --start 2026-07-01 --end 2026-08-01 --format json
```

## Notes and limitations

- **Cost and token detail are separate grains.** OpenAI's cost API groups only by
  `line_item`, so we cannot attribute dollar cost to a specific model or token
  bucket the way the Anthropic adapter does. Token records carry usage without
  cost; cost records carry dollars without tokens.
- **A window is required.** The API requires `start_time`, so always pass
  `--start`/`--end` (or `--month`).
- **Granularity is daily** (`bucket_width=1d`).
- **No cache-creation bucket.** OpenAI's prompt caching is a discounted read; the
  adapter maps `input_cached_tokens` to `cache_read` only.
