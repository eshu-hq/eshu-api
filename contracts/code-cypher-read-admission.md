# CodeCypherReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W6d 2 core pin, per the arbiter supplemental
ruling on eshu-hq/eshu-api#98 (§2D, APPROVED for exactly the W6d 2:
`POST /api/v0/code/cypher`, `POST /api/v0/code/visualize`). Minted
core-side at `81b756fd84` (merge `eshu-hq/eshu#10688`); the satellite
records this pin here. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination of
the W6d legs.

## Shape

Version: `CodeCypherReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_cypher_read_admission.go).

Ceiling 1 re-declared (never embedded) method — the bounded graph
traversal (`Run`) — sealed, with `RunSingle` and every other
`GraphQuery` method OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | both POSTs (bounded traversal under `boundedReadOnlyCypher` / `boundedVisualizationCypher` plus the route timeout) |

Both routes execute caller-supplied graph programs with no bounded
selector to intersect against a grant: shared-key/all-scope callers
only — scoped and browser-session tokens are refused before the
handler runs (`x-shared-key-only`, `auth.SharedKeyOnlyRoutes`).
There is deliberately no constructor, builder, selector, or
ReadStore surface on this pin: the satellite satisfies this port
with its own wiring; this pin carries the read shape only.

Enforcement (all in go/internal/query/codequery):

- `code_cypher_read_admission_contract_test.go` — frozen method-set
  test plus version pin
  (`TestCodeCypherReadAdmissionContractIsVersioned` — exactly 1
  method: `Run`), with the satisfiability witness and the seeded
  re-signed rejection
  (`TestCodeCypherReadAdmissionRejectsResignedMethod`).
- `code_cypher_read_admission_wire_shapes_test.go` —
  (`TestCodeCypherReadAdmissionWireShapesAreSatelliteConsumable`)
  proves a satellite can consume the pin using only the pin plus the
  `querycontract` shapes.

## Census (per route per method)

| Method | `POST /api/v0/code/cypher` | `POST /api/v0/code/visualize` |
|---|---|---|
| `Run` | bounded read-only query (`boundedReadOnlyCypher`); raw result rows | bounded query (`boundedVisualizationCypher`); rows projected to a renderable visualization packet |

Reachability notes, recorded not silently assumed: the
mutation-keyword rejection, the injected terminal `LIMIT`, the
`cypher_query is required` gate, and the statement-rejection 400
all answer before `Run`; the visualize packet marks row-window
truncation with its limitation reason; both routes time out under
the route timeout against an `AccessModeRead` session.

## Explicitly OUT (not widened by this pin)

- `RunSingle` and every other `GraphQuery` method: this pin adopts
  `Run` only.
- The mutation-keyword rejection, the `LIMIT` injection, the
  statement timeout, the grant-class gate (shared-key-only), the
  fixed failure messages, and `QueryProfile` are handler behavior,
  not port shape: no method set to freeze. The satellite must port
  that validation with parity tests; the pin does not prove it.
- No constructor, builder, selector, or ReadStore surface.

## Satellite consumption

Depend on the shape, not a constructor: serve both routes' reads
through the concurrent-safe graph reader the way `cmd/api` wiring
builds it today, preserving the shared-key-only gate and the
read-only validation exactly. A satellite PR consuming this pin for
any other backend pairing is invalid without arbiter review.
