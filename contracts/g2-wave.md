# G2 reads satellite termination

Satellite termination of the 7 G2 reads (Refs `eshu-hq/eshu#9669`,
satellite `eshu-hq/eshu-api#101` RE:G2V2). This lane terminates the 7
CUTOVER G2 rows natively against the D0-proven terminated
transport: dest owns the route (caller gate, family authorizer,
response mapping) and calls the versioned core read API as its port
transport. Dest holds no table or queue handle; authz stays
atomic core-side.

Scope source: the 5 core read-admission v0 pins (core
`eshu-hq/eshu#10796`, merged `64de9fc7f1`):

- `ExtractionReadinessReadAdmission v0` covers the 2
  collector-extraction-readiness GETs (`/api/v0/collector-extraction-readiness`,
  `/api/v0/collector-extraction-readiness/{family}`).
- `FactSchemaReadAdmission v0` covers the 2 fact-schema-versions
  GETs (`/api/v0/fact-schema-versions`,
  `/api/v0/fact-schema-versions/{fact_kind}`).
- `SurfaceInventoryReadAdmission v0` covers the surface-inventory
  GET (`/api/v0/surface-inventory`).
- `VisualizationReadAdmission v0` covers the visualizations derive
  POST (`/api/v0/visualizations/derive`).
- `ManifestDependenciesReadAdmission v0` covers the
  manifest-dependencies GET
  (`/api/v0/repository/manifest-dependencies`).

The minting rule forbids inventing pins beyond these 5: every wave
row admits through its re-declared port method over the guarded read
port, this lane cites no other pin, and no other family may cite
these pins as coverage.

## Per-route record

| Route | CUTOVER family | Pin | Core fragment |
| --- | --- | --- | --- |
| `GET /api/v0/collector-extraction-readiness` | ExtractionReadiness | `ExtractionReadinessReadAdmission v0` | `paths/status/collector_extraction_readiness.go` |
| `GET /api/v0/collector-extraction-readiness/{family}` | ExtractionReadiness | `ExtractionReadinessReadAdmission v0` | `paths/status/collector_extraction_readiness.go` |
| `GET /api/v0/fact-schema-versions` | FactSchemaVersions | `FactSchemaReadAdmission v0` | `paths/status/fact_schema_version.go` |
| `GET /api/v0/fact-schema-versions/{fact_kind}` | FactSchemaVersions | `FactSchemaReadAdmission v0` | `paths/status/fact_schema_version.go` |
| `GET /api/v0/surface-inventory` | SurfaceInventory | `SurfaceInventoryReadAdmission v0` | `paths/catalog/surface_inventory.go` |
| `POST /api/v0/visualizations/derive` | Visualization | `VisualizationReadAdmission v0` | `paths/evidence/visualization_packets.go` |
| `GET /api/v0/repository/manifest-dependencies` | ManifestDependencies | `ManifestDependenciesReadAdmission v0` | `paths/repository/manifest_dependencies.go` |

What this lane proves per the D-wave ruling is the required proof
list: explicit routes ahead of relay, byte-identical fragments under
`scripts/check-parity.sh`, authorizer + deny-shape tests with the
scope-honesty label, byte-parity dispatch/conformance tests vs the
relayed reference, the no-core-import check, extended
`viaNative`/`viaRelay` telemetry, the satellite latency evidence
(terminated-vs-proxy-relay ratios, no same-shape before/after: the
baseline is the still-relaying proxy route), and this contract
section with rollback.

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
  re-judges every forwarded call as backstop (tenant scope,
  permission catalog, governance mode) — which is also where the
  live 403s and mode-dependent denials come from. Dest adds no
  handler validation and emits no audit: request/response semantics
  stay core-owned. Scoped tokens receive only granted repositories
  and scopes per the pin fragments; an ungranted selector is core's
  not-found, and an all-scope bearer with no grant is core's 403
  under hosted modes — the dest gate relays those classes
  explicitly so core judges them.
