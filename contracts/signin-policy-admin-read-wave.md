# W7-read admin sign-in-policy GET termination contract

Versioned read-termination record for the W7-read lane (Refs
`eshu-hq/eshu#9669`, D13 tracker `eshu-hq/eshu#10491`, satellite
`eshu-hq/eshu-api#72`). This lane promotes exactly one route from
the generic D2 Leaf B wave gate to family ownership — the admin
sign-in-policy GET — and terminates it natively against the
D0-proven terminated transport: dest owns the route (family gate,
family authorizer, response mapping) and calls the versioned core
read API as its port transport. Dest holds no table or queue handle
and synthesizes nothing on this leg; authz stays atomic core-side.

Scope source: the SignInPolicyReadAdmission v0 pin (core
`eshu-hq/eshu#10432`, O6 pin 1; satellite record
`contracts/upstream/sign_in_policy_read_admission_contract_test.go`,
gated byte-identical by `scripts/check-parity.sh`). The pin admits
the admin GET through its single re-declared read
(`GetSignInPolicy`); the public sign-in-policy GET and the providers
require_sso half already terminate in the signinpolicy-group lane
(`contracts/signinpolicy-termination.md`). The core pin record
(`contracts/signin-policy-read-admission.md` at core, W7-read pin
commit) proves the admin-leg verbatim coverage: the admin detail
wire is the byte-exact projection of the pinned read's return
value, so forwarding core bytes serves the leg verbatim by
construction.

Transport pins:

- `SignInPolicyReadAdmission v0`
  (`contracts/upstream/sign_in_policy_read_admission_contract_test.go`
  at the O6 pin SHA, plus the
  `contracts/upstream/sign-in-policy-read-wire-shapes.txt` excerpt)
  covers the admin row — the leg admits through `GetSignInPolicy`.

The minting rule forbids inventing pins beyond it: the providers
list read (provider-config enumeration behind
`GET /api/v0/auth/providers`) stays explicitly out per the pin
record, and the sign-in-policy mutation (`PATCH
/api/v0/auth/admin/sign-in-policy`) stays on its RequireAuth tier-C
write-through under `SignInPolicyAdmission` v0 — neither rides this
lane.

## What this lane proves per the D-wave ruling

Explicit route ahead of relay, family authorizer + deny-shape tests
with the scope-honesty label, byte-parity dispatch/conformance tests
vs the relayed reference (both spellings, both negotiated shapes,
core-owned outcomes relayed verbatim), the no-core-import proof, the
no-store/no-parse lane-file hygiene proof, extended
`viaNative`/`viaRelay` telemetry, the satellite latency evidence
(terminated-vs-still-relaying ratio, both spellings), and this
contract section with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API,
  bounded client (30s, no redirect, no retry; `forwardToCore`
  streams the body unbounded), forwarded credential and negotiation
  headers, guaranteed correlation ID, fail-closed local 503
  (permanent, never `Retry-After`) when no core origin is
  configured.
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Dest-synthesized errors stay envelope-aware
  via `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The family authorizer
  (`authorizeSignInPolicyAdmin`, shared with the PATCH
  write-through — one trust root for the family's authed legs)
  allows it; core re-judges every forwarded call as backstop (tenant
  scope, permission catalog) — which is also where the live 403s
  come from. Dest adds no handler validation and emits no audit:
  request/response semantics stay core-owned. Scoped tokens, OIDC
  bearers, and browser sessions relay explicitly so core judges
  them.
- Reads emit no governance audit events on either path (in-tree
  reads are deliberately out of audit scope; see the core handler
  convention) — no audit write to keep atomic.

## Per-family record

Scope-honesty label (sign_in_policy family, admin leg): the dest
authorizer covers the shared-key (AllScopes) identity only. Scoped
tokens, OIDC bearers, and browser sessions relay explicitly — core
judges, core bytes come back untouched, dest invents no
401/403/404/500 for them — until the D8 session resolver lands. The
deny branch (`allScopes=false`) is pinned for that day and
unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| SignInPolicyReads (admin leg) | 1 GET (`auth/admin/sign-in-policy`, exact path + twin) | family-gated terminate / relay-explicit | shared-key allow (family authorizer shared with PATCH) |

Cookie rule: the family gate reuses the D2 name-matched session
check (`hasLeafBSessionCookie` — session name, non-empty value), so
the credential matrix is bit-identical to the LeafB loop the route
leaves. Only session-cookie-shaped credentials relay; any other
Cookie content is junk, not a browser session, and stays dest-401
with zero core contact.

Known divergences (disclosed, not hidden):

- The LeafB census keeps the row with the `wave-family` serve kind;
  the gated loop skips it and the family registration serves it.
- Non-GET spellings of the path are not registered: a keyless PATCH
  keeps the standing 401 at the key gate; a keyed PATCH crosses the
  tier-C write-through (unchanged); other keyed methods cross the
  generic relay and core judges the method.
- Deeper subtrees (`/api/v0/auth/admin/sign-in-policy/...`) are not
  registered: keyless callers keep the standing 401; keyed callers
  get core's answer via the generic relay.
- Unconfigured origin answers the permanent local 503 on the family
  route where nil-safe core handlers answer zero-defaults — the
  standing fail-closed rule, uniform across native lanes.

## Spec posture

No spec change in this lane: the admin sign-in-policy fragment
(`openapi/paths/auth/sign_in_policy.go`, byte-identical port of the
pinned core fragment per `scripts/check-parity.sh`) already carries
the route, and the assembled document key set is unchanged
(`TestSetupSpecPosture`-adjacent suites stay green untouched).

## Rollback

Single-commit revert: remove the `s.registerAdminSignInPolicyRead(mux)`
call in `api/router.go` (with `api/signinpolicy_admin_terminate.go`
and its tests), restore the admin row's serve kind to `wave-gated`
in `api/leafb_wave.go` with the LeafB census tests, and the lane
reverts to the generic Leaf B wave gate with zero state change
(core untouched, no durable dest state, no data migration). Verify
by re-running the lane conformance suite against the reverted tree:
the family assertions go red (route rides the LeafB gate again)
while every other suite stays green.
