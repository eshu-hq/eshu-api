# Code-cypher relay-leg termination

W6d code-cypher 2, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#98`). `POST /api/v0/code/cypher` and `POST
/api/v0/code/visualize` terminate dest-side inside the W6d wave gate,
ahead of the generic proxy.

## Pin

`CodeCypherReadAdmission v0`, recorded in
`contracts/code-cypher-read-admission.md` at the W6 remainder mint
(`81b756fd84`, the verified core merge of `eshu-hq/eshu#10688`).
The lane consumes only this narrow pin: the bounded graph traversal
(`Run`) — the exact 1 re-declared method, with `RunSingle` and every
other `GraphQuery` leg sealed OUT. The lane cites no other pin, and
no other family may cite this pin as coverage.

## Census (per route per method)

| Method | `POST /api/v0/code/cypher` | `POST /api/v0/code/visualize` |
|---|---|---|
| `Run` | bounded read-only query (`boundedReadOnlyCypher`); raw result rows | bounded query (`boundedVisualizationCypher`); rows projected to a renderable visualization packet |

Reachability notes, recorded not silently assumed: both routes
execute caller-supplied graph programs with no bounded selector to
intersect against a grant — shared-key/all-scope callers only;
scoped and browser-session tokens are refused core-side before any
handler runs (`x-shared-key-only`, `auth.SharedKeyOnlyRoutes`). The
mutation-keyword rejection, the injected terminal `LIMIT`, the
`cypher_query is required` gate, and the statement-rejection 400
all answer before `Run`; the visualize packet marks row-window
truncation with its limitation reason; both routes time out under
the route timeout against an `AccessModeRead` session.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeCypherSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeCypherTerminatedMatchesRelayByteForByte` across the
  documented outcomes (cypher raw-row 200s, visualize packet 200s
  incl. truncation-limitation and no-entities shapes, 400
  validation/keyword/LIMIT/rejection denials, 403 grant-denied,
  capability 501s, fixed-message 500s, retryable 503 with
  `Retry-After`, gateway 504) in both negotiated shapes, for both
  routes and every driven body shape (plain and limit-pinned
  queries).
- The shared-key-only grant-class gate is ported by the wave gate:
  any other credential class relays explicitly so core refuses it
  before any handler runs — and dest invents no refusal for it:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeCypherWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation on
  the W6d surface — both wave entries are read-only POSTs, and the
  parity corpus carries no mutation outcome
  (`TestCodeCypherDecision2NotOwed` pins the read-only wave and the
  read-outcome status set).

## D0 / wiring evidence

- `TestCodeCypherWaveFrozen`: the wave is exactly the two pinned
  W6d POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `codeGraphWave`, `deadCodeWave`,
  `codeDivergenceWave`, `relationshipWave`, `codeScanWave`,
  `codeInvestigationWave`, or `codeSecretsWave`: those pins cover
  their own rows only. Consuming this pin for any other route is a
  NO-GO (arbiter supplemental ruling, §2D).
- `TestCodeCypherWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  plain and limit-pinned shapes. Removing the
  `registerCodeCypherWave` call returns the routes to the generic
  relay, which this test pins as RED (relay row, no native row).
- `TestCodeCypherAuthorizerCountersPinned`: one allow judgment
  on the `code_cypher` family per terminated call; the deny
  branch stays observable
  (`TestCodeCypherDenyBranchShapePinned` pins its 403 shape with
  zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation inherit the shared dispatch floor (the lane
  adds no retry), and each leg forwards exactly once per caller.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 228 -> 230,
  disclosed absences 54 -> 56.

## Spec posture

Both W6d POSTs mount natively, but their Decision 5(c) fragments
stay core-side for this lane. The entries live in one core file —
`go/internal/query/openapi/paths/code/graph.go` (the
`/code/cypher` and `/code/visualize` entries). Porting an entry
without its referenced schemas would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, G-a Code-graph, Wave-4 Drift, dead-code,
divergence, G-b, W6b, and W6c relay-leg lanes, the routes are
disclosed spec absences, not a port. A follow-up spec-port lane
claims the two keys when it ports the entries entry-shaped with
their referenced schemas at a pinned base; until then the served
document is unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeCypherWave(mux)`
call in `routes()` (or the wave/gate/terminate/test files with this
contract), and the two W6d POSTs return to the generic relay behind
RequireAuth. The spec absences stay disclosed either way (the routes
leave the mounted set with the lane); no data migrates, and no other
family moves.
