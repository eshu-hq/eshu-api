# Divergence 3 lane (#125) — census + pin-design + arbiter packet

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#125`.
Worktree `/home/ubuntu/personal-repos/eshu-worktrees/eshu-api-divergence-125`,
branch `drive/9669-divergence-125`, base `origin/main` at `7675119`
(fetch-verified equal to `origin/main` at packet time).

Lane scope (exactly 3 routes, arbiter-TBD): `POST /api/v0/code/divergence/findings`,
`POST /api/v0/code/divergence/investigate`, `POST /api/v0/code/divergence/report`.
Owning pin: NONE. No traffic flip, no deploy, no in-tree removal in this lane.

## 1. Census (fresh, verified in this worktree)

- **Zero-hit, route/handler/store/pin symbols:** `rg
  'divergence/findings|divergence/investigate|divergence/report|DivergenceRead|divergenceStore|CodeDivergence'
  .` → no matches (exit 1). The 3 routes, their handlers, `divergenceStore`,
  and `CodeDivergenceReadAdmission` do not exist anywhere in the satellite tree.
- **Zero-hit, satellite surface files:** `rg -c 'divergence' api/router.go
  api/family.go api/dispatch.go api/spec_scope_test.go docs/provenance.md` → no
  matches (exit 1). No registration, no family prefix, no spec-scope entry, no
  provenance row.
- **Zero-hit, contracts dir:** `ls contracts/ | rg -i diverg` → no matches
  (exit 1). The owed pin contract does not exist yet — correctly, pre-arbiter.
- **Generic-English only:** `rg -i 'diverge' api contracts internal openapi`
  hits only disclosed boilerplate (`Known divergences (disclosed, not hidden)`
  in wave contracts, `future divergence from the relay contract` in parity tests,
  `would diverge from core` in negotiation tests). No lane symbol.
- **Router (`api/router.go`):** waves mounted are `registerCompareWave`,
  `registerCodeGraphWave`, `registerCodeFlowWave`, `registerCodeSearchWave`,
  `registerDriftWave`; no divergence registration. Unmatched
  `POST /api/v0/code/divergence/*` falls through to the generic relay
  `mux.Handle("/api/", s.instrumented(viaRelay, RequireAuth(...dispatch)))`.
- **Family (`api/family.go:61-172`):** no `/code/divergence` prefix. Nearest are
  `/code/call-chain|/code/call-graph|/code/routes → code_graph` (lines 118-120)
  and `/code/symbols|/code/search → code_search` (lines 121-122). The 3 routes
  classify as `other` today; the arbiter census assigns family plus
  authorizer/telemetry labels.
- **Wrapper-bypass (`api/dispatch.go:74-165`):** `dispatch()` is nil-core 503
  else `forwardToCore` on the relay lane; `forwardToCore` relays status+headers
  +body byte-for-byte (strips only `Content-Length` framing) with exactly one
  synthetic `backend_unavailable` 503. Wrapper-bypass = relay-leg termination.
- **Terminate shape (`api/code_graph_terminate.go:70-139`, read):**
  `serveCodeGraphTerminatedAs` = family-403 deny branch + `forwardToCore`
  (`viaNative`); `codeGraphReadGate` = shared-key terminate / foreign-class
  explicit relay / credential-less + junk-cookie 401. The lane clones this shape.
- **Store no-constructor rule:** `contracts/code-graph-termination.md:23-26`
  (graph reader + guarded Postgres content reader, as `cmd/api` builds) and
  `contracts/cloud-runtime-drift-termination.md:20-22` (existing Postgres
  adapter) both state deliberately no `WithReadStore` constructor. The
  divergence pin design follows the rule.
- **Graph track frozen:** `api/code_graph_wave.go:95-100` pins exactly 4 POSTs
  (call-chain, call-chain/compare, call-graph/metrics, routes/callers); the
  contract (`contracts/code-graph-termination.md:10-26`) seals all other
  `GraphQuery`/`ContentStore` legs OUT and voids cross-pin citation. Divergence
  needs a new wave table, never an extension.
