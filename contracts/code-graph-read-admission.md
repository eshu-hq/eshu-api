# CodeGraphReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. Wave-7 G-a core pin, per the arbiter Wave-7
ruling on eshu-hq/eshu#9669 (SPLIT, APPROVED for exactly the G-a 4:
`POST /api/v0/code/call-chain`, `/call-chain/compare`,
`/call-graph/metrics`, `/routes/callers`). Minted core-side at
`0fd6db6` (merge of `eshu-hq/eshu#10645`); the satellite records this
pin in a follow-up. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination of
the G-a legs.

## Shape

Version: `CodeGraphReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_graph_read_admission.go).

Ceiling 3 re-declared (never embedded) methods — the bounded graph
traversal (`Run`, `RunSingle`) plus the entity-name resolution
(`SearchEntitiesByName`) — sealed, with every other `GraphQuery` and
`ContentStore` leg OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | `POST /api/v0/code/call-chain` (bounded traversal); `POST /api/v0/code/call-chain/compare` (one-hop expansion reads); `POST /api/v0/code/call-graph/metrics` (bounded edge scan); `POST /api/v0/code/routes/callers` (endpoint, handler, and directional traversal reads) |
| 2 | `RunSingle(context.Context, string, map[string]any) (map[string]any, error)` | `querycontract.GraphQuery` | same four routes (single-row graph reads on the same legs) |
| 3 | `SearchEntitiesByName(context.Context, string, string, string, int) ([]querycontract.EntityContent, error)` | `querycontract.ContentStore` | `POST /api/v0/code/call-chain` (endpoint name resolution, incl. the cross_repo start/end selectors); `POST /api/v0/code/call-chain/compare` (endpoint name resolution); `POST /api/v0/code/routes/callers` (label resolution behind the traversal) |

There is deliberately no `WithReadStore` constructor on this pin:
the G-a seam spans the concurrent-safe graph reader plus the guarded
Postgres content reader, and minting one constructor would pick a
wiring the satellite never runs. The satellite keeps building both
backends the way `cmd/api` wiring builds them today; the pin freezes
only the read shape the handlers may call through. The compile-time
witness assertion in the pin file
(`var _ CodeGraphReadAdmission = (*codeGraphReadAdmissionMirror)(nil)`,
embedding `querycontract.GraphQuery` plus `querycontract.ContentStore`)
proves the pair the handlers run against today satisfies the pin.

Enforcement (all in go/internal/query/codequery):

- `code_graph_read_admission_contract_test.go` — frozen method-set
  test plus version pin
  (`TestCodeGraphReadAdmissionContractIsVersioned` — exactly 3
  methods: `Run`, `RunSingle`, `SearchEntitiesByName`), with the
  satisfiability witness
  (`codeGraphReadAdmissionWitness` must implement the pin) and the
  seeded rejection RED half
  (`TestCodeGraphReadAdmissionRejectsResignedMethod` — a witness
  with a re-signed `RunSingle` MUST NOT implement the pin).

## Census (per route per method)

| Method | `call-chain` | `call-chain/compare` | `call-graph/metrics` | `routes/callers` |
|---|---|---|---|---|
| `Run` | bounded shortest-chain traversal | one-hop BFS expansion reads | bounded edge scan (50k-edge sentinel) | endpoint + handler + directional traversal reads |
| `RunSingle` | single-row graph reads on the same leg | single-row graph reads on the same leg | single-row metric reads | single-row reads on the same legs |
| `SearchEntitiesByName` | endpoint name resolution (incl. `start_repo_id` / `end_repo_id` cross_repo selectors) | endpoint name resolution (single-repo) | — (metric rows carry no name lookup; OUT by construction) | handler label resolution behind the traversal |

Reachability notes, recorded not silently assumed: the metrics leg
runs no name lookup by construction; the compare leg refuses
`cross_repo` (single-repo v1: `repo_id` required); the call-chain leg
is the only leg that admits the cross_repo endpoint pair; the
callers leg resolves the handler label through the content reader
behind the traversal.

Every other `GraphQuery` and `ContentStore` shape is OUT: point and
list reads, file-content legs, ledger/summary/snapshot surface, and
every other `Search*`/`List*`/`Get*` leg — other families' standing,
unchanged.

## Explicitly OUT (not widened by this pin)

- relationships/story (G-b, `eshu-hq/eshu-api#122`): the story
  resolution and evidence reads stay outside this pin.
- complexity/quality, bundles, cypher/visualize: sibling-family reads
  with their own contracts.
- structure/inventory, language-query, and graph/entities (W8b #101):
  sibling surfaces, never adopted here.
- Every other `GraphQuery`/`ContentStore` method: this pin adopts the
  three listed reads only; a new shared-port method is adopted only
  by deliberate re-declaration plus a frozen-list update.
- Grant checks (`codeGrantAccessFilter`, `codeContentGrantScope`),
  the unsupported-language short-circuit, the fixed failure messages,
  endpoint disambiguation, and `QueryProfile` (handler behavior, not
  port shape: there is no method set to freeze).

## Satellite consumption

Depend on the shape, not a constructor: serve the four routes' reads
through the existing graph reader (satisfying `Run`/`RunSingle`)
plus the guarded Postgres content reader (satisfying
`SearchEntitiesByName`), the same pair `cmd/api` wiring builds
today. A satellite PR consuming this pin for any other backend pair
is invalid without arbiter review.
