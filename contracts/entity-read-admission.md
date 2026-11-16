# EntityReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W5 Mint A extended core pin, per the supplemental
arbiter ruling on eshu-hq/eshu-api#97, plus the C5 27->30 extension
(merge of `eshu-hq/eshu#10747`, recorded here at core `9bf29e53` with
zero drift since that merge). Minted core-side at `76240fda9f`
(merge of `eshu-hq/eshu#10705`); the satellite records this pin here. A
satellite PR consuming an unminted pin is invalid. The mint-diff CONFIRM
precedes satellite termination of the entity legs.

## Shape

Version: `EntityReadAdmissionContractVersion = "v0"`
(go/internal/query/entity/read_admission.go).

Ceiling 30 re-declared (never embedded) methods — the two bounded graph
traversals, the nineteen ContentStore reads the family closure proves
exercised, the three reducer-store reads, the type-asserted
EntityNameSearcher facet, the content-reference search facet, and the
four narrow read-model facets — sealed:

| # | Method | Mirrors | Serves (per pin closure) |
|---|---|---|---|
| 1 | `Run` | `querycontract.GraphQuery` | all 9 (bounded traversal, selector fallback, workload-candidate resolution) |
| 2 | `RunSingle` | `querycontract.GraphQuery` | all 9 (bounded single traversal) |
| 3 | `GetEntityContent` | `ContentStore` | entity resolution; graph enrichment |
| 4 | `GetFileContent` | `ContentStore` | LoadServiceQueryEvidence hydration; OpenAPI $ref resolution |
| 5 | `GetFileContents` | `ContentStore` | artifact hydration |
| 6 | `NonBlankRepoFilePaths` | `ContentStore` | config needed-content check |
| 7 | `SearchEntitiesByName` | `ContentStore` | name-target resolution |
| 8 | `SearchEntitiesByNameAnyRepo` | `ContentStore` | any-repo name resolution |
| 9 | `SearchEntitiesReferencingComponent` | `ContentStore` | content-fallback relationship build (reads; builder itself OUT) |
| 10 | `SearchFileContentAnyRepo` | `ContentStore` | consumer-evidence search |
| 11 | `SearchFileContentAnyRepoExactCase` | `ContentStore` | consumer-evidence search (exact case) |
| 12 | `ResolveRepository` | `ContentStore` | selector ResolveExactForAccess fallback |
| 13 | `ListRepoEntities` | `ContentStore` | provisioning chains |
| 14 | `ListRepoEntitiesByKeys` | `ContentStore` | keyed hydration |
| 15 | `ListRepoEntitiesByIDs` | `ContentStore` | id hydration |
| 16 | `ListRepoEntitiesByType` | `ContentStore` | content-fallback relationship build (reads; builder itself OUT) |
| 17 | `MatchRepositories` | `ContentStore` | selector ResolveExactForAccess |
| 18 | `ListRepositories` | `ContentStore` | selector ResolveExactForAccess |
| 19 | `ListRepoFiles` | `ContentStore` | CI/CD static-workflow evidence; service deployment-evidence stage |
| 20 | `ListFrameworkRoutes` | `ContentStore` | enrichment framework-routes stage |
| 21 | `SearchEntityNames` | `EntityNameSearcher` facet | global resolve path |
| 22 | `ListCICDRunCorrelations` | `CICDRunCorrelationStore` | LoadRepositoryScopedCICDEvidence on the workload-story path |
| 23 | `ListContainerImageIdentities` | `supplychain.ContainerImageIdentityStore` | service-story supply chain |
| 24 | `ListSBOMAttestationAttachments` | `supplychain.SBOMAttestationAttachmentStore` | service-story supply chain |
| 25 | `ServiceStoryTargetSupportEvidence` | narrow read-model facet | service-story operation |
| 26 | `DocumentationFindings` | narrow read-model facet | service-story operation |
| 27 | `RepositoryDeploymentEvidence` | narrow read-model facet | deployment-evidence fast path |
| 28 | `SearchFileReferenceAnyRepo` | `ContentStore` (indexed scan, wider bounds) | consumer-evidence search; investigations service leg |
| 29 | `ListRepoEntitiesByTypes` | `ContentStore` | repository infrastructure panel; investigations service leg |
| 30 | `RepositoryReadModelSummary` | narrow read-model facet | read-model identity fallback; investigations service leg |

