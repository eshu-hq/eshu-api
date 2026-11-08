# D3 admin/identity/recovery reads termination contract

Versioned read-termination record for the D3 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`, plus the Gap 02
follow-up for satellite `eshu-hq/eshu-api#70`). This lane terminates
the 16 D1-pinned read routes plus the 3 Gap 02-pinned Admin GETs
natively against the D0-proven terminated transport: dest owns the
route (caller gate, family authorizer, response mapping) and calls
the versioned core read API as its port transport. Dest holds no
table or queue handle; authz stays atomic core-side. The trio lane
claims no D1 `ReadStore` coverage for the three GETs — their pins
live in `contracts/admin-trio-read-admission.md`.

Scope source: `go/cmd/api/CUTOVER_OWNERSHIP.md` at the D2 lane base
plus the four D1 pins recorded in `contracts/admin-read-admission.md`,
`contracts/admin-identity-admission.md`,
`contracts/governance-audit-read-admission.md`, and
`contracts/terminal-failure-admission.md`, plus the three Gap 02 pins
recorded in `contracts/admin-trio-read-admission.md` (core
`eshu-hq/eshu#10412`, merge `4a587431fd`) — no guessing. Excluded and
staying relay (disclosed, not silent):

- the families D2 already terminated (provider-config reads, admin
  sign-in-policy).

Terminated since the Gap 02 trio lane (no longer relay):

- the 3 Admin GETs (`delta-union/completeness`,
  `reindex/progress`, `shared-projection/tuning-report`): pinned by
  `DeltaUnionCompletenessReadAdmission v0` (`Check`),
  `ReindexProgressReadAdmission v0` (the 3-method alias), and
  `SharedProjectionTuningReport v0` (the pinned `status`/`detail`
  constants the handler serves), all recorded byte-identical. Proof:
  the trio wave table (`api/d3trio_wave.go`), the same D3 wave gate
  and family authorizer with deny-shape tests
  (`api/d3trio_conformance_test.go`, `api/d3trio_auth_test.go`),
  byte-parity dispatch/conformance proof vs the relayed reference
  (`api/d3trio_parity_test.go`), per-route latency
  (`api/d3trio_latency_test.go`, `docs/d3-latency.md`), and the
  line-verbatim spec entries (`openapi/paths/status/`
  `delta_union_completeness.go`, `reindex_progress.go`,
  `admin_tuning_report.go`).

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
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Where core renders flat (its `WriteJSON`
  admin/identity/recovery reads), an envelope-negotiated caller still
  receives the flat shape with truth inline (`result_limits`,
  pagination, group fields); where core negotiates (its `WriteSuccess`
  routes), the envelope flows through untouched. Dest invents no
  truth levels (it holds no read model); dest-synthesized errors stay
  envelope-aware via `writeError`.
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
| Admin (Gap 02 trio) | 3 GETs (`admin/delta-union/completeness`, `admin/reindex/progress`, `admin/shared-projection/tuning-report`) | wave-gated terminate / relay-explicit | shared-key allow (Gap 02 pins v0, no D1 ReadStore claim) |
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
- The 3 Gap 02 Admin GETs keep their own fragment keys
  (`delta_union_completeness.go`, `reindex_progress.go`,
  `admin_tuning_report.go` — the D3 AdminReads fragment still
  carries only the six query POSTs); no served key duplicates a
  relayed route.
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
