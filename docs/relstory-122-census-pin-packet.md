# #122 relstory census + pin-design + arbiter packet (PACKET-ONLY, blocked)

Part of eshu-hq/eshu#9669. Lane: eshu-hq/eshu-api#122 — Terminate
relationships/story 2 natively (wide-port pin design owed).
Owning pin: NONE. Standing ruling (b) BUILDER-NARROWING-FIRST
(issuecomment-6861505449): no G-b pin may mint until
ContentRelationshipBuilder's reader parameter is narrowed from full
ContentStore to the exact 4-method subset. Both routes stay relay
until the refactor lands and a fresh ruling reviews the mint.

## 1. Census (satellite, origin/main @ 7675119)

Scope is exactly 2 routes; both are RELAY, nothing to terminate yet:

- `POST /api/v0/code/relationships` and
  `POST /api/v0/code/relationships/story` have zero hits repo-wide in
  eshu-api (`rg "code/relationships"` over api/ openapi/ contracts/
  docs/ internal/ cmd/ exits 1). No handler, no wave entry, no
  contract, no parity row names them.
- Frozen G-a wave (`api/code_graph_wave.go:95-100`) is exactly 4 POSTs
  (call-chain, call-chain/compare, call-graph/metrics,
  routes/callers); `TestCodeGraphWaveFrozen` pins it entry by entry.
- Gate + terminated reader (`api/code_graph_terminate.go:111-160`)
  cover only those 4 rows. Both relationships POSTs fall through
  `api/router.go:482` generic relay (`mux.Handle("/api/", ... viaRelay
  RequireAuth dispatch)`); `api/dispatch.go:74-82` forwards
  byte-for-byte, sole synthetic error local 503 backend_unavailable.
- `api/family.go:131` has `/relationships/` -> "infra" (supply-chain
  advisory rows), but no `/code/relationships` prefix, so the two
  Code POSTs classify as family "other".
- Spec scope (`api/spec_scope_test.go:86-89`) owns only the G-a 4;
  `docs/provenance.md` has no #122 row (sole "122" hit is sha
  substring `...e122d3b`); parity/conformance mention only
  infra/evidence relationships routes.
- Core side mounts (read-only reference): `CodeHandler.Mount` serves
  the pair via handleRelationships / handleRelationshipStory
  (`go/internal/query/codequery/handler.go:63-64`); wire contracts are
  `getCodeRelationships` / `getCodeRelationshipStory`
  (`go/internal/query/openapi/paths/code/routes.go:132,218`); census
  rows at core `docs/internal/w8-port-census.md:343-344` list both as
  relay-leg citing the ContentRelationshipBuilder port.

## 2. Builder-narrowing verification (core @ 820f9b6; 4945be5 ancestor: yes)

- Port UNNARROWED: `BuildContentRelationships(ctx, reader
  ContentStore, entity EntityContent, logger)` at core
  `go/internal/query/querycontract/content_relationships_port.go:46-49`.
  Gate unmet on HEAD.
- Closure uses exactly the ruled 4 (verified by rg on `reader.`):
  `SearchEntitiesByName` (content_relationships.go:164,316;
  content_relationships_rust.go:29,71),
  `SearchEntitiesReferencingComponent` (content_relationships.go:208),
  `ListRepoFiles` (content_relationships_cloudformation.go:61),
  `ListRepoEntitiesByType` (content_relationships_k8s.go:157).
- Grant-scoped extras the post-narrowing design must account for
  explicitly (per ruling): optional interfaces
  `GetEntityContentInRepositories` /
  `SearchEntitiesByNameInRepositories` used fail-closed at core
  `go/internal/query/codequery/relationship_grant.go:22,29,61,89`,
  plus `SearchEntitiesByNameAnyRepo` fallback at core
  `go/internal/query/codequery/relationship_handlers.go:437`.
- Narrowing lane NOT observable from here: no local/remote branch
  `drive/9669-gb-narrow` (`git branch -a`: zero `gb-narrow` hits
  outside drain/g2/g8 rows; `git log --all`: zero `gb-narrow`
  commits); objects `e130ac9` and `14eac7a` unknown (`cat-file -t`
  fatal) in both clones; no core PR with a gb-narrow head (open PRs
  are #10650 divergence-pin, #10648 deadcode-pin, #10498 chore);
  eshu-api has zero open PRs. The issue comment claim (rebase onto
  4945be5 as e130ac9, patch-id cc33334f, tree byte-identical to
  14eac7a, unpushed, no PR) is consistent with all local evidence but
  unverifiable without the holder's push or an arbiter ruling file.

## 3. Pin design (owed; for fresh arbiter ruling, NOT implemented)

Core refactor first:

```go
// New narrow port next to ContentRelationshipBuilder.
type ContentRelationshipReader interface {
    SearchEntitiesByName(ctx context.Context, repoID, entityType, name string, limit int) ([]EntityContent, error)
    SearchEntitiesReferencingComponent(ctx context.Context, repoID, componentName string, limit int) ([]EntityContent, error)
    ListRepoFiles(ctx context.Context, repoID string, limit int) ([]FileContent, error)
    ListRepoEntitiesByType(ctx context.Context, repoID, entityType string, limit int) ([]EntityContent, error)
}
// BuildContentRelationships takes reader ContentRelationshipReader;
// callers pass h.Content unchanged (it already satisfies the subset).
```

Re-sign the chain: content_relationships_port.go:47, the adapter,
content_relationships*.go sites. Explicitly decide the 3
grant-scoped optional-interface reads + the AnyRepo fallback:
either fold their signatures into the narrow port (preferred if the
grant path must keep working for scoped tokens) or carve them as a
second optional narrow interface with fail-closed semantics
preserved; no silent widening back to ContentStore.

Satellite mint after core merge + APPROVED (G-a pattern):
`CodeRelationshipsReadAdmission` v0 pinning exactly the narrowed
4 (+ decided grant surface), recorded byte-identical; terminate both
POSTs with byte-parity + Decision-2 proof; add parity gates +
provenance rows; tick tracker eshu#10491 Code boxes; full CI green +
owner review + merge. No traffic flip/deploy/removal in this lane.

## 4. Arbiter packet (questions for the fresh ruling)

1. Approve the 4-method `ContentRelationshipReader` above as the
   narrowed port (exact signatures from querycontract/ports.go)?
2. Grant reads: fold into the narrow port or second optional narrow
   interface? Is the `SearchEntitiesByNameAnyRepo` fallback in or out
   of the pinned surface?
3. After the refactor lands, may the satellite mint
   `CodeRelationshipsReadAdmission` v0 and terminate both routes per
   section 3, or is a supplemental shape owed instead?

No arbiter model is reachable from this leaf; escalation is this
packet + the claim comment. Await the fresh APPROVED ruling before
any mint.

## 5. Leaf state

- Worktree: /home/ubuntu/personal-repos/eshu-worktrees/eshu-api-relstory-122,
  branch drive/9669-relstory-122, base origin/main@7675119.
- This packet is the only change (docs-only; zero native code until
  the gate lands). Sibling packets untouched (#124 deadcode,
  #125 divergence own theirs).
- BLOCKED: core narrowing unmerged + narrowing-lane objects
  unpushed/unverifiable + fresh arbiter APPROVED not recorded.
  Satellite record+terminate starts after core merge + ruling.
