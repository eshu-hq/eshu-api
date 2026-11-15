# WorkItemEvidenceReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `WorkItemEvidenceReadAdmissionContractVersion = "v0"`
(`go/internal/query/workitem/evidence_read_admission.go`).

1 re-declared method (`ListWorkItemEvidence`) — sealed:

```go
type WorkItemEvidenceReadAdmission interface {
	ListWorkItemEvidence(context.Context, EvidenceFilter) (EvidencePage, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `316d3fd6ee4f005ff632c70db240c122893808e6f5059b708d4608f260182a`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `cc3c56d162b9038b509231e79e56af83e79ba2b30ac5179778666e7be09a6dc`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/work-items/evidence` | the pinned read(s) above |

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
