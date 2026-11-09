# Leaf B Native-now termination contract

Versioned read-termination record for the D2 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This lane terminates
the frozen 18 routes / 8 families natively against the D0-proven
terminated transport: dest owns the route (caller gate, family
authorizer, response mapping) and calls the versioned core read API as
its port transport. Dest holds no table or queue handle; authz stays
atomic core-side.

## Status

Termination claims stay provisional until the arbiter re-reviews the
D0 evidence (blocker B1). What this lane proves per the D-wave ruling
is the required proof list: explicit routes ahead of relay,
byte-identical fragments under `scripts/check-parity.sh`, authorizer +
deny-shape tests with scope-honesty labels, byte-parity
dispatch/conformance tests vs the relayed reference, the no-core-import
check, extended `viaNative`/`viaRelay` telemetry, and this contract
section with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API
  (`CoreReadContractsVersion v0`), bounded client (30s, no redirect,
  no retry; no dest-side byte budget — `forwardToCore` streams the
  body unbounded, the 256KiB budget lives only in the unused
  `internal/readclient`), forwarded credential and negotiation headers,
  guaranteed correlation ID, fail-closed local 503 (permanent, never
  `Retry-After`) when no core origin is configured.
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Where core renders flat (its `WriteJSON`
  admin reads), an envelope-negotiated caller still receives the flat
  shape with truth inline (`result_limits`, pagination, group
  fields); where core negotiates (its `WriteSuccess` routes), the
  envelope flows through untouched. Dest invents no truth levels (it
  holds no read model); dest-synthesized errors stay envelope-aware
  via `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation and
  emits no audit: request/response semantics stay core-owned.
- Reads emit no governance audit events on either path (the in-tree
  read handlers gate on permission features but never append), so there
  is no audit write to keep atomic; idempotency is the GET repeat check
  in the conformance matrix.

## Per-family record

Scope-honesty label (all families): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| APIRouter | `GET /health` | public probe (landed, unchanged) | none (no credential) |
| APIRouter | `GET /api/v0/openapi.json` | RequireAuth-gated (landed gate kept; subset spec) | key gate only |
| APIRouter | `GET /api/v0/docs`, `GET /api/v0/redoc` | wave-public carve-out, compiled shells, no core fetch | none (core-public parity) |
| Capabilities | `GET /api/v0/capabilities` | wave-gated terminate / relay-explicit | shared-key allow |
| CLISupport | 5 POSTs (`answer-quality/score`, `packet/render`, `bundle/capture`, `bundle/validate`, `service/report/compose`) | wave-gated terminate / relay-explicit | shared-key allow |
| Ask | `POST /api/v0/ask` | wave-gated terminate / relay-explicit | shared-key allow |
| ServiceIntelHTTP | `GET /api/v0/services/{service_name}/intelligence-report` | wave-gated terminate / relay-explicit | shared-key allow |
| AuthProviders | `GET /api/v0/auth/providers` | native-public synthesis via SignInPolicyReadAdmission v0 port, exact path only (twins stay relay) — see `contracts/signinpolicy-termination.md` | none (public, no gate to judge) |
| SignInPolicyReads | `GET /api/v0/auth/sign-in-policy` | native-public synthesis via SignInPolicyReadAdmission v0 port, exact path only (twins stay relay) — see `contracts/signinpolicy-termination.md` | none (public, no gate to judge) |
| SignInPolicyReads | `GET /api/v0/auth/admin/sign-in-policy` | wave-gated terminate / relay-explicit | shared-key allow |
| AdminProviderConfigReads | 3 GETs (list, detail, revisions) | wave-gated terminate / relay-explicit | shared-key allow (D0-proven pattern) |

Known divergences (disclosed, not hidden):

- `/health` answers the dest probe shape (versions + origin), not the
  core `{"status":"ok"}` — dest needs its own contract versions.
- `openapi.json` serves the landed subset behind the key gate; core
  serves the full spec publicly. Subset parity (every served entry
  byte-identical) rides `check-parity.sh`; full-spec parity waits for
  lane completion.
- Static shells answer their trailing-slash spellings dest-side while
  core 404s them (never-split registration, C-c1-blessed pattern).
- Unconfigured origin answers the permanent local 503 on data routes
  where core nil-safe handlers would answer zero-defaults — the
  standing fail-closed rule, uniform across native lanes.

## Spec posture

The served `openapi.json` stays behind the key gate while carrying
the permanently-public login-picker entry (`GET /api/v0/auth/providers`)
with core's truthful `"security": []` marker intact (stripping it
would make the served spec lie, and the subset gate keeps the
line-verbatim property). The setup fragment stays core-side entirely
per eshu-api#44 — no setup path keys, no other security markers in the
served document. The next lane that ports another public-marked
fragment revisits the posture test again instead of landing silently.

## Rollback

Single-commit revert: remove the wave registrations in `api/router.go`
and the lane reverts to the generic byte-relay with zero state change
(core untouched, no durable dest state, no data migration). Verify by
re-running the lane conformance suite against the reverted tree: the
ahead-of-relay assertions go red (routes relay again) while every other
suite stays green. No Plan A rehearsal is consumed by this rollback —
it restores a proxy path, not durable state.
