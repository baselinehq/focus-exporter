# Modal

Exports FOCUS 1.2 records from Modal's workspace billing report. Modal is a
serverless GPU/compute platform; this adapter captures the per-object, per-day
cost of everything running in a workspace - including self-hosted model
inference (Moondream, Qwen-VL, and other apps deployed on Modal).

Provider name (for `--provider`): `modal`

Unlike the other adapters, Modal has no REST billing endpoint: the billing data
is served over **gRPC** (`api.modal.com:443`, service `modal.client.ModalClient`,
RPC `WorkspaceBillingReport`, server-streaming). This adapter therefore talks
gRPC directly rather than going through the shared `HTTPGet` seam.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `MODAL_TOKEN_ID` | yes | Token id (`modal token new`), sent as gRPC metadata `x-modal-token-id`. |
| `MODAL_TOKEN_SECRET` | yes | Token secret, sent as gRPC metadata `x-modal-token-secret`. |
| `MODAL_WORKSPACE_ID` | yes | Populates the mandatory `BillingAccountId`. The workspace itself is implied by the token; this is a stable label for the account. |

## API used

| Transport | Target | Purpose |
| --- | --- | --- |
| gRPC (TLS) | `api.modal.com:443` `modal.client.ModalClient/WorkspaceBillingReport` | Per-object, per-interval usage cost. The adapter requests `resolution="d"` (daily) over the `[start, end)` window. |
| gRPC (TLS) | `api.modal.com:443` `modal.client.ModalClient/WorkspaceBillingSummary` | Month `metered_cost` vs `billed_cost` plus an `adjustments` map, used to emit the non-usage charges so total `BilledCost` reconciles to the invoice. |

## FOCUS mapping

| FOCUS column | Source |
| --- | --- |
| `BilledCost` / `EffectiveCost` / `ListCost` / `ContractedCost` | item `cost` (USD) |
| `BillingCurrency` / `PricingCurrency` | `USD` |
| `ChargePeriodStart` / `ChargePeriodEnd` | item `interval` (the day) |
| `BillingPeriodStart` / `BillingPeriodEnd` | calendar month of the interval |
| `Provider` / `Publisher` / `InvoiceIssuer` | `Modal` |
| `ServiceName` | `Modal` |
| `ServiceCategory` | `Compute` |
| `ResourceId` / `SkuId` | item `object_id` |
| `ResourceName` / `ChargeDescription` | item `description` |
| `ResourceType` | derived from the `object_id` prefix (`ap`->App, `fu`->Function, `vo`->Volume, `im`->Image, `sb`->Sandbox) |
| `SkuPriceDetails` | item `cost_by_resource` (per-resource breakdown, e.g. `GPU:H100`, `CPU`, `Memory`) |
| `BillingAccountId` | `MODAL_WORKSPACE_ID` |
| `x_Environment` | item `environment_name` |
| `x_Tags` | item `tags` |

## Example

```bash
export MODAL_TOKEN_ID=ak-...
export MODAL_TOKEN_SECRET=as-...
export MODAL_WORKSPACE_ID=your-workspace
focus-exporter --provider modal --start 2026-07-01 --end 2026-08-01 --format json
```

## Non-usage charges

Beyond per-object usage, the workspace billing summary's `adjustments` map is
emitted as its own records so the total reconciles to what Modal actually bills:

| Adjustment key | `ChargeCategory` | Notes |
| --- | --- | --- |
| `plan` | Purchase | platform/plan fee |
| `reservations` | Purchase | committed-use (`PricingCategory = Committed`) |
| `credits` | Credit | applied credits (negative) |
| `storage` | Usage | storage charge |
| anything else | Adjustment | forward-compatible catch-all |

Each carries the summary month as its charge period. Amounts are taken from the
API verbatim (the summary is signed so `metered_cost + sum(adjustments) =
billed_cost`); the exact keys and signs are pending live verification against a
real workspace.

## Notes and limitations

- **A window is required** (the report needs a start and end). Pass
  `--start`/`--end` or `--month`.
- **Non-usage charges need the monthly summary.** They are keyed to the summary
  month (`WorkspaceBillingSummary` is monthly), while usage records are daily. If
  the summary call fails, usage records are still emitted and the non-usage
  charges are skipped with a log line.
- **Daily grain.** The adapter requests `resolution="d"`; each record is one
  object for one day. The per-resource split (CPU / memory / specific GPU types)
  rides in `SkuPriceDetails`, so GPU spend is visible without losing the object
  total in `BilledCost`.
- **Rows with no cost are skipped** rather than emitted as empty-cost records.
- The gRPC client and the trimmed billing protobuf live under
  `pkg/integrations/modal/modalpb/` (regenerate with `buf generate` from
  `billing.proto`; the checked-in `*.pb.go` are the source of truth at build
  time).
