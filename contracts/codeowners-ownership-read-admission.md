# CodeownersOwnershipReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `CodeownersOwnershipReadAdmissionContractVersion = "v0"`
(`go/internal/query/codeowners/ownership_read_admission.go`).

3 re-declared methods (`Run`, `RunSingle`, ownership-scoped `ListServiceCatalogCorrelations`) — sealed:

```go
type CodeownersOwnershipReadAdmission interface {
	Run(context.Context, string, map[string]any) ([]map[string]any, error)
	RunSingle(context.Context, string, map[string]any) (map[string]any, error)
	ListServiceCatalogCorrelations(context.Context, querycontract.ServiceCatalogCorrelationFilter) ([]querycontract.ServiceCatalogCorrelationRow, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `75e862d6736eaf377e2f40ed7fda2b29f101aacb77e2b9aad09d119a3a8bd8`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `1ff07ee55e3c873747efecb7feae925c3f6fb395153bac96478c361f`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/codeowners/ownership` | the pinned read(s) above |

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
