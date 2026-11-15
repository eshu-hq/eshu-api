# KubernetesCorrelationReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `KubernetesCorrelationReadAdmissionContractVersion = "v0"`
(`go/internal/query/kubernetes/correlation_read_admission.go`).

1 re-declared method (`ListKubernetesCorrelations`) — sealed:

```go
type KubernetesCorrelationReadAdmission interface {
	ListKubernetesCorrelations(context.Context, CorrelationFilter) ([]CorrelationRow, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `f333cae438a363a9f1a30629a117c3ed4930e4bb070ba3912dbd58523e078dc2`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `177f03166de0f17a0d31047c859d4b7f93df170b17a32ac58671721f927beea1`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/kubernetes/correlations` | the pinned read(s) above |

## Explicitly OUT

- The scoped-empty short-circuit, grant checks, fixed failure messages, and QueryProfile are handler behavior, not port shape..
- No constructor or ReadStore surface: the satellite satisfies this
  port with its own wiring; this pin carries the read shape only.
- Sibling families and their contracts (every other W8b-narrow pin,
  the W8a pins, the drift pin, and all code-search/graph pins).

## Satellite consumption

Serve the route reads through the forwarded core call (the
`w8bnarrow` wave lane, `contracts/w8b-narrow-termination.md`). A
satellite PR consuming this pin for any other backend or family is
invalid without arbiter review.
