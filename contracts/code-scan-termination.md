# Code-scan relay-leg termination

W6b code-scan 4, terminated (Refs `eshu-hq/eshu#9669`, satellite
`eshu-hq/eshu-api#98`). `POST /api/v0/code/complexity`, `POST
/api/v0/code/quality/inspect`, `POST
/api/v0/code/imports/investigate`, and `POST /api/v0/code/bundles`
terminate dest-side inside the W6b wave gate, ahead of the generic
proxy.

## Pin

`CodeGraphScanReadAdmission v0`, recorded in
`contracts/code-scan-read-admission.md` at the W6 remainder mint
(`81b756fd84`, the verified core merge of `eshu-hq/eshu#10688`).
The lane consumes only this narrow pin: the bounded graph traversal
(`Run`) plus the by-ID entity hydration
(`ListRepoEntitiesByIDs`) — the exact 2 re-declared methods, with
`RunSingle`, every other `GraphQuery` leg, and every other
`ContentStore` leg sealed OUT. The lane cites no other pin, and no
other family may cite this pin as coverage. Citing
`CodeGraphReadAdmission v0` as coverage for any row here is void
(that pin seals the W6b scan legs out explicitly). There is
deliberately no `WithReadStore` constructor on this pin: dest serves
the reads through the existing graph reader plus the guarded Postgres
content reader, the same pair `cmd/api` wiring builds today; any
other backend pairing needs a new arbiter ruling. The grant checks,
the grant-bound builders, the repository selector, the fixed failure
messages, and QueryProfile stay handler behavior, not port shape —
core judges them on the forwarded call.

## Census (per route per method)

| Method | `POST /api/v0/code/complexity` | `POST /api/v0/code/quality/inspect` | `POST /api/v0/code/imports/investigate` | `POST /api/v0/code/bundles` |
|---|---|---|---|---|
| `Run` | by-name, by-ID, and ranked-list builders | bounded inspect scan | imports leaf dispatch | anchor plus version counts |
| `ListRepoEntitiesByIDs` | by-ID branch enrichment hydration | — (no hydration leg; OUT by construction) | — (no hydration leg; OUT by construction) | — (no hydration leg; OUT by construction) |

Reachability notes, recorded not silently assumed: the complexity
list branch runs only when neither `entity_id` nor `function_name`
is supplied; the by-ID enrichment hydrates only the by-ID branch;
the complexity stale-id fallback answers only for an unbound lookup
(an unrestricted whole-index miss); the imports grantless branch
answers the empty page with no read; the bundles grantless branch
answers the zero-row page with no graph call, and a scoped caller
reads only `visibility = 'public'` packages.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer (`authorizeCodeScanSharedKey`) runs first,
  then the call crosses the versioned port transport
  (`forwardToCore` on the native lane) with method, path, query,
  body, negotiation headers, and credential intact; core bytes come
  back unmodified. Byte parity with the generic relay is pinned by
  `TestCodeScanTerminatedMatchesRelayByteForByte` across the
  documented outcomes (complexity by-name / by-ID / list /
  stale-id-fallback 200s, grantless list and entity pages, quality
  inspect 200 with thresholds and recommended calls, imports
  investigation 200 with the grantless empty page, bundles 200 with
  the public-only and grantless pages, 400 validation/selector
  denials, 403 grant-denied, 404 unknown entity, 409 ambiguity
  rule, 422 scope-too-broad bound, capability 501s, fixed-message
  500s, retryable 503 with `Retry-After`, gateway 504) in both
  negotiated shapes, for all four routes and every driven body shape
  (by-name, by-ID, ranked list, checks, file and module scopes,
  query and ecosystem scopes).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeScanWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation on
  the W6b surface — all four wave entries are read-only POSTs, and
  the parity corpus carries no mutation outcome
  (`TestCodeScanDecision2NotOwed` pins the read-only wave and the
  read-outcome status set).

## D0 / wiring evidence

- `TestCodeScanWaveFrozen`: the wave is exactly the four pinned
  W6b POSTs; nothing else may enter it silently. The table is a NEW
  wave, never an extension of `codeGraphWave`, `deadCodeWave`,
  `codeDivergenceWave`, or `relationshipWave`: those pins cover
  their own rows only. Consuming this pin for any other route is a
  NO-GO (arbiter supplemental ruling, §2A).
- `TestCodeScanWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  by-name, by-ID, list, check, file-scope, module-scope, query, and
  ecosystem shapes. Removing the `registerCodeScanWave` call returns
  the routes to the generic relay, which this test pins as RED
  (relay row, no native row).
- `TestCodeScanAuthorizerCountersPinned`: one allow judgment
  on the `code_scan` family per terminated call; the deny
  branch stays observable
  (`TestCodeScanDenyBranchShapePinned` pins its 403 shape with
  zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation inherit the shared dispatch floor (the lane
  adds no retry), and each leg forwards exactly once per caller.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 221 -> 225,
  disclosed absences 47 -> 51.

## Spec posture

All four W6b POSTs mount natively, but their Decision 5(c)
fragments stay core-side for this lane. The entries live in four
core files — `go/internal/query/openapi/paths/code/routes.go` (the
`/code/complexity` entry), `quality.go` (the
`/code/quality/inspect` entry), `symbols.go` (the
`/code/imports/investigate` entry), and `graph.go` (the
`/code/bundles` entry). Porting an entry without its referenced
schemas would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, G-a Code-graph, Wave-4 Drift, dead-code,
divergence, and G-b relay-leg lanes, the routes are disclosed spec
absences, not a port. A follow-up spec-port lane claims the four
keys when it ports the entries entry-shaped with their referenced
schemas at a pinned base; until then the served document is
unchanged by this lane.

## Rollback

Revert-to-relay: remove the `s.registerCodeScanWave(mux)`
call in `routes()` (or the wave/gate/terminate/test files with this
contract), and the four W6b POSTs return to the generic relay behind
RequireAuth. The spec absences stay disclosed either way (the routes
leave the mounted set with the lane); no data migrates, and no other
family moves.
