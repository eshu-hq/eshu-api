# TagHistoryReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `TagHistoryReadAdmissionContractVersion = "v0"`
(`go/internal/query/tag_history_read_admission.go`).

1 re-declared method (`Run`, tag-history-scoped) — sealed:

```go
type TagHistoryReadAdmission interface {
	Run(context.Context, string, map[string]any) ([]map[string]any, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `a4b3852562c05fc4fa2484af942cf81ce2b9a69babae7d57802ab0a0ea28b5f8`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `5dc8e78ba6f1208a45addf7659df79cde66e80463d57e1e5a82ff35b568d`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/images/tag-history` | the pinned read(s) above |

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
