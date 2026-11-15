# CodeGraphScanReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W6b 4 core pin, per the arbiter supplemental
ruling on eshu-hq/eshu-api#98 (§2A, SPLIT, APPROVED for exactly the
W6b 4: `POST /api/v0/code/complexity`, `/api/v0/code/quality/inspect`,
`/api/v0/code/imports/investigate`, `/api/v0/code/bundles`). Minted
core-side at `81b756fd84` (merge `eshu-hq/eshu#10688`); the satellite
records this pin here. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination of
the W6b legs.

## Shape

Version: `CodeGraphScanReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_graph_scan_read_admission.go).

Ceiling 2 re-declared (never embedded) methods — the bounded graph
traversal (`Run`) plus the by-ID entity hydration
(`ListRepoEntitiesByIDs`) — sealed:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | all four POSTs (bounded traversal); complexity builders (by-name, by-ID, ranked list); quality inspect scan; imports leaf dispatch; bundles anchor plus version counts |
| 2 | `ListRepoEntitiesByIDs(context.Context, string, []string, int) ([]querycontract.EntityContent, error)` | `querycontract.ContentStore` | complexity by-ID branch enrichment (the rows the by-ID branch merges) |

There is deliberately no constructor on this pin: the W6b seam spans
the concurrent-safe graph reader plus the guarded Postgres content
reader, and minting one constructor would pick a wiring the satellite
never runs. The satellite keeps building both backends the way
`cmd/api` wiring builds them today; the pin freezes only the read
shape the handlers may call through. The compile-time witness
assertion in the pin file
(`var _ CodeGraphScanReadAdmission = (*codeGraphScanReadAdmissionMirror)(nil)`,
embedding `querycontract.GraphQuery` plus
`querycontract.ContentStore`) proves the pair the handlers run
against today satisfies the pin.

Enforcement (all in go/internal/query/codequery):

- `code_graph_scan_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeGraphScanReadAdmissionContractIsVersioned` — exactly 2
  methods: `Run`, `ListRepoEntitiesByIDs`), with the satisfiability
  witness (minimal shape must implement the pin) and the seeded
  rejection RED half
  (`TestCodeGraphScanReadAdmissionRejectsResignedMethod` — a witness
  with a re-signed method MUST NOT implement the pin).
- `code_graph_scan_read_admission_wire_shapes_test.go` — the
  pin-satisfying canned store
  (`TestCodeGraphScanReadAdmissionWireShapesAreSatelliteConsumable`)
  proves a satellite can consume the pin using only the pin plus the
  `querycontract` shapes: both pinned reads answer through a
  `CodeGraphScanReadAdmission`-typed value, the by-ID enrichment
  hydrates the wide `EntityContent` row (the keys the complexity
  by-ID branch merges), and an empty id set answers with no rows and
  no error — the fail-closed emptiness the grantless page relies on.

## Census (per route per method)

| Method | `POST /api/v0/code/complexity` | `POST /api/v0/code/quality/inspect` | `POST /api/v0/code/imports/investigate` | `POST /api/v0/code/bundles` |
|---|---|---|---|---|
| `Run` | by-name, by-ID, and ranked-list builders | the inspect scan (grantless ⇒ no graph call) | imports leaf dispatch | anchor plus version counts |
| `ListRepoEntitiesByIDs` | by-ID branch enrichment hydration | — (no hydration leg; OUT by construction) | — (no hydration leg; OUT by construction) | — (no hydration leg; OUT by construction) |

Reachability notes, recorded not silently assumed: the complexity
list branch runs only when neither `entity_id` nor `function_name`
is supplied; the by-ID enrichment hydrates only the by-ID branch;
the imports grantless branch answers the empty page with no read;
the bundles grantless branch answers the zero-row page with no
graph call, and a scoped caller reads only `visibility = 'public'`
packages.

## Explicitly OUT (not widened by this pin)

- `RunSingle` and every other `GraphQuery` method: no W6b route
  issues a single-row graph read.
- cypher/visualize (caller-supplied programs, own W6d pin),
  structure/inventory and topics/investigate (sibling W6c pin),
  secrets/investigate (isolated W6c least-privilege pin),
  language-query (W6c language-package pin).
- Every other `ContentStore` method, including `GetEntityContent`
  (the by-ID enrichment other routes use; the scan lane hydrates
  through `ListRepoEntitiesByIDs`): a new shared-port method is
  adopted only by deliberate re-declaration plus a frozen-list
  update.
- The grant checks, the grant-bound builders, the repository
  selector, the fixed failure messages, and `QueryProfile` are
  handler behavior, not port shape: there is no method set to
  freeze.

## Satellite consumption

Depend on the shape, not a constructor: serve the four routes'
reads through the existing graph reader plus the guarded Postgres
content reader, the same pair `cmd/api` wiring builds today. A
satellite PR consuming this pin for any other backend pair is
invalid without arbiter review.
