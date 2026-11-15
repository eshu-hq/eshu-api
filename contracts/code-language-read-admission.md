# CodeLanguageQueryReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W6c 1 core pin, per the arbiter supplemental
ruling on eshu-hq/eshu-api#98 (§2E, APPROVED for exactly the one W6c
language-query read: `POST /api/v0/code/language-query`). Minted
core-side at `81b756fd84` (merge `eshu-hq/eshu#10688`); the satellite
records this pin here. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination
of the language-query leg.

This pin lives in package `language`, not `codequery`:
`language.Handler` mounts the route and `codequery` cannot mount it
without an import cycle (`codequery/handler.go` records why).

## Shape

Version: `CodeLanguageQueryReadAdmissionContractVersion = "v0"`
(go/internal/query/language/code_query_read_admission.go).

Ceiling 4 re-declared (never embedded) methods — the bounded graph
traversal (`Run`) for the graph-backed, graph-first-content, and
directory branches, plus the three content reads for the
content-only branch and the two enrichments — sealed, with
`RunSingle` and every other graph method OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | graph-backed, graph-first-content, and directory branches |
| 2 | `SearchEntitiesByLanguageAndType(context.Context, string, string, string, string, int) ([]querycontract.EntityContent, error)` | staying content store | content-only branch reads |
| 3 | `SearchEntitiesByLanguageAndTypeForAccess(context.Context, taxonomy.LanguageEntitySearch) ([]querycontract.EntityContent, error)` | grant-scoped | scoped content-only branch reads |
| 4 | `ListEntityMetadataByIDsForAccess(context.Context, taxonomy.LanguageEntityIDLookup) ([]querycontract.EntityContent, error)` | grant-scoped (anchored metadata lookup) | per-page metadata enrichment of graph rows |

All four methods are IN because production satisfies all four: the
mirror compiles against the staying handler-facing ports, the
production content reader implements the three content reads
(proven at mint time), and the graph reader serves `Run`.

There is deliberately no constructor, builder, selector, or
ReadStore surface on this pin: the satellite satisfies this port
with its own wiring; this pin carries the read shape only.

Enforcement (all in go/internal/query/language):

- `code_query_read_admission_contract_test.go` — frozen method-set
  test plus version pin
  (`TestCodeLanguageQueryReadAdmissionContractIsVersioned` —
  exactly 4 methods), with the satisfiability witness and the seeded
  re-signed rejection
  (`TestCodeLanguageQueryReadAdmissionRejectsResignedMethod`).
- `code_query_read_admission_wire_shapes_test.go` —
  (`TestCodeLanguageQueryReadAdmissionWireShapesAreSatelliteConsumable`)
  proves a satellite can consume the pin using only the pin plus the
  `querycontract` and `taxonomy` shapes.

## Census (per route per method)

| Method | `POST /api/v0/code/language-query` |
|---|---|
| `Run` | graph-backed, graph-first-content, and directory branches |
| `SearchEntitiesByLanguageAndType` | content-only branch and the two enrichments |
| `SearchEntitiesByLanguageAndTypeForAccess` | scoped content-only branch (fallback differs) |
| `ListEntityMetadataByIDsForAccess` | anchored metadata enrichment of graph rows (page-clamped) |

Reachability notes, recorded not silently assumed: the entity-type
gate answers before the grant short-circuit (an unsupported type is
a 400 for every caller, granted or not); the empty grant is the
`noBackendReadSourceBackend` empty page with no backend read (the
same shared reason constant the imports/investigate grantless page
uses, so the two pages cannot be reworded apart); the caller page
clamp (200) is coupled to `taxonomy.MaxLanguageEntityIDLookup`
(also 200) — the reader refuses a larger id set rather than
truncating it; the graphless or zero-row graph read falls through
to the content-store fallback.

## Explicitly OUT (not widened by this pin)

- `RunSingle` and every other graph method: this pin adopts `Run`
  only.
- Every other `ContentStore` method (name, exact-name, component,
  file, catalog reads): a new shared-port method is adopted only by
  deliberate re-declaration plus a frozen-list update.
- The grant checks, the entity-type-family dispatch, the per-repo
  fallback loop, the page-read fallback, the fixed failure messages,
  and `QueryProfile` are handler behavior, not port shape.
- No constructor, builder, selector, or ReadStore surface.

## Satellite consumption

Depend on the shape, not a constructor: serve the route's reads
through the concurrent-safe graph reader plus the guarded Postgres
content reader the way `cmd/api` wiring builds them today,
preserving the entity-type gate order, the page clamp, and the
empty-grant page exactly. A satellite PR consuming this pin for any
other backend pairing is invalid without arbiter review.
