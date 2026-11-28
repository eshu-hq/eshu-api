# Admin P0 census — frozen baseline (22 operations)

**Status** — Frozen baseline, docs-only. This record observes the
current dispatch lanes; it changes no behavior and authorizes no
migration. Any lane move after this census must diff against this
table and update it in the same change.

**Baseline identity** — branch `drive/admin-p0-census-69`, HEAD
`b9611793f70859c21046eb2e8008e1cbe68e1849` ("Merge pull request #166
from eshu-hq/docs/adr-0002-compute-ownership"). Served-spec snapshot:
`spec_sha256 a4a940d979beda142ad81d401f60136f7ab08ad7ea75527125ef2592cc1c2c7a`,
`served 279`, `mounted 284`, `disclosed_absent 5`
(`openapi/spec_snapshot.json:1-5` region; metadata keys verified by
parse).

## Census scope and numbering basis

The census covers the 22 operations under `/api/v0/admin/`: the 3
GETs (census #169–171) and the 19 POSTs (census #318–336). Census
numbers are assigned sequentially in `served_operations` order —
GETs at `openapi/spec_snapshot.json:8-10`, POSTs at
`openapi/spec_snapshot.json:157-175`. The counts reconcile exactly:
3 `GET /api/v0/admin/...` entries and 19 `POST /api/v0/admin/...`
entries in the snapshot; 3 + 19 = 22. The snapshot itself carries no
numeric IDs, so the #169–171 / #318–336 labels are census-local,
bound to snapshot order by this document.

Out of scope (disclosed, not silent): the 8 non-`/api/v0/admin/`
routes in the same D3 wave (6 `auth/admin` / `auth/local` GETs and 2
`auth/admin/audit` GETs, `api/d3_wave.go:84-91`), and all
`auth/admin` mutations. They do not appear in the 22-row table.

## The two lanes (definitions)

- **d3ReadGate-gated** — registered as
  `s.d3ReadGate(http.HandlerFunc(s.serveD3Terminated))`
  (`api/router.go:273` in the D3 loop, `api/router.go:283` in the
  trio loop). A valid shared key terminates natively; a non-key
  Bearer or session-cookie-shaped Cookie relays explicitly;
  credential-less and junk-cookie callers get the standing 401
  (`api/d3_terminate.go:121-167`). Terminated calls forward at
  `api/d3_terminate.go:96` (`forwardToCore(s, w, r, viaNative)`);
  explicit relays forward at `api/d3_terminate.go:185` (`viaRelay`).
  Family authorizer: `authorizeD3SharedKey`
  (`api/d3_terminate.go:48-50`), invoked in `serveD3TerminatedAs`
  (`api/d3_terminate.go:84`).
- **d3ReadGate-gated, locally terminated (#171, #335, #324, #325, #170)** —
  the tuning-report GET (#171), the work-items query POST (#335),
  the dead-letters query POST (#324), the decisions query POST
  (#325), and the reindex-progress GET (#170) keep the d3ReadGate
  credential matrix above but terminate
  locally, each registered on its own anchor ahead of the D3 loops:
  - #171: `s.registerAdminTuningReport`, `api/router.go:243`;
    route const and handler `api/admin_tuning_terminate.go:52-110`.
    Shared-key calls serve the pinned apilib static body
    (`apilibadmin.WriteTuningReport`,
    `api/admin_tuning_terminate.go:109`) filing `local_ok` on the
    native lane (`api/admin_tuning_terminate.go:108`) with zero core
    and zero store contact.
  - #335: `s.registerAdminWorkItemsQuery`, `api/router.go:246`;
    route const (`api/admin_workitems_terminate.go:57`), server
    store field (`api/server.go:33`), `WithWorkItemStore` option
    (`api/admin_workitems_terminate.go:59-67`), and handler
    (`api/admin_workitems_terminate.go:69-128`). Shared-key calls
    serve the pinned apilib bounded-list body
    (`apilibadmin.ServeWorkItemsQuery`,
    `api/admin_workitems_terminate.go:127`) filing `local_ok` on
    the native lane (`api/admin_workitems_terminate.go:126`) with
    zero core contact, read through the reader-role PG store
    (`internal/adminread`: bounded SELECT on `fact_work_items`,
    SELECT-only grant). A missing store answers the apilib local
    503 (never a forward); a failed store answers the fixed 500.
  - #324: `s.registerAdminDeadLettersQuery`, `api/router.go:249`;
    route const (`api/admin_deadletters_terminate.go:76`), server
    store field (`api/server.go:34-38`), `WithDeadLetterStore`
    option (`api/admin_deadletters_terminate.go:90-97`), and handler
    (`api/admin_deadletters_terminate.go:99-163`). Shared-key calls
    first validate the required limit (1-500) and timeout_ms
    (1-30000) dest-side with the core-identical 400 details
    (`api/admin_deadletters_terminate.go:173-230`), then serve the
    pinned apilib bounded-list body
    (`apilibadmin.ServeDeadLettersQuery`,
    `api/admin_deadletters_terminate.go:162`) filing `local_ok` on
    the native lane (`api/admin_deadletters_terminate.go:159`) with
    zero core contact, read through the reader-role PG store
    (`internal/adminread/deadletters.go`: bounded SELECT joining
    `fact_work_items` to `ingestion_scopes` for the collector kind,
    SELECT-only grants on both). The timeout also bounds the store
    call through the request context. A missing store answers the
    apilib local 503 (never a forward); a failed store answers the
    fixed 500.
  - #325: `s.registerAdminDecisionsQuery`, `api/router.go:252`;
    route const (`api/admin_decisions_terminate.go:65`), server
    store field (`api/server.go:43`), `WithDecisionStore` option
    (`api/admin_decisions_terminate.go:72-76`), and handler
    (`api/admin_decisions_terminate.go:103-139`). Shared-key calls
    serve the pinned apilib bounded-list body with its per-decision
    evidence fan-out (`apilibadmin.ServeDecisionsQuery`,
    `api/admin_decisions_terminate.go:139`) filing `local_ok` on
    the native lane (`api/admin_decisions_terminate.go:138`) with
    zero core contact, read through the reader-role PG store
    (`internal/adminread/decisions.go`: bounded SELECTs on
    `projection_decisions` plus `projection_decision_evidence`,
    SELECT-only grants on both). The required repository_id and
    source_run_id validate inside the pinned apilib with the
    core-identical 400; an absent limit defaults to 100, like core.
    A missing store answers the apilib local 503 (never a forward);
    a failed store answers the fixed 500.
  - #170: `s.registerAdminReindexProgress`, `api/router.go:255`;
    route const (`api/admin_reindex_progress_terminate.go:68`),
    server store field (`api/server.go:44-49`), `WithReindexProgress`
    option (`api/admin_reindex_progress_terminate.go:70-78`), and
    handler (`api/admin_reindex_progress_terminate.go:90-140`).
    Shared-key calls serve the pinned apilib progress body
    (`apilibadmin.ServeReindexProgress`,
    `api/admin_reindex_progress_terminate.go:139`) filing `local_ok`
    on the native lane
    (`api/admin_reindex_progress_terminate.go:138`) with zero core
    contact, read through the reader-role PG store
    (`internal/adminread/reindexprogress.go`: one bounded SELECT per
    progress input — fleet watermark, per-repository watermarks,
    per-scope stored full plus active generation — SELECT-only
    grants on all four tables). The apilib validates scope_id and
    fleet_requested_at dest-side with the core-identical 400s. A
    missing store answers the apilib local 503 (never a forward); a
    failed store answers the fixed 500.
  All five keep foreign-class explicit relay on the untouched gate
  and the standing 401 for credential-less callers. Family
  authorizer for all four: the shared `authorizeD3SharedKey` trust
  root (`api/d3_terminate.go:48-50`), judged in
  `serveAdminTuningTerminatedAs` (`api/admin_tuning_terminate.go:93`),
  `serveAdminWorkItemsTerminatedAs`
  (`api/admin_workitems_terminate.go:95-128`),
  `serveAdminDeadLettersTerminatedAs`
  (`api/admin_deadletters_terminate.go:122-163`),
  `serveAdminDecisionsTerminatedAs`
  (`api/admin_decisions_terminate.go:103-139`), and
  `serveAdminReindexProgressTerminatedAs`
  (`api/admin_reindex_progress_terminate.go:106-140`).
- **RequireAuth-native** — registered as
  `RequireAuth(s.apiKey, http.HandlerFunc(s.<family>WriteThrough))`
  (`RequireAuth` defined at `api/auth.go:189`; cited read-only, not
  edited by this step). No credential-class relay: non-key callers
  fail `RequireAuth`; keyed callers reach the family authorizer and
  the family's single `forwardToCore(..., viaNative)` site.
- Both lanes end at `forwardToCore`, defined once at
  `api/dispatch.go:252`. Every row below also registers a
  trailing-slash twin on the same handler (never-split rule); the
  table cites the primary registration line and the twin follows on
  the next line in each case.

## Frozen baseline table — 3 GETs (#169–171)

#169 is the remaining Gap 02 wave route: wave entry
`api/d3trio_wave.go:64-66` (`Serve: d3ServeGated`), registered by the
loop at `api/router.go:281-285` via `s.d3ReadGate`
(`api/router.go:283`). #171 promoted to its own native lane in
eshu-hq/eshu-api#171 and #170 in eshu-hq/eshu-api#170 (no wave
entries; anchors `api/router.go:243` and `api/router.go:255`).
Contract record:
`contracts/d3-admin-reads.md:26-42`, `:98-99` (Gap 02 pins v0, no D1
`ReadStore` claim); the #171 termination proof lives in
`api/admin_tuning_terminate_test.go` (P-FLOW zero-core case, census
`AssertNoCorePath`, byte-identical body pin).

| # | Operation | Lane | Wave entry | Registration | forwardToCore site |
| --- | --- | --- | --- | --- | --- |
| 169 | `GET /api/v0/admin/delta-union/completeness` | d3ReadGate-gated | `api/d3trio_wave.go:69` | `api/router.go:281-285` via `s.d3ReadGate` (`api/router.go:283`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 170 | `GET /api/v0/admin/reindex/progress` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_reindex_progress_terminate.go:68`) | `api/router.go:255` via `s.d3ReadGate` + `serveAdminReindexProgressTerminated` | none — local `apilibadmin.ServeReindexProgress` (pinned apilib v0.3.0, no replace) on the reader-role store (`local_ok`); scope_id/fleet_requested_at enforced dest-side with core-identical 400s; foreign classes relay `api/d3_terminate.go:185` |
| 171 | `GET /api/v0/admin/shared-projection/tuning-report` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_tuning_terminate.go:52`) | `api/router.go:243` via `s.d3ReadGate` + `serveAdminTuningTerminated` | none — local `apilibadmin.WriteTuningReport` (`local_ok`); foreign classes relay `api/d3_terminate.go:185` |

## Frozen baseline table — 19 POSTs (#318–336)

### D3-gated query POSTs (5 of 19)

Wave entries in `d3Wave` (`Serve: d3ServeGated`,
`api/d3_wave.go:59`), registered by the loop at
`api/router.go:271-275` through `s.d3ReadGate` (`api/router.go:273`).
Same forward sites as the GETs. Contract rows:
`contracts/d3-admin-reads.md:98` (Admin, 6 POSTs — the record still
lists the pre-promotion six; 3 remain on the wave, work-items
promoted in #335, dead-letters in #324, and decisions in #325) and
`contracts/d3-admin-reads.md:102` (TerminalFailure, 2 POSTs).

| # | Operation | Lane | Wave entry | Registration | forwardToCore site |
| --- | --- | --- | --- | --- | --- |
| 319 | `POST /api/v0/admin/changed-since/poisoned-links/query` | d3ReadGate-gated | `api/d3_wave.go:86` | `api/router.go:271-275` via `s.d3ReadGate` (`api/router.go:273`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 321 | `POST /api/v0/admin/collector-failures/acknowledgements/query` | d3ReadGate-gated | `api/d3_wave.go:96` | `api/router.go:271-275` via `s.d3ReadGate` (`api/router.go:273`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 322 | `POST /api/v0/admin/collector-failures/query` | d3ReadGate-gated | `api/d3_wave.go:97` | `api/router.go:271-275` via `s.d3ReadGate` (`api/router.go:273`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 324 | `POST /api/v0/admin/dead-letters/query` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_deadletters_terminate.go:76`) | `api/router.go:249` via `s.d3ReadGate` + `serveAdminDeadLettersTerminated` | none — local `apilibadmin.ServeDeadLettersQuery` (pinned apilib v0.2.1, no replace) on the reader-role store (`local_ok`); required limit+timeout_ms enforced dest-side with core-identical 400s; updated_after/updated_before validated dest-side and applied through the reader-store core-exact range; foreign classes relay `api/d3_terminate.go:185` |
| 325 | `POST /api/v0/admin/decisions/query` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_decisions_terminate.go:65`) | `api/router.go:252` via `s.d3ReadGate` + `serveAdminDecisionsTerminated` | none — local `apilibadmin.ServeDecisionsQuery` (pinned apilib v0.3.0, no replace) on the reader-role store (`local_ok`); required repository_id+source_run_id enforced in the apilib with core-identical 400s, absent limit defaults to 100 like core; per-decision evidence fan-out through the reader-store core-exact SELECT; foreign classes relay `api/d3_terminate.go:185` |
| 326 | `POST /api/v0/admin/input-invalid-facts/query` | d3ReadGate-gated | `api/d3_wave.go:85` | `api/router.go:271-275` via `s.d3ReadGate` (`api/router.go:273`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 332 | `POST /api/v0/admin/replay-events/query` | d3ReadGate-gated | `api/d3_wave.go:87` | `api/router.go:271-275` via `s.d3ReadGate` (`api/router.go:273`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 335 | `POST /api/v0/admin/work-items/query` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_workitems_terminate.go:57`) | `api/router.go:246` via `s.d3ReadGate` + `serveAdminWorkItemsTerminated` | none — local `apilibadmin.ServeWorkItemsQuery` on the reader-role store (`local_ok`); foreign classes relay `api/d3_terminate.go:185` |

### RequireAuth-native mutation POSTs (11 of 19)

| # | Operation | Lane | Route definition | Registration | forwardToCore site | Family authorizer |
| --- | --- | --- | --- | --- | --- | --- |
| 318 | `POST /api/v0/admin/backfill` | RequireAuth-native (D7 tier-C) | `api/d7.go:36`; listed `api/d7.go:44-49` | `api/router.go:125-128` loop over `d7Routes`, `RequireAuth(... s.d7WriteThrough)` | `api/d7.go:148` in `d7WriteThroughAs` (`api/d7.go:126-149`) | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 320 | `POST /api/v0/admin/collector-failures/acknowledge` | RequireAuth-native (tier C-a) | `api/terminalfailure.go:17` | `api/native_singles.go:51-52` via `registerNativeSingles` (called `api/router.go:53`) | `api/terminalfailure.go:73` | `authorizeTerminalFailureAdmin` `api/terminalfailure.go:84-86`, checked `api/terminalfailure.go:68` |
| 323 | `POST /api/v0/admin/dead-letter` | RequireAuth-native (D7 tier-C) | `api/d7.go:37`; listed `api/d7.go:44-49` | `api/router.go:125-128` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 327 | `POST /api/v0/admin/recover-generations` | RequireAuth-native (tier C-c2) | `api/recovery.go:30`; listed `api/recovery.go:35-40` | `api/router.go:216-219` loop over `recoveryRoutes`, `RequireAuth(... s.recoveryWriteThrough)` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 328 | `POST /api/v0/admin/refinalize` | RequireAuth-native (tier C-c2) | `api/recovery.go:29`; listed `api/recovery.go:35-40` | `api/router.go:216-219` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 329 | `POST /api/v0/admin/reindex` | RequireAuth-native (D7 tier-C) | `api/d7.go:38`; listed `api/d7.go:44-49` | `api/router.go:125-128` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 330 | `POST /api/v0/admin/reopen` | RequireAuth-native (tier C-c2) | `api/recovery.go:28`; listed `api/recovery.go:35-40` | `api/router.go:216-219` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 331 | `POST /api/v0/admin/replay` | RequireAuth-native (tier C-c2) | `api/recovery.go:27`; listed `api/recovery.go:35-40` | `api/router.go:216-219` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 333 | `POST /api/v0/admin/repository-retirements` | RequireAuth-native (tier C-a) | `api/retirement.go:15` | `api/native_singles.go:43-44` via `registerNativeSingles` (called `api/router.go:53`) | `api/retirement.go:60` | `authorizeRetirementAdmin` `api/retirement.go:70-72`, checked `api/retirement.go:55` |
| 334 | `POST /api/v0/admin/skip` | RequireAuth-native (D7 tier-C) | `api/d7.go:39`; listed `api/d7.go:44-49` | `api/router.go:125-128` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 336 | `POST /api/v0/admin/workflow-retire-unclaimed` | RequireAuth-native (writer) | `api/retireunclaimed.go:16` | `api/router.go:55-56` direct `mux.Handle`, `RequireAuth(... s.retireUnclaimedWriteThrough)` | `api/retireunclaimed.go:88` | `authorizeRetireUnclaimedAdmin` `api/retireunclaimed.go:99-101`, checked `api/retireunclaimed.go:76` |

Contract records for the native rows: D7 four —
`contracts/d7-admin-four.md:151-165` (routes inside `RequireAuth`,
tier-C dispatch only, not termination); recovery four — handler
record `api/recovery.go:10-25`; acknowledge —
`api/terminalfailure.go:10-17`; retirement — `api/retirement.go:10-15`;
retire-unclaimed — `api/retireunclaimed.go:10-16`.

## Reconciliation

- 22 = 6 d3ReadGate-gated (1 wave GET + 5 query POSTs) + 5
  d3ReadGate-gated, locally terminated (#171 no-store, #335, #324,
  #325, and #170 reader-role PG) + 11 RequireAuth-native (4 D7 + 4
  recovery + acknowledge + retirement + retire-unclaimed). No census
  operation rides the generic byte-relay as its primary lane today;
  relay appears only as the d3ReadGate explicit-relay branch for
  unresolvable credential classes
  (`api/d3_terminate.go:175-186`), which #171, #335, #324, #325, and
  #170 keep for their foreign classes.
- Every native handler passes `allScopes=true` today (e.g.
  `api/d7.go:115-117`, `api/recovery.go:81-83`,
  `api/terminalfailure.go:50-52`, `api/retirement.go:37-39`,
  `api/retireunclaimed.go:54-56`); every family deny branch is
  documented unreachable until a scoped/session resolver lands.
  Core re-judges every forwarded call as backstop on both lanes.
- Registration-order note (observed, not a defect claim): the
  recovery block comment sits at `api/router.go:112-119`, followed by
  the D7 comment directly above the D7 loop (`api/router.go:125-128`),
  while the recovery
  loop itself registers later, uncommented, at
  `api/router.go:216-219` — after the D8/D9 blocks. Registration
  order in `routes()` for census rows is therefore: native singles
  (`api/router.go:53`) → retire-unclaimed
  (`api/router.go:55-56`) → D7 (`api/router.go:125-128`) →
  recovery (`api/router.go:216-219`) → tuning-report anchor
  (`api/router.go:243`) → work-items query anchor
  (`api/router.go:246`) → dead-letters query anchor
  (`api/router.go:249`) → decisions query anchor
  (`api/router.go:252`) → reindex-progress anchor
  (`api/router.go:255`) → D3 wave (`api/router.go:271-275`) →
  D3 trio (`api/router.go:281-285`), all ahead of the generic
  proxy. Each anchor precedes the loops so a promoted route never
  falls through to a wave registration; the mux would panic on the
  duplicate pattern if the wave entry had not been deleted with the
  promotion.
- Wave-freeze tests pinning the gated tables exist by reference in
  the wave files (`TestD3WaveFrozen`, `api/d3_wave.go:84-97`;
  `TestD3TrioWaveFrozen`, `api/d3trio_wave.go:58-67`); the promoted
  tuning-report lane pins its own freeze in
  `api/admin_tuning_terminate_test.go`
  (`TestAdminTuningReportRouteFrozen`,
  `TestAdminTuningReportTerminatesLocally`), the promoted
  work-items lane pins its own freeze, parity, and zero-core proof
  in `api/admin_workitems_terminate_test.go`
  (`TestAdminWorkItemsRouteFrozen`,
  `TestAdminWorkItemsTerminatesLocally`,
  `TestAdminWorkItemsBoundedListParity`, `TestAdminWorkItemsProofRead`)
  and `api/admin_workitems_parity_test.go`
  (`TestAdminWorkItemsCensus`), the promoted dead-letters lane
  pins its own freeze, parity, and zero-core proof in
  `api/admin_deadletters_terminate_test.go`
  (`TestAdminDeadLettersRouteFrozen`,
  `TestAdminDeadLettersTerminatesLocally`,
  `TestAdminDeadLettersRequiredFieldsParity`,
  `TestAdminDeadLettersBoundedListParity`,
  `TestAdminDeadLettersProofRead`) and
  `api/admin_deadletters_parity_test.go`
  (`TestAdminDeadLettersCensus`), the promoted decisions lane pins
  its own freeze, parity, and zero-core proof in
  `api/admin_decisions_terminate_test.go`
  (`TestAdminDecisionsRouteFrozen`,
  `TestAdminDecisionsTerminatesLocally`,
  `TestAdminDecisionsRequiredFieldsParity`,
  `TestAdminDecisionsBoundedListParity`,
  `TestAdminDecisionsProofRead`) and
  `api/admin_decisions_parity_test.go`
  (`TestAdminDecisionsCensus`), and the promoted reindex-progress
  lane pins its own freeze, parity, and zero-core proof in
  `api/admin_reindex_progress_terminate_test.go`
  (`TestAdminReindexProgressRouteFrozen`,
  `TestAdminReindexProgressTerminatesLocally`,
  `TestAdminReindexProgressShapeParity`,
  `TestAdminReindexProgressProofRead`) and
  `api/admin_reindex_progress_parity_test.go`
  (`TestAdminReindexProgressCensus`); this
  census step ran the touched suites green (`go test ./api/
  ./cmd/... ./internal/...`, plus the env-gated live reader proofs
  `TestAdminReadLiveProof`, `TestAdminReadDeadLettersLiveProof`,
  `TestAdminReadDecisionsLiveProof`, and
  `TestAdminReadReindexProgressLiveProof` against local PG).
  `make pre-pr` / `pre-push` stay coordinator-owned.

## Consequences / what this does not claim

- This is a lane census, not a termination or removal-readiness
  claim for any row. D7 rows stay tier-C dispatch per
  `contracts/d7-admin-four.md:201-208`; the remaining D3 wave rows
  stay provisional per `contracts/d3-admin-reads.md:44-54`
  (blocker B1). #171 terminated locally under its own proof
  (`api/admin_tuning_terminate_test.go`); #335 terminated locally
  under its own proof (`api/admin_workitems_terminate_test.go`,
  `api/admin_workitems_parity_test.go`, live reader proof
  `TestAdminReadLiveProof` in
  `internal/adminread/adminread_live_test.go`); #324 terminated
  locally under its own proof
  (`api/admin_deadletters_terminate_test.go`,
  `api/admin_deadletters_parity_test.go`, live reader proof
  `TestAdminReadDeadLettersLiveProof` in
  `internal/adminread/adminread_live_test.go`, windowed); #325 terminated
  locally under its own proof
  (`api/admin_decisions_terminate_test.go`,
  `api/admin_decisions_parity_test.go`, live reader proof
  `TestAdminReadDecisionsLiveProof` in
  `internal/adminread/decisions_live_test.go`); and #170
  terminated locally under its own proof
  (`api/admin_reindex_progress_terminate_test.go`,
  `api/admin_reindex_progress_parity_test.go`, live reader proof
  `TestAdminReadReindexProgressLiveProof` in
  `internal/adminread/reindexprogress_live_test.go`). Each op's
  core-side removal readiness is a separate claim this census does
  not make.
- No `api/auth.go`, `api/d8*.go`, or other auth/D8 file was edited;
  `api/auth.go:189` is cited read-only as the `RequireAuth`
  definition site. `api/d3_terminate.go` is byte-identical: the
  foreign classes keep the current gate behavior, and the lane
  invents no local judgments for classes core would judge.
- Unverified by this step: production runtime behavior (no server
  started; the suites above ran green), core-side handler behavior
  behind each forward, and any census numbering authority outside
  this document's snapshot-order binding. The #324 lane validates
  `updated_after` / `updated_before` with the core-identical 400s
  and applies the window through the reader store: the pinned
  apilib v0.2.1 filter surface carries both bounds, and the store
  renders the core-exact `updated_at` range (>= after inclusive,
  < before exclusive). The #325 lane validates `repository_id` /
  `source_run_id` with the core-identical 400 and applies the
  pair plus the type gate through the reader store: the pinned
  apilib v0.3.0 filter surface carries both plus the
  core-exact `created_at ASC, decision_id ASC` page and the
  per-decision evidence fan-out.

## Migration verdict (ADR-0002 test)

Five of 22 are migrated under ADR-0002's test
(`docs/decisions/0002-eshu-api-owns-api-compute.md:53-57`:
migrated means `eshu-api` executes the handler against a store with
zero core HTTP calls; forwarding labelled `viaNative` is not
termination):
- #171 serves the pinned static body with zero core HTTP calls and
  zero store contact (no-store class; the ADR's "against a store"
  leg is vacuous for the static report — the evidence is the P-FLOW
  span silence plus the counting-transport zero in
  `api/admin_tuning_terminate_test.go`).
- #335 executes the pinned apilib work-items query against the
  reader-role PG store with zero core HTTP calls (store-backed
  class — the evidence is the P-READ case with >= 1
  postgresql/read span plus the counting-transport zero in
  `api/admin_workitems_terminate_test.go`, and the live PG proof of
  the bounded SELECT in `TestAdminReadLiveProof`).
- #324 executes the pinned apilib dead-letters query against the
  reader-role PG store with zero core HTTP calls (store-backed
  class — the evidence is the P-READ case with >= 1
  postgresql/read span plus the counting-transport zero in
  `api/admin_deadletters_terminate_test.go`, the required-field /
  bounds-error parity in
  `TestAdminDeadLettersRequiredFieldsParity`, the windowed
  success threading in `TestAdminDeadLettersBoundedListParity`,
  and the live PG proof of the bounded join SELECT (windowed) in
  `TestAdminReadDeadLettersLiveProof`).
- #325 executes the pinned apilib decisions query against the
  reader-role PG store with zero core HTTP calls (store-backed
  class — the evidence is the P-READ case with >= 2
  postgresql/read spans, one for the list plus one for the evidence
  fan-out, plus the counting-transport zero in
  `api/admin_decisions_terminate_test.go`, the required-field
  parity in `TestAdminDecisionsRequiredFieldsParity`, the
  evidence-join parity in `TestAdminDecisionsBoundedListParity`,
  and the live PG proof of the bounded decision SELECTs plus the
  SELECT-only grant on both tables in
  `TestAdminReadDecisionsLiveProof`).
- #170 executes the pinned apilib reindex-progress read against the
  reader-role PG store with zero core HTTP calls (store-backed
  class — the evidence is the P-READ case with >= 1
  postgresql/read span plus the counting-transport zero in
  `api/admin_reindex_progress_terminate_test.go`, the
  empty/in-progress shape parity in
  `TestAdminReindexProgressShapeParity`, and the live PG proof of
  the three bounded SELECTs plus the SELECT-only grant on all four
  tables in `TestAdminReadReindexProgressLiveProof`).
The other 17 rows still end at `forwardToCore`
(`api/dispatch.go:252`; lane tables `## Frozen baseline table`).
The config surface is `--core-base-url`
(`cmd/eshu-api/main.go`, required flag) plus `ESHU_API_KEY`
(`cmd/eshu-api/main.go:26`; `docs/reference/configuration.md`),
plus the reader-role `ESHU_API_ADMIN_READ_DSN` (and optional
`ESHU_API_ADMIN_READ_MAX_CONNS`) that only the #335, #324, and #325
lanes read (`internal/adminread`, wired once in `cmd/eshu-api/main.go` —
one pool serves all four lanes;
`docs/reference/configuration.md`): no writer credential exists in
`eshu-api`, and none may be granted merely because the ADR exists
(`0002:12-13`).

## ADR-verbatim per-op columns (P0 census, `0002:113-114`)

Provenance rule: the Lane/Wave/Registration/`forwardToCore`
columns above are proven in this repo. The
family/store/credential/read-mutation values below are imported
from the W1 core census (read-only inspection of
`/home/ubuntu/personal-repos/eshu` at `083351b`; core seam
`CoreServeDeps` at `go/internal/query/serve.go:43`,
`NewCoreServeMux` at `go/internal/query/serve.go:254`,
admin `Mount` at `go/internal/query/admin/handler.go:385`,
`readStore` at `go/internal/query/admin/handler.go:367-374`,
static `tuningReport` at `go/internal/query/admin/handler.go:464` —
all re-verified by the coordinator). They are flagged
core-inspected (source only) and gate-implied in this repo, not
proven here. Credential class per op is UNKNOWN beyond "shared
key + core backstop re-judges" (allScopes hard-coded true, deny
branches unreachable). Any cell without a file:line below is
UNKNOWN, never inferred from the route name.

| # | Operation | Family (core) | Store (core) | Credential class | Read / mutation (core) |
| --- | --- | --- | --- | --- | --- |
| 169 | `GET /api/v0/admin/delta-union/completeness` | A trio | PG (active facts/generations) + Neo4j read-only | UNKNOWN beyond shared key + core backstop | read-only; explicitly never repairs |
| 170 | `GET /api/v0/admin/reindex/progress` | A trio | PG-only reads (via writer-handle maintenance stores) | UNKNOWN beyond shared key + core backstop | read; no write |
| 171 | `GET /api/v0/admin/shared-projection/tuning-report` | A trio | none (static pinned constants) | UNKNOWN beyond shared key + core backstop | read; no store |
| 319 | `POST /api/v0/admin/changed-since/poisoned-links/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 321 | `POST /api/v0/admin/collector-failures/acknowledgements/query` | C terminal-failure | PG read via Acknowledger (writer store, not `ReadStore`) | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 322 | `POST /api/v0/admin/collector-failures/query` | C terminal-failure | PG read via Acknowledger (writer store, not `ReadStore`) | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 324 | `POST /api/v0/admin/dead-letters/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 325 | `POST /api/v0/admin/decisions/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 326 | `POST /api/v0/admin/input-invalid-facts/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 332 | `POST /api/v0/admin/replay-events/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 335 | `POST /api/v0/admin/work-items/query` | B query | PG-only bounded read via `readStore` | UNKNOWN beyond shared key + core backstop | read-only candidate |
| 318 | `POST /api/v0/admin/backfill` | F recovery/D7 | PG writer (single INSERT `fact_backfill_requests`) | UNKNOWN beyond shared key + core backstop | mutation; NOT idempotent — re-run records a second request (`go/internal/query/admin/facts.go:264-265`, re-verified) |
| 320 | `POST /api/v0/admin/collector-failures/acknowledge` | C terminal-failure | PG writer (one READ COMMITTED tx, ack-row upsert) + shared `admin_replay_requests` ledger claim/complete + per-decision governance audit | UNKNOWN beyond shared key + core backstop | mutation; key + reason mandatory |
| 323 | `POST /api/v0/admin/dead-letter` | F recovery/D7 | PG writer (single autocommit UPDATE, ordered `FOR NO KEY UPDATE` lock + status recheck) | UNKNOWN beyond shared key + core backstop | mutation; state-idempotent; no key; no success audit |
| 327 | `POST /api/v0/admin/recover-generations` | F recovery | PG writer (one tx: reducer claim fence + projector work + reindex request) with mandatory key + ledger + audit | UNKNOWN beyond shared key + core backstop | mutation; key + reason + ledger + audit |
| 328 | `POST /api/v0/admin/refinalize` | F recovery | PG writer (same one-tx Refinalize as recover-generations) but NO key and NO governance audit (telemetry only) | UNKNOWN beyond shared key + core backstop | mutation; no key; no audit — asymmetry with recover-generations is intentional and must not be smoothed over |
| 329 | `POST /api/v0/admin/reindex` | F recovery/D7 | PG writer (watermark upserts only; effect async in ingesters) | UNKNOWN beyond shared key + core backstop | mutation; no key; no audit |
| 330 | `POST /api/v0/admin/reopen` | F recovery | PG writer (ledger + one tx, `SKIP LOCKED` + scope `FOR UPDATE`) | UNKNOWN beyond shared key + core backstop | mutation; ledger-claimed |
| 331 | `POST /api/v0/admin/replay` | F recovery | PG writer (ledger-claimed; work-items UPDATE + replay-event INSERT are NOT one atomic unit — `in_progress` stranded-on-error is by design) | UNKNOWN beyond shared key + core backstop | mutation; ledger-claimed |
| 333 | `POST /api/v0/admin/repository-retirements` | D retirement | PG-only synchronously (catalog resolve is `ingestion_scopes` SQL despite ContentReader name); mark (`ON CONFLICT` same-key resume) -> fence (one tx, advisory + EXCLUSIVE `fact_work_items` lock) -> purge (per-repo tx, NOWAIT) is a resumable sequence, NOT one transaction; graph retraction deferred (keys captured in PG) | UNKNOWN beyond shared key + core backstop | mutation; key + reason mandatory; per-repo audit |
| 334 | `POST /api/v0/admin/skip` | F recovery/D7 | PG writer (single-statement UPDATE, same lock order as dead-letter + EvalPlanQual recheck); denial-only audit | UNKNOWN beyond shared key + core backstop | mutation; state-idempotent; no key |
| 336 | `POST /api/v0/admin/workflow-retire-unclaimed` | E retire-unclaimed | PG workflow work-items (distinct table/domain from `fact_work_items`); autocommit gate + `SKIP LOCKED` retire; audit accepted/denied; NO idempotency key | UNKNOWN beyond shared key + core backstop | mutation; state-idempotent (repeat = no-op); NOT ledger-backed |

Cross-cutting, carried verbatim from the core census: synchronous
Neo4j appears only in delta-union; every other graph effect is
asynchronous downstream (projector/reducer consuming PG queue
rows). Synchronous ContentStore: none. Audit everywhere is
best-effort and nil-disabled
(`go/internal/query/serve_adapters_core.go:41-49`, re-verified:
a reader without append support yields nil and the action
proceeds) — never claim audit is transactional with the mutation.

## Provisional split (candidates only, no phase assignment)

- 11 reads: 3 trio GETs (1 still wave-gated + #171 and #170
  promoted) + 8 query POSTs (5 still wave-gated + #335, #324, and
  #325 promoted). #171, #335, #324, #325, and #170 are migrated
  (proofs in the verdict section above); the remaining 6 stay
  read-candidates.
- 11 P6-candidates: 4 D7 + 4 recovery + acknowledge +
  retirement + retire-unclaimed.
- The eshu-api gate split and the core read/mutation split agree
  numerically, but agreement is not proof. Before any phase
  assignment (P2-P4 reads / P6 mutations, `0002:121-132`) each op
  needs (i) core handler proof of store + mutation effect at a
  pinned eshu SHA, (ii) no-reachable-`forwardToCore` census plus
  runtime zero-core-origin/store-span evidence per `0002:61-63`
  once a library slice exists, and (iii) for mutations,
  tx/lock/idempotency/audit behavior re-verified by test, not
  source reading. Until then the labels stay "candidate", and the
  D3-provisional (`contracts/d3-admin-reads.md:44-54`, blocker B1)
  / D7-tier-C (`contracts/d7-admin-four.md:201-208`) caveats
  above stay in force.

## P5 dependency and carve-out decision (open, not assumed)

All 22 sit behind shared-key gates with unreachable scoped
denies today. ADR-0002 moves the full auth matrix atomically at
P5 (`0002:125-130`) before P6 mutations. Open decision for
owner/coordinator: does any admin read family migrate in P2-P4
ahead of P5, and if so which credential classes does it carry? A
forwarding exception for any row is valid only as an
owner-granted carve-out named in this census (`0002:160-161`);
silence means no carve-out. Carve-outs granted as of this
baseline: none.

## P0-output placeholders (TBD, no invented numbers)

- Per-store pool budgets (PG reader/writer, Neo4j — global
  arithmetic x max replicas per `0002:86-88`; read-replica
  eligibility per op's consistency contract per `0002:89-93`):
  TBD-P0 for every store except the #335/#324/#325/#170 reader
  line, filled below. Delta-union is the only op flagged PG+graph
  synchronous; tuning-report is flagged no-store; all other pool
  rows stay TBD until the store cells above are core-proven per op.
  - #335/#324/#325/#170 reader line (replica line, ops-qa scale carried
    from the sessionstore 002 worked example: server
    max_connections 100, max replicas 1, surge 1, no HPA; the
    reader line carries no standing core-api budget and the same
    ops headroom of 10): 5 x (1 + 1) + 0 + 10 = 20 <= 100. One pool
    serves all four lanes (`cmd/eshu-api/main.go` wires the same
    store four times), so the arithmetic is unchanged by #325.
    Validated in code by `internal/adminread` (`ReaderBudget`,
    `TestReaderBudgetDefaultValidates`: total 20, oversized
    ceilings refused) and proven against live PG by
    `TestAdminReadLiveProof` (#335),
    `TestAdminReadDeadLettersLiveProof` (#324, join included),
    `TestAdminReadDecisionsLiveProof` (#325, decision pair plus
    evidence fan-out), and
    `TestAdminReadReindexProgressLiveProof` (#170, three bounded
    SELECTs). The sizing assumes the reader DSN rides the read
    replica (the `RoleSessionRO` precedent); a DSN pointed at the
    primary must be re-sized against the primary line instead — an
    explicit re-sizing step, not a silent reuse.
- Security-design checklist (credential classes in phase order
  `0002:142-146`; secret references/rotation/audit;
  internet-facing credential-holder risk `0002:165-168`):
  TBD-P0 (unchanged by #335/#324/#325/#170: shared-key only,
  secretKeyRef-mounted DSN, SELECT-only role on `fact_work_items`
  plus `ingestion_scopes` for the join plus `projection_decisions`
  and `projection_decision_evidence` for the decisions read plus
  `runtime_ingester_control`, `repository_reindex_requests`, and
  `scope_generations` for the progress read, no writer
  credential).

## Unverified list (retained)

- Census numbering authority: #169-171 / #318-336 are
  census-local (snapshot carries no IDs; counts reconcile 3+19).
- Served 279 vs mounted 284 (`disclosed_absent` 5) not
  investigated; none of the 22 are among the absent (all 22
  verified present in `served_operations`).
- No production runtime yet (no server started); core
  tx/lock/idempotency claims are source-inspected at eshu
  `083351b` with no live PG/Neo4j or test run. The #335, #324,
  #325, and #170 lane suites ran green in this step (`go test ./api/
  ./cmd/... ./internal/...`, plus the env-gated
  `TestAdminReadLiveProof`, `TestAdminReadDeadLettersLiveProof`,
  `TestAdminReadDecisionsLiveProof`, and
  `TestAdminReadReindexProgressLiveProof` against local PG);
  every other row's wave-freeze tests were re-run, not just read.
