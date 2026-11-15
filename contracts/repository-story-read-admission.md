# RepositoryStoryReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W5f story-single core pin, per the supplemental
arbiter ruling on eshu-hq/eshu-api#97. Minted core-side at `76240fda9f`
(merge of `eshu-hq/eshu#10705`); the satellite records this pin here. A
satellite PR consuming an unminted pin is invalid. The mint-diff CONFIRM
precedes satellite termination of the story leg.

## Shape

Version: `RepositoryStoryReadAdmissionContractVersion = "v0"`
(go/internal/query/repository/read_admission.go).

Ceiling 15 re-declared (never embedded) methods — the two bounded graph
traversals, the eight story-closure content reads, the CI/CD run
correlation read, and the four narrow read-model facets — sealed:

| # | Method | Mirrors | Serves (per pin closure) |
|---|---|---|---|
| 1 | `Run` | `querycontract.GraphQuery` | story/dependency/deployment-evidence/artifact traversals; selector fallback |
| 2 | `RunSingle` | `querycontract.GraphQuery` | repository base row |
| 3 | `MatchRepositories` | `ContentStore` | path selector |
| 4 | `RepositoryCoverage` | `ContentStore` | coverage stage |
| 5 | `ListRepoEntities` | `ContentStore` | semantic stage |
| 6 | `ListRepoFiles` | `ContentStore` | semantic stage |
| 7 | `ListRepoEntitiesByTypes` | `ContentStore` | infrastructure panel |
| 8 | `GetFileContent` | `ContentStore` | narrative hydration |
| 9 | `GetFileContents` | `ContentStore` | artifact/workflow hydration |
| 10 | `NonBlankRepoFilePaths` | `ContentStore` | config needed-content check |
| 11 | `ListCICDRunCorrelations` | `CICDRunCorrelationStore` | ci/cd evidence stage |
| 12 | `RepositoryReadModelSummary` | narrow read-model facet | graph summary |
| 13 | `ServiceStoryTargetSupportEvidence` | narrow read-model facet | target support |
| 14 | `DocumentationFindings` | narrow read-model facet | target documentation |
| 15 | `RepositoryDeploymentEvidence` | narrow read-model facet | deployment-evidence fast path |

There is deliberately no constructor on this pin: the story seam spans
the graph reader, the content reader, and the reducer-owned run
correlation store, and minting one constructor would pick a wiring the
satellite never runs. The pin freezes only the read shape.

Enforcement (all in go/internal/query/repository, plus the query-level
mirror):

- `read_admission_contract_test.go` — frozen method-set test plus
  version pin (exactly 15 methods), the satisfiability witness, the
  seeded re-signed rejection, and the widening-is-caught proof.
- `read_admission_wire_shapes_test.go` — the pin-satisfying canned store
  proves a satellite can consume the pin using only the pin plus the
  querycontract and repository-contract shapes.
- `go/internal/query/w5_repository_story_read_admission_mirror.go` —
  compile-time proof that the production stores jointly serve the pin.

## Census

In scope — the W5f story single ONLY:

- `GET /api/v0/repositories/{repo_id}/story`

The impact packet single rides its own packet pin and neither route may
cite the other's pin. Minting a whole-Repositories pin to cover this
route would retroactively widen D4, which stays forbidden: the D4
transport record covers the 11 terminated repository GETs and explicitly
excludes story.

## Explicitly OUT (not widened by this pin)

- `ResolveRepository` and `ListRepositories`: the story resolves its
  repository by RunSingle on the base row plus MatchRepositories in the
  path selector; the fallback belongs to the stats, tree, freshness,
  and branches legs, and list belongs to the list leg.
- `RepositoryContextCoverage`: the story coverage stage calls
  RepositoryCoverage directly; the narrow-facet assertion lives in the
  context route's loadRepositoryContentCoverage, not on the story path.
- `GetEntityContent`, the Search* family beyond MatchRepositories,
  `ListRepoEntitiesByKeys`, `ListRepoEntitiesByIDs`,
  `ListRepoEntitiesByPaths`, `ListRepoK8sSelectCandidates`,
  `ListFrameworkRoutes`, the language aggregates,
  `MatchRepositoriesByIDs`, `RepositoryFilesLastIndexedAt`,
  `SearchEntitiesByLanguageAndType`, `GetFileLines`: no story stage
  calls them.
- Freshness, Selections, Retirements, and ServiceCatalog correlation
  reads: list and catalog legs, not story.
- The repositoryartifacts loader internals, the responder and writer
  seams, and the fixed failure messages: handler behavior, proven at
  byte-parity time and never frozen here.
- No suppression leg exists on this route; the Decision 2
  suppression-state-filtering parity obligation stays on the
  supply-chain packet leg this pin does not touch.

## Satellite consumption

Depend on the shape, not a constructor: serve the story reads through
the existing graph reader, content reader, and run correlation store,
the same backends `cmd/api` wiring builds today. Rollback: revert this
pin. The seam is unwired — no handler, route, or caller depends on it
yet — so rollback moves no traffic and changes no data.
