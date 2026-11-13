# Metrics read admission contract

Versioned admission-contract port for the metrics time-series read
(`GET /api/v0/metrics/timeseries`) (Refs `eshu-hq/eshu#9669`, O6 pin
2). Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned time-series source satisfies, composed by
the consumer. `eshu-api` never takes a direct store handle.

The pin re-declares the single mirrored signature in this
Metrics-scoped shape instead of embedding or aliasing
`TimeSeriesSource`, so no other family can cite it as coverage.

## Version

`MetricsReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type MetricsReadAdmission interface {
	RangeQuery(ctx context.Context, query RangeQuery) ([]Point, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/metrics-read-wire-shapes.txt`. The frozen method
set is `frozenMetricsReadAdmission` in
`contracts/upstream/metrics_read_admission_contract_test.go`
(1 method: `RangeQuery`).

## Read surface

The seam is read-only on purpose: the declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve the time-series read but
cannot acquire a write it did not bargain for through it.

## In scope

- `GET /api/v0/metrics/timeseries` admits its bounded time-series
  range read through `RangeQuery` (mirroring `TimeSeriesSource`).

Explicitly OUT of this pin: the code-graph-metrics POST (`POST
/api/v0/code/call-graph/metrics`) is a `GraphQuery` reader, not a
`TimeSeriesSource` consumer, and stays outside this pin — it
terminates in the Wave-7 G-a lane on `CodeGraphReadAdmission v0`
(see `contracts/code-graph-termination.md`); the grant check, the
metric allow-list, and the fixed failure messages are handler
behavior, not port shape.

## Relay posture

The metrics time-series leg stays on the generic relay until the
termination lane lands; no native route moves in this record. The pin
is recorded here first so the termination lane consumes a minted pin,
never an unminted one.

## Enforcement

`TestMetricsReadAdmissionContractIsVersioned` pins the version and
the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `MetricsReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
