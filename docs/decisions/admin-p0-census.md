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
`auth/admin/audit` GETs, `api/d3_wave.go:72-79`), and all
`auth/admin` mutations. They do not appear in the 22-row table.

## The two lanes (definitions)

- **d3ReadGate-gated** — registered as
  `s.d3ReadGate(http.HandlerFunc(s.serveD3Terminated))`
  (`api/router.go:301` in the D3 loop, `api/router.go:310` in the
  trio loop). A valid shared key terminates natively; a non-key
  Bearer or session-cookie-shaped Cookie relays explicitly;
  credential-less and junk-cookie callers get the standing 401
  (`api/d3_terminate.go:121-167`). Terminated calls forward at
  `api/d3_terminate.go:96` (`forwardToCore(s, w, r, viaNative)`);
  explicit relays forward at `api/d3_terminate.go:185` (`viaRelay`).
  Family authorizer: `authorizeD3SharedKey`
  (`api/d3_terminate.go:48-50`), invoked in `serveD3TerminatedAs`
  (`api/d3_terminate.go:84`).
- **d3ReadGate-gated, locally terminated (#171 only)** — the
  tuning-report GET keeps the d3ReadGate credential matrix above but
  terminates locally: registered on its own anchor ahead of the D3
  loops (`s.registerAdminTuningReport`, `api/router.go:283-285`;
  route const and handler `api/admin_tuning_terminate.go:52-109`),
  shared-key calls serve the pinned apilib static body
  (`apilibadmin.WriteTuningReport`, `api/admin_tuning_terminate.go:109`)
  filing `local_ok` on the native lane
  (`api/admin_tuning_terminate.go:108`) with zero core and zero
  store contact, foreign classes relay explicitly on the untouched
  gate, credential-less callers keep the standing 401. Family
  authorizer: the shared `authorizeD3SharedKey` trust root
  (`api/d3_terminate.go:48-50`), judged in
  `serveAdminTuningTerminatedAs` (`api/admin_tuning_terminate.go:93`).
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

#169–170 are the remaining Gap 02 wave routes: wave entries
`api/d3trio_wave.go:65-66` (`Serve: d3ServeGated`), registered by the
loop at `api/router.go:308-312` via `s.d3ReadGate`
(`api/router.go:310`). #171 promoted to its own native lane in
eshu-hq/eshu-api#171 (no wave entry; anchor
`api/router.go:283-285`). Contract record:
`contracts/d3-admin-reads.md:26-42`, `:98-99` (Gap 02 pins v0, no D1
`ReadStore` claim); the #171 termination proof lives in
`api/admin_tuning_terminate_test.go` (P-FLOW zero-core case, census
`AssertNoCorePath`, byte-identical body pin).

