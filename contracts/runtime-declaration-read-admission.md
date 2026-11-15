# RuntimeDeclarationReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W8b-narrow core pin, per the arbiter
SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#101. Minted
core-side; the satellite records this pin here. A satellite PR
consuming an unminted pin is invalid.

## Shape

Version: `RuntimeDeclarationReadAdmissionContractVersion = "v0"`
(`go/internal/query/runtime/declaration/read_admission.go`).

1 re-declared method (`Run`, declaration-scoped) — sealed:

```go
type RuntimeDeclarationReadAdmission interface {
	Run(context.Context, string, map[string]any) ([]map[string]any, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `76240fda9f`, sha256 `05d9393302ce4b3b71ae5dfd884c058c79e19e26ba2e1dc8ff54fac3cfd3f9a2`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `76240fda9f`, sha256 `4c4507c4959f3920ab45924288ca4a855d8a5976227ef11c4fe7b26fe`).

## Census

| Route | Admits through |
|---|---|
| `POST /api/v0/runtime-declarations/query` | the pinned read(s) above |

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
