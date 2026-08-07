# Contributing to focus-exporter

Thanks for your interest in improving focus-exporter. This document covers how
to set up a development environment, the checks a change has to pass, and the
conventions the codebase follows.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Development setup

focus-exporter is a standalone Go binary with no runtime dependencies beyond the
standard library.

Requirements:

- Go 1.26 or newer.

```bash
git clone https://github.com/baselinehq/focus-exporter
cd focus-exporter
go build ./...
```

## The checks

Every change has to pass the same gate CI runs:

```bash
go generate ./...   # regenerate pkg/focus/record_gen.go from columns.json
go build ./...
go vet ./...
gofmt -l .          # must print nothing
go test -race ./...
```

Dead code is not allowed to accumulate. New symbols with no caller are removed
before a change lands (including tests that only exercise themselves):

```bash
GOTOOLCHAIN=go1.26.0 go install golang.org/x/tools/cmd/deadcode@latest
deadcode ./...
```

Filter results to symbols your branch touches; pre-existing unreachable
functions on `main` are out of scope for your change.

## FOCUS record type

`pkg/focus/columns.json` is the single source of truth for the FOCUS 1.2 column
set, each column's data type, and its nullability. The record struct in
`pkg/focus/record_gen.go` is generated from it:

```bash
go generate ./...
```

Do not hand-edit `record_gen.go`. To change the column set, edit `columns.json`
and regenerate. A round-trip test asserts a full FOCUS 1.2 example survives
marshal/unmarshal through the typed struct without losing a column.

## Adding a provider

A provider is a small adapter behind one interface. See the walkthrough in
[docs/providers/README.md](docs/providers/README.md) and an existing adapter in
`pkg/integrations/planetscale/` for the shape.

In short:

1. Create `pkg/integrations/<provider>/` with a `New(...) integrations.Source`.
2. Implement `Fetch(ctx, start, end) ([]model.UsageRecord, error)`, filling the
   FOCUS-core fields on `model.UsageRecord` (and `Extensions` for any `x_`
   columns).
3. Register it in the CLI's registry.
4. Add a `docs/providers/<provider>.md` documenting the credentials, scopes, API
   endpoints, and FOCUS mapping.
5. Ship tests. HTTP is injected as `integrations.HTTPGet`, so adapters are
   tested against fixtures with no live credentials or network.

Real billed cost is the point: map the provider's own invoiced/billed amount,
not an estimate. Surface credits and prorations as-is (negative cost,
`ChargeCategory = "Credit"`) rather than netting them away.

## Tests

- Table-driven subtests are the default. Extend the existing test for an area
  rather than adding a near-duplicate function.
- Cover the happy path and at least one error/edge path (malformed payload,
  not-found, boundary window).
- Tests must be hermetic: fixtures under `testdata/`, no live network.

## Commits and pull requests

- Keep changes minimal and focused; touch only what the change needs.
- Write plain, descriptive commit subjects (imperative mood, e.g. "fix: skip
  invoices with invalid billing periods").
- Open the PR against `main`. Fill in the pull request template.
- Make sure the full gate above is green before requesting review.

## Reporting bugs and requesting features

Use the issue templates. For anything security-sensitive, do **not** open a
public issue: follow [SECURITY.md](SECURITY.md).

## License

focus-exporter is dual-licensed under the [GNU AGPL-3.0](LICENSE) and a
commercial license (see [LICENSING.md](LICENSING.md)). By contributing, you
agree that your contributions are licensed under the AGPL-3.0 and may also be
offered under the commercial license, so the dual-licensing model remains
viable.