- Reads emit no governance audit events on either path: the in-tree
  G2 handlers gate on permission features but never append, so there
  is no audit write to keep atomic; idempotency is the GET
  repeat check in the conformance matrix (the derive POST proves
  byte-parity per body, not idempotency).

## Per-family record

Scope-honesty label (G2 legs): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers,
and browser sessions relay explicitly — core judges, core bytes come
back untouched, dest invents no 401/403/404/500 for them — until the
D8 session resolver lands. The deny branch (`allScopes=false`) is
pinned for that day and unreachable on live routes.

Every leg classifies to the shared `other` family in
classifyFamily (no per-family prefix entries exist for these paths):
the authorizer and telemetry pin that value until family entries
land, exactly like the Wave-1b component-extension legs. The CUTOVER
families (ExtractionReadiness, FactSchemaVersions,
SurfaceInventory, Visualization, ManifestDependencies) ride the wave
table row comments and the per-route record above, not the satellite
serve family.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| other (G2 legs) | 6 GETs + 1 POST (readiness x2, fact-schema x2, surface-inventory, visualizations/derive, manifest-dependencies) | wave-gated terminate / relay-explicit | shared-key allow |

Cookie rule (D2 follow-up lesson, shared with the Wave-1b lane):
only session-cookie-shaped credentials relay — a Cookie member named
`eshu_session` (or `__Host-eshu_session`, the two names the core
caller-credential reader accepts) relays explicitly; any other Cookie
content is junk, not a browser session, and stays dest-401 with zero
core contact.

Known divergences (disclosed, not hidden):

- No G2 route stays relay: all 7 pinned rows terminate. The rows
  this lane supersedes were disclosed relay-only before the mint:
  the 6 no-port rows (readiness x2, fact-schema x2,
  surface-inventory, visualizations/derive) in
  `contracts/w8b-narrow-termination.md` and `api/compext_wave.go`,
  plus the manifest-dependencies row with no prior lane record.
- The readiness paths in this lane
  (`collector-extraction-readiness`, `collector-extraction-readiness/{family}`)
  are not the similarly named earlier-lane paths
  (`collector-readiness`, `status/collector-readiness`, `collectors`
  family reads): those keep their own lanes and families and never
  enter this wave. The new path prefix does not hijack them (the
  wave table pins full paths; `classifyFamily` still maps each to
  its owning family).
- Non-registered spellings of the 7 paths are not registered: a
  keyless POST to a GET leg keeps the standing 401 at the key gate;
  a keyed wrong-method call crosses the generic relay and core
  judges the method (core allowlists each route to its pinned
  method).
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.

## Spec posture

The 7 G2 fragments stay core-side for this lane: each entry lives
in its owning core paths file (see `api/g2_wave.go` Fragment
column), and porting the entries without their referenced blocks
would break the verbatim-port rule `scripts/check-parity.sh`
gates — so, like the Wave-1b lane, the routes are disclosed spec
absences (pinned by `TestG2Native_SpecKeysDisclosedAbsent`), not a
port. A follow-up spec-port lane claims the keys when it ports the
entries entry-shaped with their referenced schemas; until then no
served key duplicates a relayed route, and no mounted route
promises what the document does not carry. No fragment carries a
`"security": []` public-route marker question: all 7 are authed
reader-port reads, and the posture test
(`TestG2OpenAPIStillKeyGated`) pins the spec route stays key-gated —
the only served marker stays the permanently-public login-picker
entry (`GET /api/v0/auth/providers`).

## Rollback

Single-commit revert: remove the `s.registerG2Wave(mux)` call in
`api/router.go` (with `api/g2_wave.go`, `api/g2_terminate.go`, and
their tests) and the lane reverts to the generic byte-relay with
zero state change (core untouched, no durable dest state, no data
migration). The spec posture is unchanged (the keys stay absent
either way); the relay legs keep serving them from core. Verify by
re-running the lane conformance suite against the reverted tree:
the ahead-of-relay assertions go red (routes relay again) while
every other suite stays green. No Plan A rehearsal is consumed by
this rollback — it restores a proxy path, not durable state.
