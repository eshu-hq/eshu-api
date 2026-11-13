# CodeDeadCodeReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. Dead-code content-fallback core pin, per the
arbiter SUPPLEMENTAL-PIN-OWED ruling on eshu-hq/eshu-api#124
(APPROVED for exactly the dead-code 3:
`POST /api/v0/code/dead-code`, `/dead-code/cross-repo`,
`/dead-code/investigate`). Minted core-side at
`589d55e` (merge of `eshu-hq/eshu#10648`); the satellite records this
pin in a follow-up. A satellite PR consuming an unminted pin is
invalid. Arbiter review of this pin precedes satellite termination of
the dead-code legs.

## Shape

Version: `CodeDeadCodeReadAdmissionContractVersion = "v0"`
(go/internal/query/code_dead_code_read_admission.go).

Ceiling 14 re-declared (never embedded) methods — the graph run
behind the SQL-routine EXECUTES probes, the candidate/incoming/
downgrade reads behind the investigation and scan, the entity
hydration reads, the coverage and index-freshness reads, the three
cross-repo consumer-evidence reads, plus the boundary-evidence read —
sealed, with `RunSingle` OUT by construction:

| # | Method | Mirrors | Serves |
|---|--------|---------|--------|
| 1 | `DeadCodeCandidateRows(context.Context, codeshaping.DeadCodeCandidateQuery) ([]map[string]any, error)` | `ContentReader` | `POST /api/v0/code/dead-code` (candidate scan); `POST /api/v0/code/dead-code/cross-repo` (producer scan); `POST /api/v0/code/dead-code/investigate` (candidate scan) |
| 2 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | `POST /api/v0/code/dead-code` (SQL-routine reachability probe); `POST /api/v0/code/dead-code/cross-repo` and `/investigate` (scoped incoming-edge batch probes behind the candidate scan) |
| 3 | `GetEntityContents(context.Context, []string) (map[string]*EntityContent, error)` | `ContentReader` | all three routes (candidate hydration) |
| 4 | `GetEntityContent(context.Context, string) (*EntityContent, error)` | `ContentReader` | all three routes (single-entity hydration) |
| 5 | `DowngradedCodeRootKinds(context.Context, string, []string) (map[string]map[string]struct{}, error)` | `ContentReader` | `POST /api/v0/code/dead-code` and `/investigate` (modeled-root downgrade) |
| 6 | `DeadCodeIncomingEntityIDs(context.Context, string, []string) (map[string]code.DeadCodeIncomingEdge, error)` | `ContentReader` | `POST /api/v0/code/dead-code` and `/investigate` (incoming edges) |
| 7 | `CodeReachabilityIncomingEntityIDs(context.Context, string, []string, []string) (map[string]code.DeadCodeIncomingEdge, error)` | `ContentReader` | all three routes (grant-bound incoming probe) |
| 8 | `RepositoryContextCoverage(context.Context, string) (RepositoryContentCoverage, error)` | `ContentReader` | `POST /api/v0/code/dead-code/investigate` (cheap coverage) |
| 9 | `RepositoryFilesLastIndexedAt(context.Context, string) (time.Time, error)` | `ContentReader` | `POST /api/v0/code/dead-code/investigate` (freshness) |
| 10 | `RepositoryCoverage(context.Context, string) (RepositoryContentCoverage, error)` | `ContentReader` | `POST /api/v0/code/dead-code/investigate` (full aggregate) |
| 11 | `RepositoryRelationshipReadModel(context.Context, string) (RepositoryRelationshipReadModel, error)` | `ContentReader` | `POST /api/v0/code/dead-code/cross-repo` (boundary evidence) |
| 12 | `CrossRepoDeadCodeConsumerEvidence(context.Context, string, []string, code.CrossRepoDeadCodeConsumerReads) (map[string][]deadcode.CrossRepoDeadCodeEvidence, code.CrossRepoDeadCodeHiddenConsumers, error)` | `ContentReader` | `POST /api/v0/code/dead-code/cross-repo` (consumer evidence) |
| 13 | `CrossRepoDeadCodeConsumerCoverage(context.Context, code.CrossRepoDeadCodeCoverageRequest) (code.CrossRepoDeadCodeCoverage, error)` | `ContentReader` | `POST /api/v0/code/dead-code/cross-repo` (consumer coverage) |
| 14 | `CrossRepoDeadCodeConsumerRootPaths(context.Context, []string, []string) (map[string]string, error)` | `ContentReader` | `POST /api/v0/code/dead-code/cross-repo` (root paths) |