| # | Operation | Lane | Wave entry | Registration | forwardToCore site |
| --- | --- | --- | --- | --- | --- |
| 169 | `GET /api/v0/admin/delta-union/completeness` | d3ReadGate-gated | `api/d3trio_wave.go:65` | `api/router.go:308-312` via `s.d3ReadGate` (`api/router.go:310`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 170 | `GET /api/v0/admin/reindex/progress` | d3ReadGate-gated | `api/d3trio_wave.go:66` | `api/router.go:308-312` via `s.d3ReadGate` (`api/router.go:310`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 171 | `GET /api/v0/admin/shared-projection/tuning-report` | d3ReadGate-gated, locally terminated | none (promoted; `api/admin_tuning_terminate.go:52`) | `api/router.go:283-285` via `s.d3ReadGate` + `serveAdminTuningTerminated` | none — local `apilibadmin.WriteTuningReport` (`local_ok`); foreign classes relay `api/d3_terminate.go:185` |

## Frozen baseline table — 19 POSTs (#318–336)

### D3-gated query POSTs (8 of 19)

Wave entries in `d3Wave` (`Serve: d3ServeGated`,
`api/d3_wave.go:46`), registered by the loop at
`api/router.go:299-303` through `s.d3ReadGate` (`api/router.go:301`).
Same forward sites as the GETs. Contract rows:
`contracts/d3-admin-reads.md:98` (Admin, 6 POSTs) and
`contracts/d3-admin-reads.md:102` (TerminalFailure, 2 POSTs).

| # | Operation | Lane | Wave entry | Registration | forwardToCore site |
| --- | --- | --- | --- | --- | --- |
| 319 | `POST /api/v0/admin/changed-since/poisoned-links/query` | d3ReadGate-gated | `api/d3_wave.go:70` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 321 | `POST /api/v0/admin/collector-failures/acknowledgements/query` | d3ReadGate-gated | `api/d3_wave.go:80` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 322 | `POST /api/v0/admin/collector-failures/query` | d3ReadGate-gated | `api/d3_wave.go:81` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 324 | `POST /api/v0/admin/dead-letters/query` | d3ReadGate-gated | `api/d3_wave.go:68` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 325 | `POST /api/v0/admin/decisions/query` | d3ReadGate-gated | `api/d3_wave.go:67` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 326 | `POST /api/v0/admin/input-invalid-facts/query` | d3ReadGate-gated | `api/d3_wave.go:69` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 332 | `POST /api/v0/admin/replay-events/query` | d3ReadGate-gated | `api/d3_wave.go:71` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |
| 335 | `POST /api/v0/admin/work-items/query` | d3ReadGate-gated | `api/d3_wave.go:66` | `api/router.go:299-303` via `s.d3ReadGate` (`api/router.go:301`) | `api/d3_terminate.go:96` (`viaNative`); relay `api/d3_terminate.go:185` |

### RequireAuth-native mutation POSTs (11 of 19)

| # | Operation | Lane | Route definition | Registration | forwardToCore site | Family authorizer |
| --- | --- | --- | --- | --- | --- | --- |
| 318 | `POST /api/v0/admin/backfill` | RequireAuth-native (D7 tier-C) | `api/d7.go:36`; listed `api/d7.go:44-49` | `api/router.go:165-168` loop over `d7Routes`, `RequireAuth(... s.d7WriteThrough)` | `api/d7.go:148` in `d7WriteThroughAs` (`api/d7.go:126-149`) | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 320 | `POST /api/v0/admin/collector-failures/acknowledge` | RequireAuth-native (tier C-a) | `api/terminalfailure.go:17` | `api/native_singles.go:51-52` via `registerNativeSingles` (called `api/router.go:93`) | `api/terminalfailure.go:73` | `authorizeTerminalFailureAdmin` `api/terminalfailure.go:84-86`, checked `api/terminalfailure.go:68` |
| 323 | `POST /api/v0/admin/dead-letter` | RequireAuth-native (D7 tier-C) | `api/d7.go:37`; listed `api/d7.go:44-49` | `api/router.go:165-168` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 327 | `POST /api/v0/admin/recover-generations` | RequireAuth-native (tier C-c2) | `api/recovery.go:30`; listed `api/recovery.go:35-40` | `api/router.go:256-259` loop over `recoveryRoutes`, `RequireAuth(... s.recoveryWriteThrough)` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 328 | `POST /api/v0/admin/refinalize` | RequireAuth-native (tier C-c2) | `api/recovery.go:29`; listed `api/recovery.go:35-40` | `api/router.go:256-259` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 329 | `POST /api/v0/admin/reindex` | RequireAuth-native (D7 tier-C) | `api/d7.go:38`; listed `api/d7.go:44-49` | `api/router.go:165-168` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 330 | `POST /api/v0/admin/reopen` | RequireAuth-native (tier C-c2) | `api/recovery.go:28`; listed `api/recovery.go:35-40` | `api/router.go:256-259` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 331 | `POST /api/v0/admin/replay` | RequireAuth-native (tier C-c2) | `api/recovery.go:27`; listed `api/recovery.go:35-40` | `api/router.go:256-259` | `api/recovery.go:104` | `authorizeRecoveryAdmin` `api/recovery.go:115-117`, checked `api/recovery.go:99` |
| 333 | `POST /api/v0/admin/repository-retirements` | RequireAuth-native (tier C-a) | `api/retirement.go:15` | `api/native_singles.go:43-44` via `registerNativeSingles` (called `api/router.go:93`) | `api/retirement.go:60` | `authorizeRetirementAdmin` `api/retirement.go:70-72`, checked `api/retirement.go:55` |
| 334 | `POST /api/v0/admin/skip` | RequireAuth-native (D7 tier-C) | `api/d7.go:39`; listed `api/d7.go:44-49` | `api/router.go:165-168` | `api/d7.go:148` | `authorizeD7Admin` `api/d7.go:159-161`, checked `api/d7.go:136` |
| 336 | `POST /api/v0/admin/workflow-retire-unclaimed` | RequireAuth-native (writer) | `api/retireunclaimed.go:16` | `api/router.go:95-96` direct `mux.Handle`, `RequireAuth(... s.retireUnclaimedWriteThrough)` | `api/retireunclaimed.go:88` | `authorizeRetireUnclaimedAdmin` `api/retireunclaimed.go:99-101`, checked `api/retireunclaimed.go:76` |

Contract records for the native rows: D7 four —
`contracts/d7-admin-four.md:151-165` (routes inside `RequireAuth`,
tier-C dispatch only, not termination); recovery four — handler
record `api/recovery.go:10-25`; acknowledge —
`api/terminalfailure.go:10-17`; retirement — `api/retirement.go:10-15`;
retire-unclaimed — `api/retireunclaimed.go:10-16`.

## Reconciliation

- 22 = 10 d3ReadGate-gated (2 wave GETs + 8 query POSTs) + 1
  d3ReadGate-gated, locally terminated (#171) + 11
  RequireAuth-native (4 D7 + 4 recovery + acknowledge + retirement
  + retire-unclaimed). No census operation rides the generic
  byte-relay as its primary lane today; relay appears only as the
  d3ReadGate explicit-relay branch for unresolvable credential
  classes (`api/d3_terminate.go:175-186`), which #171 keeps for its
  foreign classes.
- Every native handler passes `allScopes=true` today (e.g.
  `api/d7.go:115-117`, `api/recovery.go:81-83`,
  `api/terminalfailure.go:50-52`, `api/retirement.go:37-39`,
  `api/retireunclaimed.go:54-56`); every family deny branch is
  documented unreachable until a scoped/session resolver lands.
  Core re-judges every forwarded call as backstop on both lanes.
- Registration-order note (observed, not a defect claim): the
  recovery block comment sits at `api/router.go:152-158` directly
  above the D7 loop (`api/router.go:165-168`), while the recovery
  loop itself registers later, uncommented, at
  `api/router.go:256-259` — after the D8/D9 blocks. Registration
  order in `routes()` for census rows is therefore: native singles
  (`api/router.go:93`) → retire-unclaimed (`api/router.go:95-96`) →
  D7 (`api/router.go:165-168`) → recovery (`api/router.go:256-259`)
  → tuning-report anchor (`api/router.go:283-285`) → D3 wave
  (`api/router.go:299-303`) → D3 trio (`api/router.go:308-312`), all
  ahead of the generic proxy. The anchor precedes the loops so the
  promoted route never falls through to a wave registration; the
  mux would panic on the duplicate pattern if the wave entry had
  not been deleted with the promotion.
- Wave-freeze tests pinning the gated tables exist by reference in
  the wave files (`TestD3WaveFrozen`, `api/d3_wave.go:60-64`;
  `TestD3TrioWaveFrozen`, `api/d3trio_wave.go:58-67`); the promoted
  tuning-report lane pins its own freeze in
  `api/admin_tuning_terminate_test.go`
  (`TestAdminTuningReportRouteFrozen`,
  `TestAdminTuningReportTerminatesLocally`); this census
  ran no test suite (docs-only step; `make pre-pr` / `pre-push` are
  coordinator-only).

## Consequences / what this does not claim

- This is a lane census, not a termination or removal-readiness
  claim for any row. D7 rows stay tier-C dispatch per
  `contracts/d7-admin-four.md:201-208`; the remaining D3 wave rows
  stay provisional per `contracts/d3-admin-reads.md:44-54`
  (blocker B1). #171 terminated locally under its own proof
  (`api/admin_tuning_terminate_test.go`); its core-side removal
  readiness is a separate claim this census does not make.
- No `api/auth.go`, `api/d8*.go`, or other auth/D8 file was edited;
  `api/auth.go:189` is cited read-only as the `RequireAuth`
  definition site.
- Unverified by this step: runtime behavior (no server started, no
  tests run), core-side handler behavior behind each forward, and
  any census numbering authority outside this document's
  snapshot-order binding.

## Migration verdict (ADR-0002 test)

One of 22 is migrated under ADR-0002's test
(`docs/decisions/0002-eshu-api-owns-api-compute.md:53-57`:
migrated means `eshu-api` executes the handler against a store with
zero core HTTP calls; forwarding labelled `viaNative` is not
termination): #171 serves the pinned static body with zero core HTTP
calls and zero store contact (no-store class; the ADR's "against a
store" leg is vacuous for the static report — the evidence is the
P-FLOW span silence plus the counting-transport zero in
`api/admin_tuning_terminate_test.go`). The other 21 rows still end
at `forwardToCore` (`api/dispatch.go:252`; lane tables
`## Frozen baseline table`).
The config surface is only `--core-base-url`
(`cmd/eshu-api/main.go:49`, required at `:57`) plus `ESHU_API_KEY`
(`cmd/eshu-api/main.go:21`; `docs/reference/configuration.md:12-13`):
no store credential exists in `eshu-api` today, and none may be
granted merely because the ADR exists (`0002:12-13`).

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

- 11 read-candidates: 3 trio GETs + 8 d3-gated query POSTs.
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
  TBD-P0 for every store. Delta-union is the only op flagged
  PG+graph synchronous; tuning-report is flagged no-store; all
  other pool rows stay TBD until the store cells above are
  core-proven per op.
- Security-design checklist (credential classes in phase order
  `0002:142-146`; secret references/rotation/audit;
  internet-facing credential-holder risk `0002:165-168`):
  TBD-P0.

## Unverified list (retained)

- Census numbering authority: #169-171 / #318-336 are
  census-local (snapshot carries no IDs; counts reconcile 3+19).
- Served 279 vs mounted 284 (`disclosed_absent` 5) not
  investigated; none of the 22 are among the absent (all 22
  verified present in `served_operations`).
- No runtime or test execution yet (docs-only step); core
  tx/lock/idempotency claims are source-inspected at eshu
  `083351b` with no live PG/Neo4j or test run; wave-freeze tests
  read, not run.
