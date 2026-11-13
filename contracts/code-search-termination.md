# Code-search relay-leg termination

W4 Code search 2, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#113`). `POST /api/v0/code/search` and
`POST /api/v0/code/symbols/search` terminate dest-side inside the W4
wave gate, ahead of the generic proxy.

## Pin

`CodeSearchReadAdmission v0`, recorded in
`contracts/code-search-read-admission.md` at the W4 base
(`e504e328a7`, the verified core HEAD; pins minted at `330828359a`,
merge of `eshu-hq/eshu#10540`). The lane consumes only this narrow
pin: the graph lane (`Run`) plus the eight content-reader legs
(`SearchEntityNames`, `SearchSymbols`, `SearchCodeCandidates`,
`SearchEntitiesByName`, `SearchEntityContent`,
`SearchEntitiesByNameAnyRepo`, `SearchEntityContentAnyRepo`,
`ListRepoEntitiesByIDs`) — the exact 9 re-declared methods, with
`RunSingle` sealed OUT. The lane cites no other pin, and no other
family may cite this pin as coverage. There is deliberately no
`WithReadStore` constructor on this pin: dest serves the reads through
the existing graph reader plus the guarded Postgres content reader,
the same pair `cmd/api` wiring builds today; any other backend pairing
needs a new arbiter ruling. The grant checks (`codeGrantAccessFilter`,
`codeContentGrantScope`), the repository selector, the fixed failure
messages, the hybrid rerank, the semantic overlay (`entitysemantics`
summaries), and `QueryProfile` stay handler behavior, not port shape —
core judges them on the forwarded call.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeSearchSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeSearchTerminatedMatchesRelayByteForByte` across the
  fragment's documented outcomes (200 found and empty, 400
  validation/malformed, 403 grant-denied, fixed-message 500,
  store-unavailable 503, graph-deadline 504, retryable 503 with
  `Retry-After`, unfollowed redirect) in both negotiated shapes, for
  both routes and every driven body shape (graph, global-index,
  language/exact candidate, and fallback shapes; symbol-aware and
  plain-name fallback shapes).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, or 500 for these classes
  (`TestCodeSearchWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.

## D0 / wiring evidence

- `TestCodeSearchWaveFrozen`: the wave is exactly the two pinned code
  search POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `codeFlowWave`: the W6a flow pin covers
  flow reads only. Consuming this pin for content routes or any other
  `/code/*` route is a NO-GO (arbiter W4 ruling).
- `TestCodeSearchWaveAheadOfRelay` (+ slash spellings): shared-key
  calls file their dispatch on the native lane ahead of the generic
  proxy, with request bodies forwarded intact across graph,
  global-index, candidate, fallback, and symbol shapes. Removing the
  `registerCodeSearchWave` call returns the routes to the generic
  relay, which this test pins as RED (relay row, no native row).
- `TestCodeSearchAuthorizerCountersPinned`: one allow judgment on
  the `code_search` family per terminated call; the deny branch stays
  observable (`TestCodeSearchDenyBranchShapePinned` pins its 403
  shape with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and mid-flight
  cancellation without retry, and a 16-caller parallel probe (one
  forward per caller).
- Latency: `BenchmarkCodeSearchTerminatedPOST` vs
  `BenchmarkCodeSearchRelayPOST` bound the gate + authorizer
  overhead; both legs do one server-side fetch (see
  `docs/code-search-latency.md` for the measured run). The relay
  comparison leg is the still-relayed `POST /api/v0/code/bundles` —
  no existing latency leg uses either terminated route as a relay leg,
  so no re-point is needed.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 201 -> 203,
  disclosed absences 31 -> 33.

## Spec posture

The two search POSTs mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in two core
files — `go/internal/query/openapi/paths/code/routes.go` (the
`/code/search` entry, first-in-file in a seven-key block) and
`go/internal/query/openapi/paths/code/symbols.go` (the
`/code/symbols/search` entry, first-in-file in a five-key block) —
and they reference schemas the served document does not carry yet
(`CodeSearchResponse`, `SymbolSearchResponse`, `RankCursorConflict`).
Porting one entry without its referenced blocks, or re-shaping a
satellite subset across two fragments, would break the verbatim-port
rule `scripts/check-parity.sh` gates — so, like the gap-04 Compare
and W6a Code-flow relay-leg lanes, the routes are disclosed spec
absences (pinned by `TestSpec_CodeSearchStaysDisclosedAbsent`),
not a port. A follow-up spec-port lane claims both keys when it ports
the two entries entry-shaped with their referenced schemas at a pinned
base; until then the served document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeSearchWave(mux)` call in
`routes()` (or the wave/gate/terminate/test files with this
contract), and the two code search POSTs return to the generic relay
behind RequireAuth. The spec absences stay disclosed either way (the
routes leave the mounted set with the lane); no data migrates, and no
other family moves.