There is deliberately no `WithReadStore` constructor on this pin:
the dead-code seam spans the concurrent-safe graph reader plus the
guarded Postgres content reader, and minting one constructor would pick
a wiring the satellite never runs. The satellite keeps building both
backends the way `cmd/api` wiring builds today; the pin freezes only
the read shape the handlers may call through. The compile-time witness
assertion in the pin file
(`var _ CodeDeadCodeReadAdmission = codeDeadCodeReadAdmissionWitness{}`,
embedding `querycontract.GraphQuery` plus `*ContentReader`) proves the
pair the handlers run against today satisfies the pin.

Enforcement (all in go/internal/query):

- `code_dead_code_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeDeadCodeReadAdmissionContractIsVersioned` — exactly 14
  methods in the listed order), with the satisfiability witness and
  the seal test holding the ledger/summary/snapshot exclusions.

## Census (per route per method)

| Method | `dead-code` scan | `dead-code/cross-repo` | `dead-code/investigate` |
|---|---|---|---|
| `DeadCodeCandidateRows` | candidate scan (all labels) | producer scan (repo-scoped) | candidate scan |
| `Run` | SQL-routine EXECUTES probe + scoped incoming batch probe | scoped incoming batch probe behind the producer scan | scoped incoming batch probe behind the scan |
| `GetEntityContents` / `GetEntityContent` | candidate hydration | candidate hydration | candidate hydration |
| `DowngradedCodeRootKinds` | modeled-root downgrade | — (single-repo evidence only; OUT by construction) | modeled-root downgrade |
| `DeadCodeIncomingEntityIDs` | incoming edges | — (grant-bound probe covers it) | incoming edges |
| `CodeReachabilityIncomingEntityIDs` | grant-bound incoming probe | grant-bound incoming probe | grant-bound incoming probe |
| `RepositoryContextCoverage` / `RepositoryFilesLastIndexedAt` / `RepositoryCoverage` | — (investigate-only coverage; OUT by construction) | — | cheap freshness + full aggregate |
| `CrossRepoDeadCodeConsumerEvidence` / `CrossRepoDeadCodeConsumerCoverage` / `CrossRepoDeadCodeConsumerRootPaths` | — (cross-repo-only; OUT by construction) | consumer evidence, coverage, root paths | — |
| `RepositoryRelationshipReadModel` | — | boundary evidence | — |

Reachability notes, recorded not silently assumed: the scan leg runs
no consumer-evidence or coverage reads by construction; the
cross-repo leg runs no downgrade or single-repo incoming read by
construction; the investigate leg runs no cross-repo evidence reads
by construction; the SQL-routine probe admits only `Run`, never
`RunSingle`.

Every other `GraphQuery` and `ContentReader` shape is OUT: `RunSingle`,
point and list reads, file-content legs, `Search*`/`List*` legs,
ledger/summary/snapshot surface, and every other `Get*` leg — other
families' standing, unchanged.

## Explicitly OUT (not widened by this pin)

- The single-row graph read (`RunSingle`) stays unpinned here: the
  dead-code lane never issues one.
- The search legs (`SearchEntityContent`, `SearchCodeCandidates`, the
  AnyRepo and exact-name twins) and the file-content point reads keep
  the CodeSearch, Content search, and D13 point pins; there is no
  search or file method in the frozen set.
- The list legs (`ListRepoEntities` and the by-type/by-path/by-ID
  twins, `ListRepoFiles`) stay unpinned here for the same reason.
- Grant checks (`codeGrantAccessFilter`, `codeContentGrantScope`),
  the repository selector, the fixed failure messages, the rerank, and
  `QueryProfile` (handler behavior, not port shape: there is no method
  set to freeze).
- No ledger, `RepositoryReadModelSummary` stored-summary, or snapshot
  surface is pinned here; the seal test holds those exclusions. The
  boundary-evidence read lives in the in-scope list above.

## Satellite consumption

Depend on the shape, not a constructor: serve the three routes' reads
through the existing graph reader (satisfying `Run`) plus the guarded
Postgres content reader (satisfying the thirteen content reads), the
same pair `cmd/api` wiring builds today. A satellite PR consuming this
pin for any other backend pair is invalid without arbiter review.
