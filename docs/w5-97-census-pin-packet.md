# W5 census + pin-design + arbiter packet (eshu-hq/eshu-api#97)

Lane: satellite termination, W5 entities/impact/story/packet, 24 routes.
Worktree: `/home/ubuntu/personal-repos/eshu-worktrees/eshu-api-w5-97`,
branch `drive/9669-w5-97`, base `origin/main@289da69` (post-#132 dead-code
merge; PRs #133/#134 open and conflicting — untouched by this lane).
Scope guard: divergence/dead-code files and leaves 125/122/101/124 own
their packets; this doc adds one new file only, no shared-file edits.

## 1. Method-level census (satellite, origin/main@289da69 — inspected)

All 24 routes fall through to the generic relay
(`api/router.go:483`, `mux.Handle("/api/", ... s.dispatch)`). Exact-path
fragment search over `api/`, `openapi/`, `contracts/` returns zero native
registrations for every W5 spelling; near-misses belong to other families
(`content/entities/search`, `freshness/services/changed-since`,
`GET supply-chain/impact/explain` — none is a W5 route).

| # | Route | Satellite state |
|---|-------|-----------------|
| 1 | GET /api/v0/entities/{entity_id}/context | relay, zero native hits |
| 2 | POST /api/v0/entities/resolve | relay, zero native hits |
| 3 | GET /api/v0/investigations/services/{service_name} | relay, zero native hits |
| 4 | GET /api/v0/services/{service_name}/api-surface/endpoints | relay, zero native hits |
| 5 | GET /api/v0/services/{service_name}/context | relay, zero native hits |
| 6 | GET /api/v0/services/{service_name}/downstream-consumers | relay; core Mint A explicitly OUT ("not a W5 route") |
| 7 | GET /api/v0/services/{service_name}/story | relay, zero native hits |
| 8 | GET /api/v0/workloads/{workload_id}/context | relay, zero native hits |
| 9 | GET /api/v0/workloads/{workload_id}/story | relay, zero native hits |
| 10-22 | POST /api/v0/impact/* (13: blast-radius, change-surface, change-surface/investigate, contracts, deployment-config-influence, developer-change-plan, entity-map, explain-dependency-path, pre-change, resource-investigation, trace-deployment-chain, trace-exposure-path, trace-resource-to-code) | relay, zero native hits |
| 23 | GET /api/v0/repositories/{repo_id}/story | relay by explicit leg `d4RelayLegs` (`api/d4_wave.go:55-57`, registered `api/router.go:380-384`); fragment waits on `schema.EvidenceBoundaries` (gap #74) |
| 24 | GET /api/v0/investigations/supply-chain/impact/packet | relay; admits through writer `ImpactPacketResponder` + shared graph/content ports the pin excludes (`api/d6_wave.go:61-68`; gap #71) |

No `*_wave.go`, `*_terminate.go`, or spec fragment exists for any W5
family in `api/` / `openapi/paths/`. Tracker eshu-hq/eshu#10491 rows for
these 24 remain unchecked — correct, no edit owed by this leaf.

## 2. Core pin state (read-only, via gh API)

- Base `e504e328a7` verified (core Go-toolchain commit, not a pin).
- **Mint A EXISTS + MERGED:** `EntityReadAdmission v0`
  (`go/internal/query/entity/read_admission.go`, 15 re-declared reads:
  Run, RunSingle, GetEntityContent, SearchEntitiesByName,
  SearchEntitiesByNameAnyRepo, ResolveRepository, ListRepoEntitiesByKeys,
  ListRepoEntitiesByIDs, MatchRepositories, ListRepositories,
  ListRepoFiles, SearchEntityNames, ListCICDRunCorrelations,
  ListContainerImageIdentities, ListSBOMAttestationAttachments),
  mint commit `8758041` (2026-11-12), ancestor of core main `d584e38`
  (behind_by 0), 10 commits ahead of base `e504e328a7`. One-direction
  mirror assertion + frozen method-set test. Unwired seam (no handler
  depends on it). Cites a W5 arbiter ruling (APPROVED-WITH-CONDITIONS).
- **No other W5 mints:** code search for `ImpactReadAdmission`,
  `ServiceReadAdmission`, `WorkloadReadAdmission` returns zero files;
  no open core PR titles match "admission".
- `SupplyChainReadAdmission v0` exists (D6 wave) but stays partial per
  the issue: writer + graph/content composition OUT.
- `schema/evidence_boundaries.go` exists core-side; satellite story
  fragment not ported (gap #74 open).

Derived Mint A coverage (routes 1-5, 7-9 = 8 routes; arbiter to
confirm): entity context + resolve, investigations/services, services
context + api-surface/endpoints + story, workloads context + story.
"Service and workload story ride this mint" per the pin header.

## 3. Per-family pin designs for arbiter approval

- **F-A entity-8:** record Mint A byte-identical
  (`contracts/entity-read-admission.md` + frozen method test vendored),
  new `entity_wave.go` (8 entries) + gate + family/router/spec-scope
  wiring, byte-parity + Decision-2-not-owed proof (pin records no
  suppression leg). Drift note: Mint A is 10 ahead of `e504e328a7`;
  approval must cover the delta or rebase the design to it.
- **F-B impact-13:** core mint owed — propose narrow
  `ImpactReadAdmission v0` (13 POST routes, read-only, per-route method
  closure proven core-side first). No satellite work starts before mint.
- **F-C downstream-consumers-1:** explicitly OUT of Mint A; arbiter to
  assign a family (entity-extension vs own narrow pin) before any mint.
- **F-D repo-story-1:** waits on core `schema.EvidenceBoundaries`
  satisfier + fragment port (gap #74); then record + terminate with
  spec-bijection proof both directions.
- **F-E supply-packet-1:** composition pin owed (writer + graph +
  content) with Decision-2 suppression-filter proof (gap #71); partial
  `SupplyChainReadAdmission` must not be cited as coverage.

## 4. Arbiter questions owed (no arbiter tool in leaf session)

1. APPROVE F-A..F-E designs against core `e504e328a7` (box 1), incl.
   Mint A delta reconcile and exact 8-route enumeration.
2. Release full text/conditions of the cited W5 APPROVED-WITH-CONDITIONS
   ruling (not found in epic comments pp. 1-2; code search rate-limited
   before verbatim confirm).
3. Confirm Decision-2 NOT-OWED for F-A legs vs owed proof shape for F-E.
4. Assign F-C downstream-consumers to a family/pin.

## 5. Block + handoff

BLOCKED by design: arbiter APPROVED per-family designs not recorded;
core mints for F-B/F-C/F-D-composition/F-E owed; satellite
record+terminate starts after rulings + merges. No code touched, no PR
opened, no tracker/gap edits, no new issues. Proof: `go build ./...`,
`go test ./api/ -run 'TestSpecScope|TestD4|TestD6'`, `go vet ./api/`,
`scripts/check-parity.sh` — results in lane report.
