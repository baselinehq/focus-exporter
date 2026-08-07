# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- FOCUS compliance is now enforced: every mapped record is run through
  `focus.Validate` before export and the run fails on the first non-compliant
  row. `Validate` now also checks the mandatory columns it previously skipped -
  `BillingAccountId`, `BillingCurrency`, `ServiceName`, `Provider`,
  `ChargeDescription`, and `BillingPeriodStart`/`BillingPeriodEnd`.
- `BillingPeriodStart`/`BillingPeriodEnd` are now the calendar month containing
  the charge period (the invoice cycle) rather than a copy of the charge period.
- `OPENAI_ORG_ID`, `OPENROUTER_ORG_ID`, and `CONFLUENT_ORG_ID` are now required
  (they populate the mandatory `BillingAccountId` column and their APIs do not
  return an account id). Anthropic still resolves the account automatically from
  `GET /v1/organizations/me`.

### Added

- Confluent Cloud adapter: one FOCUS 1.2 record per `/billing/v1/costs` line item
  (real billed `amount` in dollars, `resource` id/name, `product`/`line_type`
  SKU), invoice-cost shape. Basic auth; `metadata.next` pagination; promo/negative
  lines flagged as `Credit`. Registered as `--provider confluent`
  (env `CONFLUENT_CLOUD_API_KEY` / `CONFLUENT_CLOUD_API_SECRET`).
- OpenRouter adapter (LLM gateway): one FOCUS 1.2 record per (day, model, upstream
  provider) from the `/api/v1/activity` API, with real credit spend (USD) as
  `BilledCost` and token counts as `x_` extensions. One integration captures spend
  across every model/provider routed through the gateway. `byok_usage_inference`
  is kept out of `BilledCost` (carried as `x_ByokUsage`) to avoid double-counting.
  Requires a management key; iterates the window day by day (30-day API cap) and
  skips out-of-window days. Registered as `--provider openrouter`
  (env `OPENROUTER_MANAGEMENT_KEY`).
- OpenAI adapter: token records per (day, model, bucket) from the
  `usage/completions` Admin API and separate line-item cost records (real dollars)
  from `costs`. Cost and token detail are separate grains because the adapter
  groups cost by `line_item` (a coarse product bucket that can't be attributed to
  a model). Cache-write tokens map to `cache_creation`, audio tokens ride as `x_`
  extensions, and malformed rows are skipped per row. Registered as
  `--provider openai` (env `OPENAI_ADMIN_KEY`, optional `OPENAI_ORG_ID`).
- Registry capability flag (`Capabilities.RequiresTimeRange`) so the CLI rejects
  an open-ended window for providers whose API mandates a start time, instead of
  hard-coding provider names.

- Richer FOCUS output across providers: `Publisher`, `InvoiceIssuer`,
  `ChargeFrequency`, `PricingCategory`, `PricingCurrency`, `ContractedCost`, and
  `ResourceId`/`ResourceName` are now populated. The Anthropic adapter also emits
  derived per-MTok pricing (`PricingQuantity`, `PricingUnit`, `ListUnitPrice`,
  `ContractedUnitPrice` - a blended effective rate when a day mixes tiers),
  `SkuPriceDetails`, and `BillingAccountId`/`BillingAccountName` resolved
  automatically from `GET /v1/organizations/me` (override the id with
  `ANTHROPIC_ORG_ID`).
- Typed FOCUS enums in `pkg/model` (`ChargeCategory`, `ChargeFrequency`,
  `PricingCategory`, `ServiceCategory`, `ServiceSubcategory`) with `Valid()`
  checks, so adapters emit spec-conformant values instead of raw strings.
- Anthropic adapter: one FOCUS 1.2 record per (day, model, token bucket) from
  the Admin API, joining the cost report (real billed cost, cents to major) with
  the messages usage report (token counts). Both cache-creation token types sum
  into `cache_creation`; token detail rides as `x_` extensions. Registered as
  `--provider anthropic` (env `ANTHROPIC_ADMIN_KEY`).
- PlanetScale billing adapter: one FOCUS 1.2 record per (invoice month,
  database, billing metric) from the PlanetScale invoices API, with real billed
  cost, credit/proration lines flagged as `ChargeCategory = "Credit"`, and
  `x_InfraProvider` carrying the underlying cloud.
- CLI (`focus-exporter`) with `--provider`, `--month`, `--start`/`--end`,
  `--format json|csv`, and `-o/--out`. With no window flags it exports the full
  available period.
- Generated FOCUS 1.2 record type from `pkg/focus/columns.json`, with JSON and
  CSV sinks.
- Project documentation: README, per-provider docs under `docs/providers/`,
  contributing guide, code of conduct, and security policy.
