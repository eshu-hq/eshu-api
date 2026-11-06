# D3 admin/identity/recovery reads termination contract

Versioned read-termination record for the D3 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This lane terminates
the 16 D1-pinned read routes natively against the D0-proven
terminated transport: dest owns the route (caller gate, family
authorizer, response mapping) and calls the versioned core read API as
its port transport. Dest holds no table or queue handle; authz stays
atomic core-side.

Scope source: `go/cmd/api/CUTOVER_OWNERSHIP.md` at the D2 lane base
plus the four D1 pins recorded in `contracts/admin-read-admission.md`,
`contracts/admin-identity-admission.md`,
`contracts/governance-audit-read-admission.md`, and
`contracts/terminal-failure-admission.md` — no guessing. Excluded and
staying relay (disclosed, not silent):

- the 3 Admin GETs (`delta-union/completeness`, `reindex/progress`,
  `shared-projection/tuning-report`): their readers carry no
  versioned pin (`DeltaUnionCompletenessChecker`,
  `ReindexProgressReader`, the static tuning report), and the minting
  rule admits no blanket pins;
- the families D2 already terminated (provider-config reads, admin
  sign-in-policy).

## Status

Termination claims stay provisional until the arbiter re-reviews the
D0 evidence (blocker B1). What this lane proves per the D-wave ruling
is the required proof list: explicit routes ahead of relay,
byte-identical fragments under `scripts/check-parity.sh`, authorizer +
deny-shape tests with scope-honesty labels, byte-parity
dispatch/conformance tests vs the relayed reference, the no-core-import
check, extended `viaNative`/`viaRelay` telemetry, the satellite
latency gate with before/after numbers (GET 0.55x, POST 0.79x), and
this contract section with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API
  (`CoreReadContractsVersion v0`), bounded client (30s, no redirect,
  no retry; no dest-side byte budget — `forwardToCore` streams the
  body unbounded), forwarded credential and negotiation headers,
  guaranteed correlation ID, fail-closed local 503 (permanent, never
  `Retry-After`) when no core origin is configured.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation
  and emits no audit: request/response semantics stay core-owned.
- Reads emit no governance audit events on either path: the in-tree
  query handlers gate on permission features but never append (the
  ack-mutation audit stays core-side on the tier C-a acknowledgement
  route, which this lane does not touch), so there is no audit write
  to keep atomic; idempotency is the GET repeat check plus the POST
  query re-read in the conformance matrix.

## Per-family record

Scope-honesty label (all families): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| Admin | 6 POSTs (`admin/work-items/query`, `decisions/query`, `dead-letters/query`, `input-invalid-facts/query`, `changed-since/poisoned-links/query`, `replay-events/query`) | wave-gated terminate / relay-explicit | shared-key allow (AdminReadAdmission v0) |
| AdminIdentity | 6 GETs (`auth/admin/api-tokens`, `idp-group-mappings`, `idp-providers`, `role-assignments`, `roles`, `auth/local/invitations`) | wave-gated terminate / relay-explicit | shared-key allow (AdminIdentityReadAdmission v0) |
| GovernanceAudit | 2 GETs (`auth/admin/audit/events`, `audit/summary`) | wave-gated terminate / relay-explicit | shared-key allow (GovernanceAuditReadAdmission v0) |
| TerminalFailure | 2 POSTs (`admin/collector-failures/query`, `acknowledgements/query`) | wave-gated terminate / relay-explicit | shared-key allow (TerminalFailureReadAdmission v0) |

Cookie rule (D2 follow-up lesson): only session-cookie-shaped
credentials relay — a Cookie member named `eshu_session` (or
`__Host-eshu_session`, the two names the core caller-credential reader
accepts) relays explicitly; any other Cookie content is junk, not a
browser session, and stays dest-401 with zero core contact.

Known divergences (disclosed, not hidden):

- The served `openapi.json` carries the invitations path key get-only:
  the create POST sharing the key stays relay until D8, so the served
  entry documents the landed method only. D8 merges the POST.
- The 3 unpinned Admin GETs keep their own path keys out of the D3
  fragment and stay relay; the served spec has no entry for them.
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.

## Spec posture

No D3 fragment carries a `"security": []` marker: every D3 read is an
all-scopes admin read the core serves authed, so the per-entry posture
test (`TestSetupSpecPosture_StaysCoreSide`, which fails on any served
marker outside the permanently-public login-picker entry) covers this
lane without edits. The next lane that ports another public-marked
fragment revisits that test again instead of landing silently.

## Rollback

Single-commit revert: remove the D3 wave registrations in
`api/router.go` and the lane reverts to the generic byte-relay with
zero state change (core untouched, no durable dest state, no data
migration). The 4 read pins stay valid (they are unwired seams
core-side; the satellite version constants keep recording them), so a
re-landing re-registers the same routes. Verify by re-running the lane
conformance suite against the reverted tree: the ahead-of-relay
assertions go red (routes relay again) while every other suite stays
green. No Plan A rehearsal is consumed by this rollback — it restores
a proxy path, not durable state.
