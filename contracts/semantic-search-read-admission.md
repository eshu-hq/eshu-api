# Semantic-search read admission contract

Versioned admission-contract port for the semantic-search read (`POST
/api/v0/search/semantic`) (Refs `eshu-hq/eshu#9669`, O6 pin 4).
Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned index, hybrid, scope, and watermark readers
satisfy jointly, composed by the consumer. `eshu-api` never takes a
direct store handle.

The pin re-declares the five mirrored signatures in this
SemanticSearch-scoped shape instead of embedding or aliasing
`SemanticSearchIndexStore`, `SemanticSearchHybridStore`,
`SemanticSearchScopeResolver`, or `SemanticSearchVectorReadyReader`,
so no other family can cite it as coverage.

## Version

`SemanticSearchReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type SemanticSearchReadAdmission interface {
	IndexSearch(ctx context.Context, query SemanticSearchIndexQuery) (SemanticSearchIndexResult, error)
	HybridSearch(ctx context.Context, query SemanticSearchIndexQuery) (SemanticSearchIndexResult, error)
	ResolveScope(ctx context.Context, repoID string) (string, error)
	ResolveRepositoryForScope(ctx context.Context, scopeID string) (string, error)
	VectorReadyWatermark(ctx context.Context) (SearchVectorReadyFreshness, error)
}
```

The version line, the interface, the one-directional compile-time
mirror, and the surrounding contract comment (narrow by construction,
explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/semantic-search-read-wire-shapes.txt`. The frozen
method set is `frozenSemanticSearchReadAdmission` in
`contracts/upstream/semantic_search_read_admission_contract_test.go`
(5 methods: `IndexSearch`, `HybridSearch`, `ResolveScope`,
`ResolveRepositoryForScope`, `VectorReadyWatermark`).

## Read surface

The seam is read-only on purpose: every declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve semantic search but cannot
acquire a write it did not bargain for through it.

## In scope

- `POST /api/v0/search/semantic` admits its persisted-index search
  through `IndexSearch` (mirroring
  `SemanticSearchIndexStore.Search`), its local hybrid search through
  `HybridSearch` (mirroring `SemanticSearchHybridStore.Search`), its
  scope resolution through `ResolveScope` and
  `ResolveRepositoryForScope` (mirroring
  `SemanticSearchScopeResolver`), and its search-vector build sweep
  watermark probe through `VectorReadyWatermark` (mirroring
  `SemanticSearchVectorReadyReader.SearchVectorReadyWatermark`).

Explicitly OUT of this pin: grant checks and fixed failure messages
are handler behavior, not port shape; the rerank, telemetry, and
response-normalization legs consume no port.

## Relay posture

The semantic-search POST stays on the generic relay until the
termination lane lands; no native route moves in this record. The pin
is recorded here first so the termination lane consumes a minted pin,
never an unminted one, with truth-freshness parity on the
pending_search_vector cause.

## Enforcement

`TestSemanticSearchReadAdmissionContractIsVersioned` pins the version
and the frozen method set, with the seeded GREEN proof
(`TestSemanticSearchReadAdmissionServesSeededReads`) and the seeded
RED proof (`TestSemanticSearchReadAdmissionSurfacesSeededFailures`).
A renamed, added, removed, or re-signed method fails the frozen test
until the list is deliberately updated; bump
`SemanticSearchReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
