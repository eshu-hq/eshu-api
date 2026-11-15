# CloudInventoryReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `CloudInventoryReadAdmissionContractVersion = "v0"`
(`go/internal/query/cloud_inventory_read_admission.go`).

2 re-declared methods (`CloudInventoryIdentities`, `CloudInventoryPreRolloutEvidenceExists`) — sealed:

```go
type CloudInventoryReadAdmission interface {
	CloudInventoryIdentities(context.Context, cloudInventoryFilter) (cloudInventoryListReadModel, error)
	CloudInventoryPreRolloutEvidenceExists(context.Context, cloudInventoryFilter) (bool, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `c5fd8665c17aec3ca6aa96b17c3043a417445a83907482d0d9b42f20bcab3815175`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `a2f232ef8d636318963064a31f50f970ffcf966623e8a68693dc833d8051b067`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/cloud/inventory` | the pinned read(s) above |

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