- **Satellite history/PRs:** `gh pr list` in `eshu-api` → open `[]`; `git log -i
  --grep=divergence` → only `f73a59c` (contract wording bullet, not the routes);
  no divergence parity/conformance file (siblings `code_graph_parity_test.go`,
  `code_graph_conformance_test.go` exist). No satellite claim comments on #125.
- **Siblings:** #124 (dead-code 3, OPEN, pin NONE) and #122 (relationships/story
  2, OPEN) are distinct scopes, no overlap with the divergence 3.

## 2. Pin design (SUPPLEMENTAL-PIN-OWED, per arbiter ruling on #125)

- **Pin:** net-new `CodeDivergenceReadAdmission v0`, exactly 6 methods —
  `DivergenceGroupStats`, `DivergenceMembers`, `DivergenceMembersByEntityID`,
  `DriftedFindingStats`, `DriftedFindingRows`, `Run` — minted in core
  `codequery` before any satellite consumption. All 3 routes stay relay until it
  merges.
- **Mirror:** struct embedding staying `divergenceStore` + `GraphQuery`
  (CodeGraph pattern). No `WithReadStore` constructor, no handler rewiring in
  the mint, unwired seam (no behavior change).
- **Coverage in one pin:** both graph tracks (wrapper-bypass + convention
  outlier) share `Run` + `DivergenceMembersByEntityID`, so one pin covers
  findings, investigate, and report; the convention-outlier track adds zero pin
  methods.
- **Sealed OUT:** `RunSingle`, selectors, `ReadStore`, builders.
- **No existing pin covers these routes:** CodeGraph is G-a-4-only, ContentPoint
  is route-scoped, CodeFlow is single-method.
- **Core mint:** `eshu-hq/eshu#10650` (`drive/9669-divergence-pin`, +265/-0, 2
  new files, unwired seam) matches the ruled mint verbatim. State at packet
  time: OPEN, MERGEABLE, unmerged.

## 3. Arbiter approval packet

Request: lane arbiter review **APPROVED** on (a) the census in §1 and (b) the
pin design in §2. Current acceptance state on #125: all 4 boxes unchecked —
box 1 (APPROVED census+pin) is the decision owed back to this leaf; boxes 2-4
(core merge, parity+provenance+tracker, CI+review+merge) are downstream of it.
Per the ruling, satellite performs record + terminate only after the core mint
merges; byte-parity + Decision-2 proof where owed.

## 4. Worktree state + focused proof (no `make pre-pr` / `make pre-push`)

- Worktree verified (`WORKTREE_OK`) before any edit; this packet doc is the only
  change on the leaf branch.
- `go build ./...` → exit 0.
- `go test -count=1 ./api/ -run 'TestSpecScope|TestClassifyFamily|TestFamily'` →
  `ok github.com/eshu-hq/eshu-api/api`.
- Zero-hit `rg` probes above → exit 1 (no matches), as required pre-approval.
- No implementation started (TDD N/A until arbiter APPROVED + core merge); no
  traffic flip, deploy, or removal.
- Issues-Created-During-A-Goal ledger: no new issues created by this leaf; no
  `.issues` goal file exists in-repo to update.

## 5. Block + handoff

**BLOCKED on:** (1) core pin merge — `eshu-hq/eshu#10650` still OPEN/unmerged;
(2) arbiter APPROVED ruling on this census+pin (box 1 unchecked).
Post-unblock, in order: satellite record + terminate with byte-parity +
Decision-2 proof, parity gates + provenance rows, tracker boxes, full CI green +
owner review + merge. Local HEAD SHA recorded in the commit carrying this file;
handed off for coordinator promotion (no pre-pr/pre-push run here, per lane
rules).
