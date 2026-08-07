# OpenAI

Exports FOCUS 1.2 records from the OpenAI [Usage and Costs Admin API](https://platform.openai.com/docs/api-reference/usage).
OpenAI reports token usage and cost at **different grains**, so this adapter
emits two kinds of record:

- **Token records** - one per (day, model, token bucket) from
  `usage/completions`, carrying `ConsumedQuantity` (tokens) but **no cost**.
- **Cost records** - one per (day, cost line item) from `costs`, carrying real
  billed `BilledCost` in dollars but no token quantity.

They are not joined because this adapter requests **`group_by[]=line_item`** for
cost (a coarse product bucket like `"gpt-4o, input"` or `"Image models"`). The
cost endpoint can also group by `project_id` and `api_key_id`, but not by model,
so line-item cost cannot be attributed to a specific model or token bucket.

Provider name (for `--provider`): `openai`

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `OPENAI_ADMIN_KEY` | yes | An Admin API key (`sk-admin-...`) or a key with the `api.usage.read` scope. Only an org owner can create an admin key / grant the scope. |
| `OPENAI_ORG_ID` | yes | Sets `BillingAccountId` on every record (mandatory FOCUS column; OpenAI's usage/cost endpoints don't return the org id). |

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
| `input_uncached_tokens` (fallback `input_tokens - input_cached_tokens - input_cache_write_tokens`) | `input` |
| `input_cached_tokens` | `cache_read` |
| `input_cache_write_tokens` | `cache_creation` |
| `output_tokens` | `output` |

`input_audio_tokens` / `output_audio_tokens` are a subset of the input / output
totals; they are carried as `x_InputAudioTokens` / `x_OutputAudioTokens`
extensions on the respective records rather than as separate buckets, so audio
usage stays distinguishable without double-counting.

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
- **`--start` is required, `--end` optional.** The API requires `start_time`;
  the CLI rejects an open-ended window for this provider. `--end` maps to
  `end_time` and is only sent when given.
- **Granularity is daily** (`bucket_width=1d`).
- **Cache-write tokens** (`input_cache_write_tokens`, billed at a premium on
  newer models) map to the `cache_creation` bucket; cached reads map to
  `cache_read`.
