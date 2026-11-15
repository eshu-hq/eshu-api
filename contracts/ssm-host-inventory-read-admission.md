# SSMHostInventoryReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `SSMHostInventoryReadAdmissionContractVersion = "v0"`
(`go/internal/query/ssm_host_inventory_read_admission.go`).

2 re-declared methods (`SSMHostActiveFacts`, `Run` SSM-scoped) — sealed:

```go
type SSMHostInventoryReadAdmission interface {
	SSMHostActiveFacts(context.Context, ssmHostInventoryFilter) ([]ssmAdmittedFact, error)
	Run(context.Context, string, map[string]any) ([]map[string]any, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `e7b2a9718bef94cbb656bf020d60063da3583bf93cf662ab9eb62b6dd7c907c7`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `2163b493dac66754a0680f85a332b435d781a7825ff531ee75b0a`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/cloud/ssm-host-inventory` | the pinned read(s) above |

## Explicitly OUT

- Grant checks, fixed failure messages, and QueryProfile are handler behavior, not port shape..
- No constructor or ReadStore surface: the satellite satisfies this
  port with its own wiring; this pin carries the read shape only.
- Sibling families and their contracts (every other W8b-narrow pin,
  the W8a pins, the drift pin, and all code-search/graph pins).

## Satellite consumption

Serve the route reads through the forwarded core call (the
`w8bnarrow` wave lane, `contracts/w8b-narrow-termination.md`). A
satellite PR consuming this pin for any other backend or family is
invalid without arbiter review.
