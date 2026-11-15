# Code-investigation relay-leg termination

W6c code-investigation 2, terminated (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#98`). `POST
/api/v0/code/structure/inventory` and `POST
/api/v0/code/topics/investigate` terminate dest-side inside the W6c
investigation wave gate, ahead of the generic proxy.

## Pin

`CodeContentInvestigationReadAdmission v0`, recorded in
`contracts/code-investigation-read-admission.md` at the W6 remainder
mint (`81b756fd84`, the verified core merge of
`eshu-hq/eshu#10688`). The lane consumes only this narrow pin: the
two structural-inventory reads (`InspectStructuralInventory`,
`CountStructuralInventoryByFile`) plus the one topic investigation
(`InvestigateCodeTopic`) — the exact 3 re-declared methods, with
`Run`, `RunSingle`, and every other graph and content leg sealed
OUT. The lane cites no other pin, and no other family may cite this
pin as coverage.

## Census (per route per method)

| Method | `POST /api/v0/code/structure/inventory` | `POST /api/v0/code/topics/investigate` |
|---|---|---|
| `InspectStructuralInventory` | default-kind window hydration | — (OUT by construction) |
| `CountStructuralInventoryByFile` | `function_count_by_file` kind only | — (OUT by construction) |
| `InvestigateCodeTopic` | — (OUT by construction) | ranked evidence page, incl. the deadline-partial shape |

Reachability notes, recorded not silently assumed: the inventory
investigate leg is a window hydration and runs no stat stream; the
topic point lookup reads its scored rows directly; the empty grant
is a blocked page built, not zero-valued — the inventory branch
answers an allocated empty `results` slice so a grantless caller is
indistinguishable from an empty index; the topic deadline carries
its bounded partial rows alongside the marker error with
`deadline_truncated` set, while every other failure drops the rows.

## Serving

- Shared-key (`AllScopes`) callers terminate on the native lane:
  the family authorizer
  (`authorizeCodeInvestigationSharedKey`) runs first, then the call
  crosses the versioned port transport (`forwardToCore` on the
  native lane) with method, path, query, body, negotiation headers,
  and credential intact; core bytes come back unmodified. Byte
  parity with the generic relay is pinned by
  `TestCodeInvestigationTerminatedMatchesRelayByteForByte` across
  the documented outcomes (inventory entity and
  function-count-by-file 200s with the allocated empty-grant
  blocked page, topic ranked 200 with the deadline-partial shape,
  400 validation/selector denials, 403 grant-denied, capability
  501s, fixed-message 500s, configuration-state and retryable 503s
  with `Retry-After`, gateway 504) in both negotiated shapes, for
  both routes and every driven body shape (entity and file-count
  inventories, plain and intent-scoped topics).
- Credential classes dest cannot resolve relay explicitly until
  D8-grade parity, and core judges them there:
  - scoped API tokens (`Bearer [REDACTED]`, any non-key bearer),
  - OIDC bearers (`Bearer eyJ...` carrying an OIDC `iss`),
  - browser sessions (`eshu_session` / `__Host-eshu_session`
    cookies). The explicit relay carries the caller's credential,
  method, path, query, and body to the same core route untouched;
  dest invents no 401, 403, 404, 409, 422, or 500 for these classes
  (`TestCodeInvestigationWaveRelayExplicitForUnresolvableIdentities`).
- Credential-less callers — and junk-cookie carriers, per the D2
  follow-up lesson — get the standing dest 401 with zero core
  contact. No public carve-out exists in this lane: the routes stay
  inside RequireAuth, keyless callers keep the 401, and no
  public-registration change rides this lane.
- Decision 2: NOT OWED in this lane. There is no tier-C mutation on
  the W6c surface — both wave entries are read-only POSTs, and the
  parity corpus carries no mutation outcome
  (`TestCodeInvestigationDecision2NotOwed` pins the read-only wave
  and the read-outcome status set).

## D0 / wiring evidence

- `TestCodeInvestigationWaveFrozen`: the wave is exactly the two
  pinned W6c POSTs; nothing else may enter it silently. The table is
  a NEW wave, never an extension of `codeGraphWave`,
  `deadCodeWave`, `codeDivergenceWave`, `relationshipWave`, or
  `codeScanWave`: those pins cover their own rows only. Consuming
  this pin for any other route is a NO-GO (arbiter supplemental
  ruling, §2B).
- `TestCodeInvestigationWaveAheadOfRelay` (+ slash spellings):
  shared-key calls file their dispatch on the native lane ahead of
  the generic proxy, with request bodies forwarded intact across
  entity, file-count, plain-topic, and intent-scoped shapes.
  Removing the `registerCodeInvestigationWave` call returns the
  routes to the generic relay, which this test pins as RED (relay
  row, no native row).
- `TestCodeInvestigationAuthorizerCountersPinned`: one allow
  judgment on the `code_investigation` family per terminated call;
  the deny branch stays observable
  (`TestCodeInvestigationDenyBranchShapePinned` pins its 403 shape
  with zero core contact).
- Resilience: dead-core single-attempt 503, pre-cancel and
  mid-flight cancellation inherit the shared dispatch floor (the lane
  adds no retry), and each leg forwards exactly once per caller.
- Spec scope: the routes mount natively, but their Decision 5(c)
  fragments stay core-side for this lane; mounted 225 -> 227,
  disclosed absences 51 -> 53.

## Spec posture

Both W6c POSTs mount natively, but their Decision 5(c) fragments
stay core-side for this lane. The entries live in one core file —
`go/internal/query/openapi/paths/code/symbols.go` (the
`/code/structure/inventory` and `/code/topics/investigate` entries
at `symbols.go:59` and `symbols.go:312`). Porting an entry without
its referenced schemas would break the verbatim-port rule
`scripts/check-parity.sh` gates — so, like the gap-04 Compare, W6a
Code-flow, W4 search, G-a Code-graph, Wave-4 Drift, dead-code,
divergence, G-b, and W6b relay-leg lanes, the routes are disclosed
spec absences, not a port. A follow-up spec-port lane claims the two
keys when it ports the entries entry-shaped with their referenced
schemas at a pinned base; until then the served document is unchanged
by this lane.

## Rollback

Revert-to-relay: remove the
`s.registerCodeInvestigationWave(mux)` call in `routes()` (or the
wave/gate/terminate/test files with this contract), and the two W6c
POSTs return to the generic relay behind RequireAuth. The spec
absences stay disclosed either way (the routes leave the mounted set
with the lane); no data migrates, and no other family moves.