There is deliberately no constructor on this pin: the entity seam spans
the guarded Postgres content reader, the concurrent-safe graph reader,
and the reducer-owned stores, and minting one constructor would pick a
wiring the satellite never runs. The satellite keeps building its
backends the way `cmd/api` wiring builds them today; the pin freezes
only the read shape the handlers may call through.

Enforcement (all in go/internal/query/entity, plus the query-level mirror):

- `read_admission_contract_test.go` — frozen method-set test plus
  version pin (`TestEntityReadAdmissionContractIsVersioned` — exactly 30
  methods), the satisfiability witness, the seeded re-signed rejection,
  and the widening-is-caught proof.
- `read_admission_wire_shapes_test.go` — the pin-satisfying canned store
  proves a satellite can consume the pin using only the pin plus the
  querycontract, taxonomy, repository-contract, and supply-chain shapes.
- `go/internal/query/w5_entity_read_admission_mirror.go` —
  compile-time proof that the production stores jointly serve the pin.

## Census (per route per method, per the pin closure)

In scope — the 9 W5 entity routes (Mint A extended; the 9th route is
downstream-consumers, per the mint message):

- `POST /api/v0/entities/resolve`
- `GET /api/v0/entities/{entity_id}/context`
- `GET /api/v0/workloads/{workload_id}/context`
- `GET /api/v0/workloads/{workload_id}/story`
- `GET /api/v0/services/{service_name}/context`
- `GET /api/v0/services/{service_name}/story`
- `GET /api/v0/services/{service_name}/api-surface/endpoints`
- `GET /api/v0/services/{service_name}/downstream-consumers`
- `GET /api/v0/graph/entities`

PLUS the W5c-e impact deployment-trace enrichment leg by explicit
dual-pin cite: `trace-deployment-chain` and `deployment-config-influence`
require this pin alongside ImpactReadAdmission for their TraceContext
leg (see contracts/impact-read-admission.md). Silent reliance on the
Entity pin is forbidden; this cite is the reliance, recorded here and
there. No other route may cite this pin as coverage.

The graph pair serves all 9 routes; the seventeen content reads serve
the resolution, selector, evidence, enrichment, fallback-read, and
service-tail legs the pin comment enumerates; the reducer three plus
the read-model three serve the workload-story and service-story legs.
Service and workload story ride this mint; the story served-spec
bijection (TestSpecScope, evidence_boundaries present for
get_workload_story and absent for get_service_story per
EvidenceBoundariesFor at the minted HEAD) closes at satellite-record
time, not in this pin.

## Explicitly OUT (not widened by this pin)

- `ContentRelationshipBuilder.BuildContentRelationships` (the GET
  entities/{entity_id}/context fallback): the production satisfier is
  now the satellite-consumable `entity/content/fallback.Builder` (merged
  core-side via #10745 + #10779). The METHOD stays out by construction —
  do not freeze a builder method on this read pin — while its two
  exercised reads (SearchEntitiesReferencingComponent,
  ListRepoEntitiesByType) are frozen here, so a satisfier built to this
  pin serves the fallback leg's reads. Wave-2 terminates the leg on
  this pin plus the merged C2 leaf executing core-side.
- The optional image-candidate explainer facet
  (ExplainContainerImageCandidate): StoryImageCandidateMissingExplanation
  degrades to a generic detail when the store does not implement it.
- The RepositoryContextCoverage and RepositoryFilesLastIndexedAt facets:
  the story coverage stage calls RepositoryCoverage directly.
- GraphBackend, Profile, Logger, Instruments, and the fixed failure
  messages are handler behavior, not port shape.
- The grant checks (RepositoryAccessFilterFromContext plus the scoped
  grant comparisons) and scoped/empty-grant parity are proven per route
  at byte-parity time, not frozen here.
- No suppression leg exists on these routes; the Decision 2
  suppression-state-filtering parity obligation stays on the
  supply-chain finding/reconciliation legs this pin does not touch.

## Satellite consumption

Depend on the shape, not a constructor: serve the entity reads through
the existing graph reader plus the guarded Postgres content reader plus
the reducer-owned stores, the same backends `cmd/api` wiring builds
today. A satellite PR consuming this pin for any other backend pairing
is invalid without arbiter review. Rollback: revert this pin. The seam
is unwired — no handler, route, or caller depends on it yet — so
rollback moves no traffic and changes no data.
