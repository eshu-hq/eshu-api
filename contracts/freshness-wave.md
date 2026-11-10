# Wave-1 Freshness reads termination contract

Versioned read-termination record for the Wave-1 Freshness wave (Refs
`eshu-hq/eshu#9669`, D13 tracker `eshu-hq/eshu#10491`, satellite
`eshu-hq/eshu-api#72`). This lane terminates exactly the 3 Freshness
GETs natively against the D0-proven terminated transport: dest owns
the route (caller gate, family authorizer, response mapping) and calls
the versioned core read API as its port transport. Dest holds no table
or queue handle; authz stays atomic core-side.

Scope source: the FreshnessReadAdmission v0 pin
(`contracts/freshness-read-admission.md` at core `51b84b2`, merge of
`eshu-hq/eshu#10494`): the bounded generation-lifecycle page plus the
two bounded changed-since delta summaries behind the freshness
drilldown routes.

Transport pins:

- `FreshnessReadAdmission v0`
  (`contracts/upstream/freshness_read_admission_contract_test.go` at
  the Wave-1 merge SHA, plus the
  `contracts/upstream/freshness-read-wire-shapes.txt` excerpt) covers
  all 3 wave rows — every row admits through its re-declared port
  method: `GET /api/v0/freshness/changed-since` through
  `ComputeChangedSinceDelta`, `GET
  /api/v0/freshness/services/changed-since` through
  `ComputeServiceChangedSinceDelta`, `GET
  /api/v0/freshness/generations` through `ListGenerationLifecycle`.

The minting rule forbids inventing pins beyond it: the changed-since
link-ledger read path (`LedgerReadEnabled`, #7127 PR-3c), the
stored-summary reader (`WithSummaryReader`, #7009), and the snapshot
legs (`ReadStatusSnapshot`, `ReadStatusSnapshotFiltered`,
`ReadRawSnapshot` — covered by `status.StatusReadAdmission` v0) stay
explicitly out per the pin record and stay relay with the reason
disclosed per route below.

## Freshness

What this lane proves per the D-wave ruling is the required proof
list: explicit routes ahead of relay, byte-identical fragments under
`scripts/check-parity.sh`, authorizer + deny-shape tests with the
scope-honesty label, byte-parity dispatch/conformance tests vs the
relayed reference (including the filter-query forwarding cases the pin
shape requires), the no-core-import check, extended
`viaNative`/`viaRelay` telemetry, the satellite latency evidence
(terminated-vs-proxy-relay ratios, no same-shape before/after: the
baseline is the still-relaying proxy route), and this contract section
with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API
  (`CoreReadContractsVersion v0`), bounded client (30s, no redirect,
  no retry; no dest-side byte budget — `forwardToCore` streams the
  body unbounded), forwarded credential and negotiation headers,
  guaranteed correlation ID, fail-closed local 503 (permanent, never
  `Retry-After`) when no core origin is configured.
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Dest-synthesized errors stay envelope-aware
  via `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation and
  emits no audit: request/response semantics stay core-owned. Scoped
  tokens receive only granted repositories and scopes per the pin
  fragments; an ungranted selector is core's not-found, and an
  all-scope bearer with no grant is core's 403 under hosted modes —
  the dest gate relays those classes explicitly so core judges them.
- Reads emit no governance audit events on either path: the in-tree
  freshness handlers gate on permission features but never append, so
  there is no audit write to keep atomic; idempotency is the GET
  repeat check in the conformance matrix.

## Per-family record

Scope-honesty label (freshness family): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| Freshness | 3 GETs (`freshness/changed-since`, `freshness/generations`, `freshness/services/changed-since`) | wave-gated terminate / relay-explicit | shared-key allow |

Cookie rule (D2 follow-up lesson, shared with the D3/D4/D6 gates): only
session-cookie-shaped credentials relay — a Cookie member named
`eshu_session` (or `__Host-eshu_session`, the two names the core
caller-credential reader accepts) relays explicitly; any other Cookie
content is junk, not a browser session, and stays dest-401 with zero
core contact.

Known divergences (disclosed, not hidden):

- No Wave-1 route stays relay: all 3 pinned GETs terminate. The
  out-of-pin legs stay out with no wire change: the changed-since
  link-ledger read path serves ledger-off (the pin's constructor
  default), the stored-summary reader is never acquired, and the
  snapshot legs stay covered by `status.StatusReadAdmission` v0.
- The earlier-lane freshness-path reads keep their own lanes and
  families and never enter this wave: `repositories/{repo_id}/freshness`
  (D4, repositories family), `documentation/evidence-packets/{packet_id}/freshness`
  (D4, documentation family), `status/freshness-causality` (D6, status
  family). The new `/freshness` family prefix does not hijack them
  (pinned in `TestFreshnessOtherFreshnessPathsNotInWave`).
- Non-GET spellings of the 3 paths are not registered: a keyless POST
  keeps the standing 401 at the key gate; a keyed POST crosses the
  generic relay and core judges the method (core allowlists these
  routes GET-only).
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.

## Spec posture

The 3 Wave-1 fragments are whole-file byte-identical ports of their
standalone core fragments (all unshaped entries — none is a
paths-closing entry shaped like the Compare fragment, so no disclosed
absence was needed): `ChangedSince`, `Generations`, and
`ServiceChangedSince` land in `openapi/paths/freshness/` and assemble
right after the freshness causality entry. No fragment carries a
`"security": []` public-route marker: all 3 are authed reader-port
reads, and the posture test (`TestSetupSpecPosture`) is unaffected —
the only served marker stays the permanently-public login-picker entry
(`GET /api/v0/auth/providers`).

## Rollback

Single-commit revert: remove the `s.registerFreshnessWave(mux)` call
in `api/router.go` (with `api/freshness_wave.go`,
`api/freshness_terminate.go`, and their tests) and the lane reverts to
the generic byte-relay with zero state change (core untouched, no
durable dest state, no data migration). The served spec drops the 3
keys with the fragments; the relay legs keep serving them from core.
Verify by re-running the lane conformance suite against the reverted
tree: the ahead-of-relay assertions go red (routes relay again) while
every other suite stays green. No Plan A rehearsal is consumed by this
rollback — it restores a proxy path, not durable state.
