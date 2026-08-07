# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
