# CLAUDE.md - focus-exporter

Agent conventions for this repo live in **[AGENTS.md](AGENTS.md)** - read it
before adding or changing a provider adapter.

## Output must be information-rich

Every FOCUS record must carry the maximum useful detail the source API provides.
When building or reviewing an adapter, capture **every** meaningful field the
response returns: map each to a FOCUS column when one fits, otherwise an `x_`
extension. That includes token breakdowns, per-unit prices, pricing/consumed
quantities, resource and account identity, and provider / endpoint / tier /
region dimensions.

A sparse, few-field record is a defect. If the API returns a field and it isn't
in the output, that's a gap to close - prefer more signal over less, always.

See AGENTS.md -> "Maximize information density" for the full rule and the rest of
the adapter conventions (seams, adapter shapes, money handling, hermetic tests,
real-data verification, the gate).
