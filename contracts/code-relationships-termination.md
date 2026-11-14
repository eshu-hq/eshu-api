# Code-relationships relay-leg termination

G-b 2, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#122`). `POST /api/v0/code/relationships` and `POST
/api/v0/code/relationships/story` terminate dest-side inside the G-b
wave gate, ahead of the generic proxy.

## Pin

`CodeRelationshipReadAdmission v0`, recorded in
`contracts/code-relationships-read-admission.md` at the G-b mint
(`8ee32698`, the verified core merge of `eshu-hq/eshu#10669`). The
lane consumes only this narrow pin: the bounded graph traversals
(`Run`, `RunSingle`), the twelve ContentStore reads, and the three
grant-scoped reads — the exact 17 re-declared methods. The lane cites
no other pin, and no other family may cite this pin as coverage.

## Census (per route per method)

| Method | `POST /api/v0/code/relationships` | `POST /api/v0/code/relationships/story` |
|---|---|---|
| `Run` / `RunSingle` | bounded traversal; transitive CALLS walk | story graph tracks; class_hierarchy walk |
| Twelve ContentStore reads | resolution, repo-identity, enrichment, fallback, builder closure | story resolution, repo-identity, enrichment, builder closure |
| Three grant-scoped reads | scoped callers (two fail-closed; language differs by fallback) | scoped story callers (same) |

Reachability notes, recorded not silently assumed: the fallback leg
runs only when the graph row is nil (builder closure answers through
its four narrowed reads); the transitive walk runs only for
`relationship_type=CALLS` with a direction (otherwise 400); the story
`class_hierarchy` enrichment runs only for that query type on a class
target (otherwise 400); the repo-scoped overrides story answers
without entity resolution; scoped grant-blocked targets answer
`not_found` on the content basis, not a 403.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeRelationshipSharedKey`) runs
  first, then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestRelationshipTerminatedMatchesRelayByteForByte` across the
  documented outcomes (direct graph 200, content-fallback 200,
  transitive CALLS 200, scoped grant-clipped 200, story resolved /
  class_hierarchy / overrides / repo-scoped / ambiguous / not_found
  200s, 400 validation/selector denials, 403 grant-denied, 404 unknown
  entity, capability 501s, fixed-message 500s, configuration-state 503,
  retryable 503 with `Retry-After`, gateway 504) in both negotiated
  shapes, for both routes and every driven body shape (direct,
  name-fallback, transitive CALLS, scoped grant reads,
  class_hierarchy, overrides, repo-scoped overrides).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestRelationshipWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation on
  the G-b surface — both wave entries are read-only POSTs, and the
  parity corpus carries no mutation outcome
  (`TestRelationshipDecision2NotOwed` pins the read-only wave and the
  read-outcome status set).

## D0 / wiring evidence

- `TestRelationshipWaveFrozen`: the wave is exactly the two pinned
  G-b POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `codeGraphWave`, `deadCodeWave`, or
  `codeDivergenceWave`: those pins cover their own rows only.
  Consuming this pin for any other route is a NO-GO (arbiter
  SUFFICIENT ruling).
- `TestRelationshipWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  direct, fallback, transitive, scoped, hierarchy, and override
  shapes. Removing the `registerRelationshipWave` call returns the
  routes to the generic relay, which this test pins as RED (relay
  row, no native row).
- `TestRelationshipAuthorizerCountersPinned`: one allow judgment
  on the `code_relationships` family per terminated call; the deny
  branch stays observable
  (`TestRelationshipDenyBranchShapePinned` pins its 403 shape with
  zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation inherit the shared dispatch floor (the lane
  adds no retry), and each leg forwards exactly once per caller.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 219 -> 221,
  disclosed absences 45 -> 47.

## Spec posture

Both G-b POSTs mount natively, but their Decision 5(c) fragments stay
core-side for this lane. The entries live in one core file —
`go/internal/query/openapi/paths/code/routes.go` (the
`/code/relationships` and `/code/relationships/story` entries).
Porting an entry without its referenced schemas would break the
verbatim-port rule `scripts/check-parity.sh` gates — so, like the
gap-04 Compare, W6a Code-flow, W4 search, G-a Code-graph, Wave-4
Drift, dead-code, and divergence relay-leg lanes, the routes are
disclosed spec absences, not a port. A follow-up spec-port lane claims
the two keys when it ports the entries entry-shaped with their
referenced schemas at a pinned base; until then the served document
is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerRelationshipWave(mux)`
call in `routes()` (or the wave/gate/terminate/test files with this
contract), and the two G-b POSTs return to the generic relay behind
RequireAuth. The spec absences stay disclosed either way (the routes
leave the mounted set with the lane); no data migrates, and no other
family moves.
