# Code-graph relay-leg termination

Wave-7 G-a graph 4, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#121`). `POST /api/v0/code/call-chain`, `POST
/api/v0/code/call-chain/compare`, `POST
/api/v0/code/call-graph/metrics`, and `POST
/api/v0/code/routes/callers` terminate dest-side inside the G-a wave
gate, ahead of the generic proxy.

## Pin

`CodeGraphReadAdmission v0`, recorded in
`contracts/code-graph-read-admission.md` at the G-a mint
(`0fd6db6`, the verified core merge of `eshu-hq/eshu#10645`). The lane
consumes only this narrow pin: the bounded graph traversal (`Run`,
`RunSingle`) plus the entity-name resolution
(`SearchEntitiesByName`) — the exact 3 re-declared methods, with
point/list reads, file-content legs, ledger/summary/snapshot surface,
and every other `GraphQuery`/`ContentStore` leg sealed OUT. The lane
cites no other pin, and no other family may cite this pin as coverage.
Citing `MetricsReadAdmission v0` as coverage for the metrics POST
here is void: that pin seals the graph leg out explicitly. There is
deliberately no `WithReadStore` constructor on this pin: dest serves
the reads through the existing graph reader plus the guarded Postgres
content reader, the same pair `cmd/api` wiring builds today; any
other backend pairing needs a new arbiter ruling. The grant checks
(`codeGrantAccessFilter`, `codeContentGrantScope`), the
unsupported-language short-circuit, the fixed failure messages,
endpoint disambiguation, and `QueryProfile` stay handler behavior,
not port shape — core judges them on the forwarded call.

## Census (per route per method)

| Method | `POST /api/v0/code/call-chain` | `POST /api/v0/code/call-chain/compare` | `POST /api/v0/code/call-graph/metrics` | `POST /api/v0/code/routes/callers` |
|---|---|---|---|---|
| `Run` | bounded shortest-chain traversal | one-hop BFS expansion reads | bounded edge scan (50k-edge sentinel) | endpoint + handler + directional traversal reads |
| `RunSingle` | single-row graph reads on the same leg | single-row graph reads on the same leg | single-row metric reads | single-row reads on the same legs |
| `SearchEntitiesByName` | endpoint name resolution, incl. the `start_repo_id` / `end_repo_id` cross_repo selectors | endpoint name resolution (single-repo) | — (metric rows carry no name lookup; OUT by construction) | handler label resolution behind the traversal |

Reachability notes, recorded not silently assumed: the metrics leg
runs no name lookup by construction; the compare leg refuses
`cross_repo` (single-repo v1: `repo_id` required); the call-chain leg
is the only leg that admits the cross_repo endpoint pair; the callers
leg resolves the handler label through the content reader behind the
traversal.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeGraphSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeGraphTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 found and scoped-empty zero
  page, 200 paths/empty/truncated-budget, 200
  complete/partial/unsupported traces, 400
  validation/selector/disambiguation, 403 grant-denied, 404 endpoint
  and route not-found, 409 ambiguous selector, 422 scope-too-broad
  with no partial rows, capability and read-model 501s,
  fixed-message 500s, backend-unavailable 503, retryable 503 with
  `Retry-After`, gateway 504) in both negotiated shapes, for all
  four routes and every driven body shape (repo-scoped, entity-id,
  cross_repo endpoint selectors, single-repo compare caps, metric
  types with paging, repo and service caller scopes).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeGraphWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestCodeGraphWaveFrozen`: the wave is exactly the four pinned G-a
  graph POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `w8aWave`: the W8a pins cover their
  own 16 rows only. Consuming this pin for the
  code-graph-metrics-adjacent time-series GET or any other route is
  a NO-GO (arbiter Wave-7 ruling).
- `TestCodeGraphWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact across repo-scoped,
  entity-id, cross_repo, compare-cap, metric-page, and caller-scope
  shapes. Removing the `registerCodeGraphWave` call returns the
  routes to the generic relay, which this test pins as RED (relay
  row, no native row).
- `TestCodeGraphAuthorizerCountersPinned`: one allow judgment on the
  `code_graph` family per terminated call; the deny branch stays
  observable (`TestCodeGraphDenyBranchShapePinned` pins its 403
  shape with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe across
  the four legs (one forward per caller).
- Latency: `BenchmarkCodeGraphTerminatedRead` (one leg per wave
  route) vs `BenchmarkCodeGraphRelayRead` (still-relayed POST, on
  the generic proxy); every leg does one server-side fetch (see
  `docs/code-graph-latency.md` for the measured run). The relay leg
  rides `POST /api/v0/code/bundles` — no existing latency leg covers
  a terminated route, so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 209 -> 213,
  disclosed absences 35 -> 39.

## Spec posture

The four graph POSTs mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in three
core files — `go/internal/query/openapi/paths/code/routes.go` (the
`/code/call-chain` and `/code/call-chain/compare` entries),
`go/internal/query/openapi/paths/code/symbols.go` (the
`/code/call-graph/metrics` entry), and
`go/internal/query/openapi/paths/code/route_to_caller.go` (the
`/code/routes/callers` entry) — and the call-chain, compare, and
metrics entries reference the `EntityRef` schema the served document
does not carry yet. Porting an entry without its
referenced blocks, or re-shaping a satellite subset across three
fragments, would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, and Wave-4 Drift relay-leg lanes, the routes
are disclosed spec absences (pinned by
`TestSpec_CodeGraphStaysDisclosedAbsent`), not a port. A follow-up
spec-port lane claims the four keys when it ports the entries
entry-shaped with their referenced schemas at a pinned base; until
then the served document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeGraphWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the four graph POSTs return to the generic relay
behind RequireAuth. The spec absences stay disclosed either way (the
routes leave the mounted set with the lane); no data migrates, and no
other family moves.
