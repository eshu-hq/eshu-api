# ComponentExtensionReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. Wave-1b component-extensions core pin, per the
arbiter Wave-1b ruling on eshu-hq/eshu-api#101 (PROCEED on both legs —
no hold, no owed core work). Minted core-side; the satellite records
this pin here. A satellite PR consuming an unminted pin is invalid.

## Shape

Version: `ComponentExtensionReadAdmissionContractVersion = "v0"`
(`go/internal/query/component_extension_read_admission.go`).

1 re-declared method (`Readback`) — sealed:

```go
type ComponentExtensionReadAdmission interface {
	Readback(component.Policy) ([]component.RegistryReadbackComponent, error)
}
```

There is deliberately no constructor on this pin: the satellite keeps
building its backends the way `cmd/api` wiring builds them today, and
any other backend pairing needs a new arbiter ruling. The satellite
serves the reads through the versioned forward transport only.

Enforcement: the frozen method-set contract test (whole-file
byte-identical at core `9bf29e53`, sha256 `6ee1e46874fc0a5a7471925ced6da2b09e7f3ab152774781af2e31fdaad0c371`), plus the
compile-time mirror/satisfier assertions in the vendored port
(whole-file byte-identical at core `9bf29e53`, sha256 `601c577ee7f906928e7779ed58b31d9104898b76180d951ea3e2b140666a8f19`).

## Census

| Route | Admits through |
|---|---|
| `GET /api/v0/component-extensions` | the pinned read above (list projection is handler behavior) |
| `GET /api/v0/component-extensions/{component_id}/diagnostics` | the pinned read above (`component_id` filter + 404 are handler behavior) |

## Explicitly OUT

- ComponentHome and Policy construction are config, not port shape.
- Grant checks, fixed failure messages, and QueryProfile are handler
  behavior, not port shape.
- Selectors (the `component_id` path value, the limit query bound) are
  handler behavior, not port shape.
- The mutation and wider read surface (Install, Enable, Disable,
  Uninstall, PlanEnable, List, producer grants) stays unpinned here by
  construction: the routes never issue a registry write.
- No ReadStore surface: the satellite satisfies this port with its own
  wiring; this pin carries the read shape only.
- Sibling families and their contracts (every other pin). No other
  family may cite this pin as coverage.

## Satellite consumption

Serve the route reads through the forwarded core call (the `compext`
wave lane, `contracts/component-extension-termination.md`). A satellite
PR consuming this pin for any other backend or family is invalid
without arbiter review.
