# focus-exporter

[![CI](https://github.com/baselinehq/focus-exporter/actions/workflows/ci.yml/badge.svg)](https://github.com/baselinehq/focus-exporter/actions/workflows/ci.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Go 1.26+](https://img.shields.io/badge/Go-1.26+-00ADD8.svg)](go.mod)

A standalone Go binary that pulls a provider's own cost/usage API and emits
[FinOps FOCUS 1.2](https://focus.finops.org/) records as JSON or CSV.

It is gateway-independent (no shared service, no database) and domain-agnostic:
each provider is a small adapter behind one interface, so the same tool exports
managed-infrastructure cost, streaming-platform cost, and AI model/token cost
into one normalized FOCUS schema. Fields FOCUS does not yet model natively ride
as `x_`-prefixed extension columns, which conformant consumers ignore.

## Integrations

| Provider | Category | Grain exported | Setup |
| --- | --- | --- | --- |
| <img src="https://cdn.simpleicons.org/planetscale/888888" height="18" align="center"> **PlanetScale** | Managed database | invoice month x database x billing metric | [planetscale.md](docs/providers/planetscale.md) |
| <img src="https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/anthropic.svg" height="18" align="center"> **Anthropic** | AI / LLM | day x model x token bucket (cost + tokens) | [anthropic.md](docs/providers/anthropic.md) |
| <img src="https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/openai.svg" height="18" align="center"> **OpenAI** | AI / LLM | day x model x token bucket + line-item cost | [openai.md](docs/providers/openai.md) |
| <img src="https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/confluent.svg" height="18" align="center"> **Confluent Cloud** | Streaming (Kafka) | billing line item (product x resource) | [confluent.md](docs/providers/confluent.md) |
| <img src="https://cdn.simpleicons.org/openrouter/888888" height="18" align="center"> **OpenRouter** | AI gateway | day x model x upstream provider (credit spend) | [openrouter.md](docs/providers/openrouter.md) |
| <img src="https://cdn.simpleicons.org/modal/888888" height="18" align="center"> **Modal** | Serverless GPU / compute | day x object (per-resource CPU/mem/GPU cost) | [modal.md](docs/providers/modal.md) |

One integration behind an LLM gateway (OpenRouter) captures spend across every
model and upstream provider you route through it. Per-provider setup
(environment variables, credential scopes, API endpoints, and the full FOCUS
mapping) lives under [docs/providers/](docs/providers/).

## Why FOCUS 1.2

FOCUS 1.2 is the version with ecosystem traction (the FinOps conformance
program is scoped to it, and cloud-native FOCUS exports are GA on it). Fields
that FOCUS does not yet model natively are carried as `x_`-prefixed extension
columns, which conformant consumers ignore. When later spec versions promote
those semantics to native columns, the extension columns can be mapped over
mechanically.

## Install

```bash
go install github.com/baselinehq/focus-exporter/cmd/focus-exporter@latest
```

Or build from source:

```bash
git clone https://github.com/baselinehq/focus-exporter
cd focus-exporter
go build ./cmd/focus-exporter
```

Requires Go 1.26+. No runtime dependencies beyond the standard library.

## Usage

```bash
focus-exporter --provider planetscale --month 2026-07 --format json
```

Flags:

| Flag | Meaning | Default |
| --- | --- | --- |
| `--provider` | Provider to export (repeatable) | required |
| `--month` | `YYYY-MM`, expands to that month's `[start, end)` | - |
| `--start` / `--end` | Explicit window, RFC3339 or `YYYY-MM-DD` | - |
| `--format` | `json` or `csv` | `json` |
| `-o`, `--out` | Output file | stdout |

Provide either `--month` or `--start`/`--end` together (not both, and `--start`
without `--end` is rejected). With no window flags at all, the exporter fetches
the entire period the provider makes available - except providers whose API
mandates a start time (the AI/streaming ones), which require an explicit window.

Every emitted record is validated against the FOCUS 1.2 mandatory-column rules
before it is written; a record that would violate them fails the run rather than
producing non-conformant output. A provider that fails at fetch time is logged
to stderr and skipped; a provider that cannot be built at all (unknown name or
missing credentials) is a fatal error, so a misconfigured export never silently
produces an empty file.

## Example output (PlanetScale)

```json
{
  "BilledCost": "30.0",
  "EffectiveCost": "30.0",
  "ListCost": "30.0",
  "BillingCurrency": "USD",
  "BillingPeriodStart": "2026-07-01T00:00:00Z",
  "BillingPeriodEnd": "2026-08-01T00:00:00Z",
  "ChargePeriodStart": "2026-07-01T00:00:00Z",
  "ChargePeriodEnd": "2026-08-01T00:00:00Z",
  "Provider": "PlanetScale",
  "ServiceName": "PlanetScale",
  "ServiceCategory": "Databases",
  "ServiceSubcategory": "Managed Database",
  "ChargeCategory": "Usage",
  "ResourceId": "7p5sqtyldrf6",
  "ResourceName": "pscale-1",
  "ResourceType": "Database",
  "RegionId": "us-east",
  "RegionName": "AWS us-east-1",
  "SkuId": "scaler_pro",
  "SkuMeter": "PS_10_AWS_ARM",
  "ChargeDescription": "PS-10-AWS-ARM for branch 'main'",
  "SkuPriceId": "scaler_pro|PS_10_AWS_ARM",
  "x_InfraProvider": "AWS"
}
```

## FOCUS record type

`pkg/focus` holds the canonical FOCUS 1.2 record. The struct is generated from
`pkg/focus/columns.json` - the single source of truth for the column set and
each column's data type and nullability - by a repo-owned generator:

```bash
go generate ./...
```

Do not hand-edit `pkg/focus/record_gen.go`. To change the column set, edit
`columns.json` and regenerate. A round-trip test asserts a full FOCUS 1.2
example survives marshal/unmarshal through the typed struct without losing a
column.

## Adding a provider

See [AGENTS.md](AGENTS.md) for the full conventions. In short:

1. Create `pkg/integrations/<provider>/` with a
   `New(get integrations.HTTPGet, ...creds) integrations.Source` constructor and
   a `Fetch(ctx, start, end) ([]model.UsageRecord, error)` that fills the
   FOCUS-core fields on `model.UsageRecord` (and `Extensions` for `x_` columns).
2. Expose a package-level `var Provider = integrations.Provider{...}` that reads
   the provider's env vars, and add it to `defaultRegistry` in
   `cmd/focus-exporter/main.go` (one line, no factory in the CLI).
3. Add `docs/providers/<provider>.md`, a row to the Integrations table above,
   and hermetic tests beside the package
   (`pkg/integrations/<provider>/<provider>_test.go`), with fixtures under
   `pkg/integrations/<provider>/testdata/`.

`model.UsageRecord` is domain-agnostic: infrastructure providers fill the
resource/region/period fields, and model providers add token detail through the
`Extensions` bag.

## Layout

```text
cmd/focus-exporter/          CLI (flags, provider registry, window guard, sinks)
pkg/focus/                   canonical FOCUS 1.2 Record (generated) + mapper + validator
pkg/model/                   UsageRecord (domain-agnostic) + typed FOCUS enums
pkg/sink/                    Sink interface + JSON/CSV writers
pkg/integrations/            Source interface + HTTPGet + Registry + Provider descriptor
pkg/integrations/<name>/     one adapter per provider
internal/gen/                FOCUS-type generator (go:generate)
```

## Development

```bash
go generate ./...
go build ./...
go vet ./...
go test -race ./...
```

## Roadmap

- More provider adapters across AI, data, database, observability, and comms
  categories.
- A stable synthetic record id (`x_LineItemId`) so downstream ingestion can
  dedup without relying on field values, which FOCUS does not guarantee to be
  unique.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for the
development setup, the checks a change has to pass, and how to add a provider.
Participation is governed by our [Code of Conduct](CODE_OF_CONDUCT.md).

Security issues: please follow [SECURITY.md](SECURITY.md) and do not open a
public issue. Release notes live in [CHANGELOG.md](CHANGELOG.md).

## License

focus-exporter is dual-licensed:

- **Open source:** [GNU AGPL-3.0](LICENSE). Free to use, modify, and
  redistribute; if you modify it and distribute it or offer it over a network,
  you must release your changes under the AGPL.
- **Commercial:** for organizations that cannot meet the AGPL's terms (embedding
  in a closed-source product, offering a proprietary/hosted service without
  releasing source, etc.). Contact **support@costgraph.ai**.

See [LICENSING.md](LICENSING.md) for details and [NOTICE](NOTICE).
