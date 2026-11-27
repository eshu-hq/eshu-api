# D4 Native-next reads termination contract

Versioned read-termination record for the D4 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This lane terminates
the 45 Native-next read routes natively against the D0-proven
terminated transport: dest owns the route (caller gate, family
authorizer, response mapping) and calls the versioned core read API as
its port transport. Dest holds no table or queue handle; authz stays
atomic core-side. The repositories story GET terminated natively in
the W5 story lane (`storyWave`, superseding the D4 relay leg) and is
a terminated route, not a relay leg (see the relay-leg disclosure
below).

Scope source: `go/cmd/api/CUTOVER_OWNERSHIP.md` at the D2 lane base
(17feac86e0): the CICD, Documentation, Infra, PackageRegistry,
Repositories, Evidence, InvestigationWorkflows, and Playbooks read rows
(Compare deferred past this lane — it terminated in its gap-04 lane,
see `contracts/compare-termination.md` — so no Compare route
terminates here). Excluded
and staying relay (disclosed, not silent):

- the supply-chain, drift, and services packets sharing the
  investigations file: only the deployable-unit entry ports this lane.
- the mux-health entry sharing the repository routes file: dest serves
  `/health` from its own probe with a dest-owned shape.

(The repositories story GET was excluded here until this record's
relay-leg update: its spec entry interpolates the shared
`schema.EvidenceBoundaries` helper, which has no line-verbatim port
without a core import (forbidden), so the route reverted to the
generic relay at the time; it has since terminated natively in the W5
story lane — see the relay-leg disclosure below.)

Transport pin: `CoreReadContractsVersion v0`
(`contracts/upstream/api_boundary.go` at the provenance pin) covers
every D4 family — all 45 rows are `read`-channel rows on reader ports
only (`db.ReadStore`, graph `GraphQuery`, `ContentStore`) per the
ownership map, so no per-family admission alias is minted or needed
here (D1 minted family pins for the admin reads only). The minting
rule forbids inventing one; the pin-gate record for this lane is
"v0 present core-side and recorded satellite-side, no family alias
required, none invented".

## Status

Termination claims stay provisional until the arbiter re-reviews the
D0 evidence (blocker B1). What this lane proves per the D-wave ruling
is the required proof list: explicit routes ahead of relay,
byte-identical fragments under `scripts/check-parity.sh`, authorizer +
deny-shape tests with scope-honesty labels, byte-parity
dispatch/conformance tests vs the relayed reference, the no-core-import
check, extended `viaNative`/`viaRelay` telemetry, the satellite
latency gate with before/after numbers, and this contract section with
rollback.

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
  reads), an envelope-negotiated caller still receives the flat shape
  with truth inline (`result_limits`, pagination, group fields);
  where core negotiates (its `WriteSuccess` routes), the envelope
  flows through untouched. Dest invents no truth levels (it holds no
  read model); dest-synthesized errors stay envelope-aware via
  `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation and
  emits no audit: request/response semantics stay core-owned.
- Reads emit no governance audit events on either path: the in-tree
  query handlers gate on permission features but never append, so there
  is no audit write to keep atomic; idempotency is the GET repeat check
  plus the POST query re-read in the conformance matrix.

## Per-family record

Scope-honesty label (all families): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| CICD | 3 GETs (`ci-cd/run-correlations`, `count`, `inventory`) | wave-gated terminate / relay-explicit | shared-key allow |
| Documentation | 6 GETs (`documentation/facts`, `findings`, `findings/count`, `findings/inventory`, `findings/{finding_id}/evidence-packet`, `evidence-packets/{packet_id}/freshness`) | wave-gated terminate / relay-explicit | shared-key allow |
| Infra | 9 routes (`cloud/resources`, `ecosystem/overview`, `infra/resources/count`, `infra/resources/inventory` GETs; `ecosystem/graph-summary`, `infra/relationships`, `infra/resources/search`, `relationships/catalog`, `relationships/edges` POSTs) | wave-gated terminate / relay-explicit | shared-key allow |
| PackageRegistry | 7 GETs (`package-registry/correlations`, `dependencies`, `dependency-chains`, `packages`, `packages/count`, `packages/inventory`, `versions`) | wave-gated terminate / relay-explicit | shared-key allow |
| Repositories | 11 GETs (`catalog`, `repositories`, `by-language`, `language-inventory`, `{repo_id}/branches`, `content`, `context`, `coverage`, `freshness`, `stats`, `tree`) | wave-gated terminate / relay-explicit | shared-key allow |
| Evidence | 5 routes (`evidence/admission-decisions`, `bundle`, `evidence/relationships/{resolved_id}`, `investigations/deployable-unit/packet` GETs; `evidence/citations` POST) | wave-gated terminate / relay-explicit | shared-key allow |
| InvestigationWorkflows | 2 routes (`investigation-workflows` GET + `resolve` POST) | wave-gated terminate / relay-explicit | shared-key allow |
| Playbooks | 2 routes (`query-playbooks` GET + `resolve` POST) | wave-gated terminate / relay-explicit | shared-key allow |

Cookie rule (D2 follow-up lesson, shared with the D3 gate): only
session-cookie-shaped credentials relay — a Cookie member named
`eshu_session` (or `__Host-eshu_session`, the two names the core
caller-credential reader accepts) relays explicitly; any other Cookie
content is junk, not a browser session, and stays dest-401 with zero
core contact.

Known divergences (disclosed, not hidden):

- The served `openapi.json` omits the repositories story key: the
  story GET terminated natively in the W5 story lane, but its entry
  interpolates the shared `schema.EvidenceBoundaries` helper and has
  no line-verbatim port without a core import, so its fragment never
  lands (see `eshu-hq/eshu-api#74`).
