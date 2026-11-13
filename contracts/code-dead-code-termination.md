# Dead-code relay-leg termination

Dead-code 3, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#124`). `POST /api/v0/code/dead-code`, `POST
/api/v0/code/dead-code/cross-repo`, and `POST
/api/v0/code/dead-code/investigate` terminate dest-side inside the
dead-code wave gate, ahead of the generic proxy.

## Pin

`CodeDeadCodeReadAdmission v0`, recorded in
`contracts/code-dead-code-read-admission.md` at the dead-code mint
(`589d55e`, the verified core merge of `eshu-hq/eshu#10648`). The lane
consumes only this narrow pin: the graph run behind the SQL-routine
EXECUTES probes plus the thirteen content reads (candidate,
incoming-edge, reachability, downgrade, hydration, coverage,
freshness, cross-repo consumer-evidence, boundary-evidence) — the
exact 14 re-declared methods, with `RunSingle`, search, list,
file-content, ledger/summary/snapshot surface, and every other
`GraphQuery`/`ContentReader` leg sealed OUT. The lane cites no other
pin, and no other family may cite this pin as coverage. Citing
`CodeGraphReadAdmission v0`, `CodeSearchReadAdmission v0`, or
`ContentSearchReadAdmission v0` as coverage here is void: those pins
seal the dead-code graph and content legs out explicitly. There is
deliberately no `WithReadStore` constructor on this pin: dest serves
the reads through the existing graph reader plus the guarded Postgres
content reader, the same pair `cmd/api` wiring builds today; any
other backend pairing needs a new arbiter ruling. The grant checks,
the repository selector, the fixed failure messages, the rerank, and
`QueryProfile` stay handler behavior, not port shape — core judges
them on the forwarded call.

## Census (per route per method)

| Method | `POST /api/v0/code/dead-code` | `POST /api/v0/code/dead-code/cross-repo` | `POST /api/v0/code/dead-code/investigate` |
|---|---|---|---|
| `DeadCodeCandidateRows` | candidate scan (all labels) | producer scan (repo-scoped) | candidate scan |
| `Run` | SQL-routine reachability probe + scoped incoming batch probe | scoped incoming batch probe | scoped incoming batch probe |
| `GetEntityContents` / `GetEntityContent` | candidate hydration | candidate hydration | candidate hydration |
| `DowngradedCodeRootKinds` | modeled-root downgrade | — (OUT by construction) | modeled-root downgrade |
| `DeadCodeIncomingEntityIDs` | incoming edges | — (grant-bound probe covers it) | incoming edges |
| `CodeReachabilityIncomingEntityIDs` | grant-bound incoming probe | grant-bound incoming probe | grant-bound incoming probe |
| `RepositoryContextCoverage` / `RepositoryFilesLastIndexedAt` / `RepositoryCoverage` | — (investigate-only; OUT by construction) | — | cheap freshness + full aggregate |
| `CrossRepoDeadCodeConsumerEvidence` / `CrossRepoDeadCodeConsumerCoverage` / `CrossRepoDeadCodeConsumerRootPaths` | — (cross-repo-only; OUT by construction) | consumer evidence, coverage, root paths | — |
| `RepositoryRelationshipReadModel` | — | boundary evidence | — |

Reachability notes, recorded not silently assumed: the scan leg runs
no coverage or evidence reads by construction; the cross-repo leg is
the only leg that admits the consumer-evidence triple plus the
boundary-evidence read; the investigate leg is the only leg that
admits the coverage triple; `RunSingle` is OUT everywhere by
construction.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeDeadCodeSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestDeadCodeTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 candidates/empty/truncated
  pages, 200 live_by_consumer/unknown/handles shapes, 200
  investigate packets, 400 validation/selector, 403 grant-denied,
  501 capability, fixed-message 500s, backend-unavailable 503,
  retryable 503 with `Retry-After`, gateway 504) in both negotiated
  shapes, for all three routes and every driven body shape
  (corpus-wide, repo-scoped, candidate-kind, consumer-scoped,
  handles detail).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer [REDACTED]` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestDeadCodeWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestDeadCodeWaveFrozen`: the wave is exactly the three pinned
  dead-code POSTs; nothing else may enter it silently. The table is a
  NEW wave, never an extension of `w8aWave`: the W8a pins cover their
  own 16 rows only. Consuming this pin for the search, list,
  file-content, or graph-entity surfaces is a NO-GO (arbiter
  SUPPLEMENTAL-PIN-OWED ruling).
- `TestDeadCodeWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact across corpus-wide,
  repo-scoped, candidate-kind, consumer-scoped, and handles shapes.
  Removing the `registerDeadCodeWave` call returns the routes to the
  generic relay, which this test pins as RED (relay row, no native
  row).
- `TestDeadCodeAuthorizerCountersPinned`: one allow judgment on the
  `code_dead_code` family per terminated call; the deny branch stays
  observable (`TestDeadCodeDenyBranchShapePinned` pins its 403 shape
  with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe across
  the three legs (one forward per caller).
- Latency: `BenchmarkDeadCodeTerminatedRead` (one leg per wave
  route) vs `BenchmarkDeadCodeRelayRead` (still-relayed POST, on the
  generic proxy); every leg does one server-side fetch (see
  `docs/dead-code-latency.md` for the measured run). The relay leg
  rides `POST /api/v0/code/bundles` — no existing latency leg covers
  a terminated route, so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 213 -> 216,
  disclosed absences 39 -> 42.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation
  survivor to split: all three wave rows are reads behind the same
  14-method pin, and the seal test holds the ledger/summary/snapshot
  exclusions core-side. No parity split is proven because none is
  owed.

## Spec posture

The three dead-code POSTs mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in three
core files — `go/internal/query/openapi/paths/code/dead/scan.go`
(the `/code/dead-code` entry),
`go/internal/query/openapi/paths/code/dead/cross_repo.go` (the
`/code/dead-code/cross-repo` entry), and
`go/internal/query/openapi/paths/code/dead/investigation.go` (the
`/code/dead-code/investigate` entry). Porting an entry without its
referenced blocks would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, Wave-7 G-a, and Wave-4 Drift relay-leg lanes,
the routes are disclosed spec absences (pinned by
`TestSpec_DeadCodeStaysDisclosedAbsent`), not a port. A follow-up
spec-port lane claims the three keys when it ports the entries
entry-shaped with their referenced schemas; until then the served
document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerDeadCodeWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the three dead-code POSTs return to the generic relay
behind RequireAuth. The spec absences stay disclosed either way (the
routes leave the mounted set with the lane); no data migrates, and no
other family moves.
