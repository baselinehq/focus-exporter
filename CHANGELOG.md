# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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
