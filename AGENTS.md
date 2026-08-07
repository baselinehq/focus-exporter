# AGENTS.md - focus-exporter

Conventions for this repo. Follow them so new provider adapters look like the
ones already shipped (PlanetScale, Anthropic, OpenAI).

## What this tool is

A standalone Go binary, **stdlib-only at runtime** (no third-party deps in
non-test code), that pulls a provider's own billing/usage API and emits FinOps
**FOCUS 1.2** records as JSON or CSV. Gateway-independent: no shared service, no
database. Each provider is a small adapter behind one interface.

## Layout

```text
cmd/focus-exporter/          CLI (flags, provider registry, window guard, sinks)
pkg/focus/                   generated FOCUS 1.2 Record + FromUsage mapper
pkg/model/                   UsageRecord (domain-agnostic) + typed FOCUS enums
pkg/sink/                    Sink interface + JSON/CSV writers
pkg/integrations/            Source interface + HTTPGet + Registry
pkg/integrations/<name>/     one adapter per provider
internal/gen/                FOCUS-type generator (go:generate)
```

## The seams (do not bypass)

- **`integrations.Source`**: `Name() string` + `Fetch(ctx, start, end) ([]model.UsageRecord, error)`.
- **`integrations.HTTPGet`** `func(ctx, url, headers) ([]byte, error)`: the ONLY
  way an adapter does HTTP. It is injected, so adapters are tested against
  fixtures with **no network and no credentials**. Never import `net/http` in an
  adapter; the CLI provides the real client.
- **`model.UsageRecord`**: domain-agnostic FOCUS-core fields + an `Extensions`
  bag for `x_`-prefixed columns. Adapters fill this; `focus.FromUsage` maps it to
  the generated `focus.Record`. Add a field here (and map it in `FromUsage`)
  rather than reaching into `focus.Record` from an adapter.

## Adapter shapes (reuse one)

1. **invoice-cost** (PlanetScale): pre-priced line items; set `Cost`, real
   `ResourceId`/`SkuMeter`, `PeriodStart/End`.
2. **token-cost** (Anthropic): join a cost report to a usage report per
   (day, model, token bucket); token detail via `x_` extensions.
3. **token + separate cost** (OpenAI): when the cost API can't be joined to
   models, emit token records (no cost) and coarse cost records (no tokens);
   document the split grain.

## Rules for a new adapter

- **Use the typed FOCUS enums** in `pkg/model` (`ChargeCategory`,
  `ChargeFrequency`, `PricingCategory`, `ServiceCategory`, `ServiceSubcategory`) -
  never raw strings for those columns.
- **Fill the shared columns** every adapter sets: `Provider`, `Publisher`,
  `InvoiceIssuer`, `ServiceCategory`/`Subcategory`, `ChargeCategory`,
  `ChargeFrequency`, `PricingCategory`, `Currency`/`PricingCurrency`,
  `ChargeDescription`, `SkuMeter`/`SkuPriceID`, and `BillingAccountId`/`Name`.
  `BillingAccountId` is a mandatory FOCUS column: resolve it from the API when
  there's an endpoint (e.g. Anthropic `/organizations/me`), otherwise require an
  org-id env in the factory so it is never empty.
- **No float drift on money**: decode amounts as `json.Number` or a decimal
  string; never parse to `float64` and re-format. Cents vs dollars: verify per
  API and convert exactly (`math/big.Rat`).
- **Skip, don't emit, malformed rows** (bad amount, unparseable date) - log and
  continue; a single bad row must never abort the run or produce a nil-cost
  record.
- **Real billed cost, not estimates.** Map the provider's own invoiced/billed
  amount. Surface credits/prorations as-is (negative cost,
  `ChargeCategory = Credit`), never netted away.
- **Merge, don't overwrite, dimensions** when one logical bucket splits across
  rows.
- **Constructor** `New(get integrations.HTTPGet, ...creds) integrations.Source`.
  Register in `cmd/focus-exporter/main.go`. If the API mandates a start time,
  register with `integrations.Capabilities{RequiresTimeRange: true}` (the CLI
  rejects an open-ended window for those) - do NOT name-match providers in the
  CLI.
- **Docs**: add `docs/providers/<name>.md` (env vars, scopes, endpoints, FOCUS
  mapping table, example, limitations) and move the provider to "Available" in
  `docs/providers/README.md`.

## Tests

- Table-driven, hermetic: fixtures under `pkg/integrations/<name>/testdata/`,
  HTTP injected via `HTTPGet`, no network. Match the existing adapter tests.
- Cover the happy path AND at least one error/edge (malformed amount, boundary
  window, cost-without-usage).
- Extend the nearest existing test rather than adding a near-duplicate.

## Verify against real data

Fixtures passing is NOT proof. Before claiming an adapter done, run it against a
real key over a recent window and sanity-check the emitted dollars/tokens
against the provider's console. If no key is available, say so and mark live
verification pending. (Provider keys are session-only - never commit or persist
them.)

## The gate (run before reporting work done)

```sh
go generate ./...   # pkg/focus/record_gen.go must be regenerated from columns.json; leaves no diff
gofmt -l .          # empty
go vet ./...
go test -race ./...
GOTOOLCHAIN=go1.26.0 deadcode ./...   # nothing new on the branch
```

Do not hand-edit `pkg/focus/record_gen.go`; edit `columns.json` and regenerate.

## Standards

- **No comments.** Write zero comments - not on functions, structs, fields, or
  inline. Make the code read on its own with clear names and small functions.
  The only exception is a machine-required marker such as the generated-file
  `DO NOT EDIT` line. If you think a comment is needed, fix the code instead.
- **Plain ASCII, no AI tells.** No em-dash or en-dash (use a spaced hyphen, a
  comma, or two sentences), no ellipsis character (use three periods), no
  curly/smart quotes (use straight quotes), no arrows (use `->`), no
  multiplication sign (use `x`), no section sign (write "section"), no
  inequality glyphs (use `>=`, `<=`). Applies to code, docs, commits, and PRs.
- **Never ignore an error.** Handle every error path; do not discard with `_`.
  Surface cleanup/close failures too (`errors.Join` or log).
- **One builder for derived values.** A composite key, identity string, or
  fingerprint gets a single named function that owns its format; callers reuse
  it, never re-concatenate.
- **Tests are table-driven and minimal.** Extend the nearest existing test
  before adding a new one; cover the happy path and one realistic failure. Test
  LOC should not dwarf the code under test.
- **No attribution or tool branding** in commits, PR titles/bodies, or docs. No
  `Co-Authored-By`, no "generated with" lines.
- Smallest diff that matches the surrounding code. No backwards-compat shims or
  feature flags for cases that cannot happen.
