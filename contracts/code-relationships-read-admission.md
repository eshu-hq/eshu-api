# CodeRelationshipReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. G-b 2 core pin, per the arbiter SUFFICIENT
ruling on eshu-hq/eshu-api#122 (grant reads IN). Minted core-side at
`8ee32698` (merge of `eshu-hq/eshu#10669`); the satellite records this
pin here. A satellite PR consuming an unminted pin is invalid. The
mint-diff CONFIRM precedes satellite termination of the G-b legs.

## Shape

Version: `CodeRelationshipReadAdmissionContractVersion = "v0"`
(go/internal/query/codequery/code_relationships_read_admission.go).

Ceiling 17 re-declared (never embedded) methods — the bounded graph
traversals (`Run`, `RunSingle`), the twelve ContentStore reads, and
the three grant-scoped reads — sealed:

| # | Method | Mirrors | Serves |
|---|---|--------|--------|
| 1 | `Run(context.Context, string, map[string]any) ([]map[string]any, error)` | `querycontract.GraphQuery` | both POSTs (bounded graph traversal) |
| 2 | `RunSingle(context.Context, string, map[string]any) (map[string]any, error)` | `querycontract.GraphQuery` | both POSTs (bounded single traversal) |
| 3 | `GetEntityContent(context.Context, string) (*querycontract.EntityContent, error)` | `ContentStore` | story target resolution; graph enrichment |
| 4 | `SearchEntitiesByName(context.Context, string, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | name-target resolution; builder closure |
| 5 | `SearchEntitiesByNameAnyRepo(context.Context, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | name-target resolution any-repo |
| 6 | `SearchEntitiesByExactName(context.Context, string, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | name-target exact resolution |
| 7 | `SearchEntitiesByExactNameAnyRepo(context.Context, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | name-target exact any-repo |
| 8 | `SearchEntitiesByLanguageAndType(context.Context, string, string, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | language story resolution |
| 9 | `SearchEntitiesReferencingComponent(context.Context, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | builder closure; component back-refs |
| 10 | `ListRepoFiles(context.Context, string, int) ([]querycontract.FileContent, error)` | `ContentStore` | builder closure |
| 11 | `ListRepoEntitiesByType(context.Context, string, string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | builder closure |
| 12 | `ListRepoEntitiesByIDs(context.Context, string, []string, int) ([]querycontract.EntityContent, error)` | `ContentStore` | hydration |
| 13 | `ListRepositories(context.Context) ([]querycontract.RepositoryCatalogEntry, error)` | `ContentStore` | repo-identity hydration |
| 14 | `MatchRepositories(context.Context, string) ([]querycontract.RepositoryCatalogEntry, error)` | `ContentStore` | repo-identity match |
| 15 | `GetEntityContentInRepositories(context.Context, string, []string) (*querycontract.EntityContent, error)` | grant-scoped (`*ContentReader`) | scoped callers (fail-closed) |
| 16 | `SearchEntitiesByNameInRepositories(context.Context, []string, string, string, int) ([]querycontract.EntityContent, error)` | grant-scoped (`*ContentReader`) | scoped callers (fail-closed) |
| 17 | `SearchEntitiesByLanguageAndTypeForAccess(context.Context, taxonomy.LanguageEntitySearch) ([]querycontract.EntityContent, error)` | grant-scoped (`taxonomy.LanguageEntityContentSearcher`) | scoped language-story (fallback differs) |

There is deliberately no constructor on this pin: the G-b seam spans
the guarded Postgres content reader plus the concurrent-safe graph
reader, and minting one constructor would pick a wiring the satellite
never runs. The satellite keeps building both backends the way
`cmd/api` wiring builds them today; the pin freezes only the read
shape the handlers may call through.

Enforcement (all in go/internal/query/codequery, plus the query-level
mirror):

- `code_relationships_read_admission_contract_test.go` — frozen
  method-set test plus version pin
  (`TestCodeRelationshipReadAdmissionContractIsVersioned` — exactly 17
  methods), with the satisfiability witness (minimal shape must
  implement the pin), the seeded re-signed rejection
  (`TestCodeRelationshipReadAdmissionRejectsResignedMethod`), and the
  widening-is-caught proof.
- `code_relationships_read_admission_wire_shapes_test.go` — the
  pin-satisfying canned store proves a satellite can consume the pin
  using only the pin plus the querycontract and taxonomy shapes,
  including the grant-bound `taxonomy.LanguageEntitySearch` end to
  end and the fail-closed nil-entity `null` wire shape.
- `go/internal/query/code_relationships_read_admission_mirror.go` —
  compile-time proof that `*Neo4jReader` plus `*ContentReader` jointly
  serve the pin (the grant three live on `*ContentReader`, not on the
  `ContentStore` interface, so a GraphQuery+ContentStore mirror alone
  is insufficient), plus the assertion that
  `querycontract.ContentRelationshipReader`'s four signatures match
  the pin's (the builder leg is covered through those four reads;
  `BuildContentRelationships` itself is core logic, not a pin
  method).

## Census (per route per method)

| Method | `POST /api/v0/code/relationships` | `POST /api/v0/code/relationships/story` |
|---|---|---|
| `Run` | bounded graph traversal; transitive CALLS walk | story graph tracks; class_hierarchy INHERITS walk |
| `RunSingle` | bounded single traversal | story single traversal |
| `GetEntityContent` | enrichment; fallback path | story target resolution |
| `SearchEntitiesByName` | name-target resolution; builder closure | story resolution; builder closure |
| `SearchEntitiesByNameAnyRepo` | any-repo name resolution | story any-repo resolution |
| `SearchEntitiesByExactName` | exact name resolution | story exact resolution |
| `SearchEntitiesByExactNameAnyRepo` | exact any-repo resolution | story exact any-repo |
| `SearchEntitiesByLanguageAndType` | language resolution | language story resolution |
| `SearchEntitiesReferencingComponent` | builder closure; back-refs | story back-refs; builder closure |
| `ListRepoFiles` | builder closure | builder closure |
| `ListRepoEntitiesByType` | builder closure | builder closure |
| `ListRepoEntitiesByIDs` | hydration | story hydration |
| `ListRepositories` | repo-identity hydration | story repo-identity |
| `MatchRepositories` | repo-identity match | story repo-identity match |
| `GetEntityContentInRepositories` | scoped callers (fail-closed) | scoped story callers (fail-closed) |
| `SearchEntitiesByNameInRepositories` | scoped callers (fail-closed) | scoped story callers (fail-closed) |
| `SearchEntitiesByLanguageAndTypeForAccess` | scoped language callers (fallback differs) | scoped language-story (fallback differs) |

The grant three are IN, not OUT (per the ruling): two fail closed,
so a pin satisfier missing them turns granted scoped reads into
silent not-founds, and the third changes scoped language-story
answers by fallback. Grant *checks* stay handler behavior OUT (G-a
precedent); grant *reads* are store shape and belong here.

`BuildContentRelationships` is NOT a pin method. It is core logic;
the pin freezes the four reads its narrowed port
(`querycontract.ContentRelationshipReader`) is compiler-confined to.

## Explicitly OUT (not widened by this pin)

- `BuildContentRelationships` and every other builder surface: core
  logic, covered through the four narrowed reads above.
- Every other `ContentStore` method (`GetFileContent`, `GetFileLines`,
  `SearchFileContent`, `SearchFileContentAnyRepo`,
  `SearchFileContentAnyRepoExactCase`, `SearchEntityContent`,
  `SearchEntityContentAnyRepo`, `SearchCodeCandidates`,
  `NonBlankRepoFilePaths`, `GetFileContents`, `ListRepoEntities`,
  `ListRepoEntitiesByTypes`, `ListRepoEntitiesByPaths`,
  `ListRepoEntitiesByKeys`, `ListRepoK8sSelectCandidates`,
  `ListFrameworkRoutes`, `RepositoryCoverage`,
  `CountRepositoriesByLanguage`, `ListRepositoriesByLanguage`,
  `ReadRepositoriesByLanguage`, `RepositoryLanguageInventory`,
  `MatchRepositoriesByIDs`, `ResolveRepository`).
- Sibling families and their contracts (G-a graph, divergence,
  code-flow, content point/search, complexity/quality, bundles,
  cypher/visualize, structure/inventory, language-query,
  graph/entities).
- Grant checks, the unsupported-language short-circuit, fixed failure
  messages, `QueryProfile` (handler behavior, not port shape).
- No constructor, selector, or ReadStore surface.

## Satellite consumption

Depend on the shape, not a constructor: serve both routes' reads
through the existing graph reader plus the guarded Postgres content
reader, the same pair `cmd/api` wiring builds today. A satellite PR
consuming this pin for any other backend pair is invalid without
arbiter review.
