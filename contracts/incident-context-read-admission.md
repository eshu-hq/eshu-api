# IncidentContextReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `IncidentContextReadAdmissionContractVersion = "v0"`
(`go/internal/query/incident/context_read_admission.go`).

2 re-declared methods (`ReadIncidentContext`, `ResolveDurableIncidentRepositories`) — sealed:

```go
type IncidentContextReadAdmission interface {
	ReadIncidentContext(context.Context, model.IncidentContextFilter) (model.IncidentContextSnapshot, error)
	ResolveDurableIncidentRepositories(context.Context, string, string, string) ([]string, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `c6ddac1566147c9d23bed5b9b18f1cb609d0d9b42f20bcacfae4d3bdaef0d0a`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `41992d662a150adffd0afd0837d52acf489e8c3465b502f89e8c8`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/incidents/{incident_id}/context` | the pinned read(s) above |

## Explicitly OUT

- Grant checks, fixed failure messages, packet bounds, and QueryProfile are handler behavior, not port shape..
- No constructor or ReadStore surface: the satellite satisfies this
  port with its own wiring; this pin carries the read shape only.
- Sibling families and their contracts (every other W8b-narrow pin,
  the W8a pins, the drift pin, and all code-search/graph pins).

## Satellite consumption

Serve the route reads through the forwarded core call (the
`w8bnarrow` wave lane, `contracts/w8b-narrow-termination.md`). A
satellite PR consuming this pin for any other backend or family is
invalid without arbiter review.