- The served admission-decisions entry omits the core 501
  `UnsupportedCapability` response: that `$ref` target has no
  definition in the pinned core tree, so porting the line would dangle
  the served document (`TestSpec_LocalRefsResolve` fails closed).
- The four slash-skip literals (`documentation/findings/count`,
  `documentation/findings/inventory`, `repositories/by-language`,
  `repositories/language-inventory`) skip the duplicate slash
  registration that would panic Go 1.22+ ServeMux against the wildcard
  sibling; their slash spellings still terminate natively under the
  parent list-subtree registration.
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.
- Relay-leg disclosure: the D4 terminated GET is
  `/api/v0/repositories` and the terminated POST is
  `/api/v0/infra/resources/search`; the relay legs the suite rides
  are `/api/v0/dependencies` (GET) and
  `/api/v0/impact/change-surface` (POST) — the POST is the route the
  suite moved to when the W6 scan lane terminated bundles natively.
  Both legs have since terminated natively themselves (dependencies
  in W8b-narrow, change-surface in the impact lane's Wave-3), so the
  comparison now runs terminated-vs-terminated. The story GET
  `/api/v0/repositories/{repo_id}/story` likewise terminated natively
  in the W5 story lane (`storyWave`, pinned by
  `TestStoryWaveAheadOfRelay`); `d4RelayLegs` in `api/d4_wave.go` is
  retired empty and `TestD4StoryRelayLegOnGenericProxy` is retired
  with it. The D3 relay legs ride the same now-native routes
  (`BenchmarkD3RelayRead`), so every lane gates
  terminated-vs-terminated until a suite re-points at a genuinely
  relayed leg.
  Foreign credential classes (scoped/OIDC bearer, session/junk cookie)
  on wave-mounted paths relay explicitly through the wave gate (dest
  401 or explicit relay, zero native contact for unresolvable
  identities) — the standing wave-gate posture, not a D4 relay leg.

## Spec posture

No D4 fragment carries a `"security": []` public-route marker: the
only served marker stays the permanently-public login-picker entry
(`GET /api/v0/auth/providers`), and the posture test matches the
marker shape (`"security":[]` re-marshaled) rather than the bare word
— the infra fragment ports a `resource_kind` enum value named
"security" that is not a posture marker. The next lane that ports
another public-marked fragment revisits that test again instead of
landing silently.

## Rollback

Single-commit revert: remove the D4 wave registrations in
`api/router.go` and the lane reverts to the generic byte-relay with
zero state change (core untouched, no durable dest state, no data
migration). The served spec drops the D4 keys with the fragments; the
relay legs keep serving them from core. Verify by re-running the lane
conformance suite against the reverted tree: the ahead-of-relay
assertions go red (routes relay again) while every other suite stays
green. No Plan A rehearsal is consumed by this rollback — it restores
a proxy path, not durable state.
