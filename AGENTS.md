# AGENTS.md - focus-exporter

Conventions for this repo. Follow them so new provider adapters look like the
ones already shipped (PlanetScale, Anthropic, OpenAI, Confluent, OpenRouter).

## Every change ships as one unit

Code, docs, and tests move together in the same change - never one without the
others.

- **Docs track the code.** Any change to a flag, env var, provider, FOCUS
  mapping, or behaviour updates the docs in the same commit: the affected
  `docs/providers/<name>.md`, the Integrations table and flag table in
  `README.md`, and `CHANGELOG.md` under `## [Unreleased]`. A new provider also
  adds a logo row to the README table. Stale docs are a defect, not a follow-up.
- **Tests come with the code.** Every new adapter, function, or behaviour change
  ships a test in the same change (table-driven, hermetic - see Tests below). A
  change with no test is not done.
- **Verify before you claim done.** Fixtures passing is not proof. Run the real
  code path and check the output, and for an adapter run it against a live key
  over a recent window and sanity-check the emitted dollars/tokens against the
  provider's console (see "Verify against real data"). If you cannot, say so
  explicitly and mark live verification pending - do not imply it works.
- **Run the gate** (bottom of this file) before reporting any change done.

## What this tool is

A standalone Go binary that pulls a provider's own billing/usage API and emits
FinOps **FOCUS 1.2** records as JSON or CSV. Gateway-independent: no shared
service, no database. Each provider is a small adapter behind one interface.

Dependencies are kept **minimal and well-chosen**, not zero: stdlib first, then
`golang.org/x/*`, then a de-facto-standard library only when a format or
protocol genuinely needs one (`shopspring/decimal` for money,
`google.golang.org/grpc` + `protobuf` for the one provider - Modal - whose
billing is gRPC-only). Do not add a dependency an adapter could reasonably do
without; when in doubt, prefer the stdlib.

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
- **`integrations.HTTPGet`** `func(ctx, url, headers) ([]byte, error)`: the
  default way an adapter does HTTP. It is injected, so adapters are tested
  against fixtures with **no network and no credentials**. Never import
  `net/http` in an HTTP adapter; the CLI provides the real client. A non-HTTP
  provider (e.g. Modal over gRPC) owns its own client, but still keeps the same
  testability contract: put the transport behind a small injected function type
  (Modal's `reporter`) so `Fetch` is tested against in-memory items with no
  network - never dial in a unit test.
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
4. **gateway** (OpenRouter): one record per (day, model, upstream provider) with
   real billed spend; keep pass-through/BYOK spend out of `BilledCost`.
5. **gRPC / streaming** (Modal): provider with no REST billing API. A trimmed
   `.proto` (billing messages + the one RPC, matching the vendor's package and
   service name) lives under `<adapter>/modalpb/`; regenerate the checked-in
   `*.pb.go` with `buf generate` (not wired into `go generate ./...`, which must
   stay dependency-free). Generated `*.pb.go` are exempt from the no-comments
   rule and from deadcode (the grpc plugin emits unused server-side types).

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
- **Constructor** `New(get integrations.HTTPGet, ...creds) integrations.Source`
  (explicit creds, so tests build a source directly).
- **Registration** is a package-level `var Provider = integrations.Provider{...}`
  exposing `Name`, `Capabilities`, and a `New(get, env)` that reads the provider's
  env vars and calls the constructor. Add it to the slice in
  `defaultRegistry` (`cmd/focus-exporter/main.go`) - one line, no factory in the
  CLI. Set `Capabilities{RequiresTimeRange: true}` when the API mandates a start
  time; never name-match providers in the CLI.
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
