# ServiceCatalogCorrelationReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `ServiceCatalogCorrelationReadAdmissionContractVersion = "v0"`
(`go/internal/query/service/catalog_correlation_read_admission.go`).

3 re-declared methods (`ListServiceCatalogCorrelations`, `ListServiceCatalogLocalDescriptorEvidence`, `MatchRepositories`) — sealed:

```go
type ServiceCatalogCorrelationReadAdmission interface {
	ListServiceCatalogCorrelations(context.Context, CatalogCorrelationFilter) ([]CatalogCorrelationRow, error)
	ListServiceCatalogLocalDescriptorEvidence(context.Context, string, int) ([]CatalogLocalDescriptorEvidenceRow, error)
	MatchRepositories(context.Context, string) ([]querycontract.RepositoryCatalogEntry, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `01415c3849eac2f62b3d33343891ef6b086d0881720a73236c3db098a8c321`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `fa9739304a6c858c0eb3de974c5eb2260c43d0772e5cd2a7bd5a7fbd32`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/service-catalog/correlations` | the pinned read(s) above |

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
