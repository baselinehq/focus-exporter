# focus-exporter

A standalone Go binary that pulls a provider's own cost/usage API and emits
[FinOps FOCUS 1.2](https://focus.finops.org/) records as JSON or CSV.

It is gateway-independent (no shared service, no database) and domain-agnostic:
each provider is a small adapter behind one interface, so the same tool exports
infrastructure cost and, later, AI model cost. The first shipped provider is
**PlanetScale**, whose monthly invoices map cleanly onto native FOCUS columns
(real `ResourceId`, `RegionName`, `SkuMeter`); the only vendor-specific
extension is `x_InfraProvider` (the underlying cloud), everything else is
native.

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

Provide either `--month` or `--start`/`--end` (not both). With no window flags
at all, the exporter fetches the entire period the provider makes available.

A provider that fails at fetch time is logged to stderr and skipped; the run
still emits the records it could gather. A provider that cannot be built at all
(unknown name, or missing credentials) is a fatal error, so a misconfigured
export never silently produces an empty file.

## Providers

Per-provider setup (environment variables, credential scopes, API endpoints,
FOCUS mapping) is documented under [docs/providers/](docs/providers/).

### PlanetScale

Full setup: [docs/providers/planetscale.md](docs/providers/planetscale.md).

Exports one FOCUS record per (invoice month, database, billing metric) from the
PlanetScale billing API.

Authentication uses a [service token](https://api-docs.planetscale.com/reference/service-tokens):

```bash
export PLANETSCALE_ORG=your-org
export PLANETSCALE_SERVICE_TOKEN_ID=xxxxxxxxxxxx
export PLANETSCALE_SERVICE_TOKEN=pscale_tkn_...
focus-exporter --provider planetscale --month 2026-07
```

The service token needs read access to the organization's invoices and
databases.

Example output:

```json
{
  "BilledCost": "30.0",
  "EffectiveCost": "30.0",
  "ListCost": "30.0",
  "BillingCurrency": "USD",
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
  "ChargeDescription": "PS_10_AWS_ARM",
  "SkuPriceId": "scaler_pro|PS_10_AWS_ARM",
  "x_InfraProvider": "AWS"
}
```

Notes on PlanetScale mapping:

- `subtotal` fills `BilledCost`, `EffectiveCost`, and `ListCost`; there is no
  separate negotiated rate on the invoice.
- Credit and proration lines are surfaced as-is, including negative costs.
- `ChargePeriod` equals `BillingPeriod` (the calendar month): the invoice API
  only exposes monthly line items, so there is no finer charge granularity.
- The underlying cloud (`aws` / `gcp`) is carried as `x_InfraProvider`.

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

1. Create `pkg/integrations/<provider>/` with a `New(...) integrations.Source`.
2. Implement `Fetch(ctx, start, end) ([]model.UsageRecord, error)`, filling the
   FOCUS-core fields on `model.UsageRecord` (and `Extensions` for any `x_`
   columns).
3. Register it in the CLI's registry.

`model.UsageRecord` is domain-agnostic: infrastructure providers fill the
resource/region/period fields, and model providers add token detail through the
`Extensions` bag.

## Layout

```text
cmd/focus-exporter/          CLI
pkg/focus/                   canonical FOCUS 1.2 Record (generated) + mapper
pkg/model/                   UsageRecord (domain-agnostic)
pkg/sink/                    Sink interface + JSON/CSV writers
pkg/integrations/            Source interface + HTTPGet + Registry
pkg/integrations/planetscale PlanetScale adapter
internal/gen/                FOCUS-type generator (go:generate)
```

## Development

```bash
go generate ./...
go build ./...
go vet ./...
go test ./...
```

## Roadmap

- Model-cost providers (Anthropic, OpenAI) emitting token detail as `x_`
  extensions.
- A stable synthetic record id (`x_LineItemId`) so downstream ingestion can
  dedup without relying on field values, which FOCUS does not guarantee to be
  unique.

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
