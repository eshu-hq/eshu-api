# Impact termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates all 13 W5 impact legs natively ahead of the generic
relay (Wave-3 terminated the two change-surface adapter legs: every
leg read sits inside the 14-method pin, so no new pin is owed).

## Pin

`ImpactReadAdmission v0` (14 re-declared methods), recorded
satellite-side in contracts/impact-read-admission.md plus the vendored
upstream copies (contracts/upstream/impact_read_admission.go,
contracts/upstream/impact_read_admission_contract_test.go,
byte-identical at core `76240fda9f`) and the wire-shapes excerpt
(contracts/upstream/impact-read-wire-shapes.txt). Enforcement:
`scripts/check-parity.sh` (`SRC_SHA_W5ENTITYIMPACT` block) plus the
core-side frozen method-set test, satisfiability witness, seeded
re-signed rejection, and production-store mirror.

## Terminated (13)

| Route | Family | TraceContext dual-pin |
|---|---|---|
| `POST /api/v0/impact/trace-deployment-chain` | impact | yes (Entity pin) |
| `POST /api/v0/impact/blast-radius` | impact | no |
| `POST /api/v0/impact/contracts` | impact | no |
| `POST /api/v0/impact/deployment-config-influence` | impact | yes (Entity pin) |
| `POST /api/v0/impact/trace-exposure-path` | impact | no |
| `POST /api/v0/impact/pre-change` | impact | no |
| `POST /api/v0/impact/developer-change-plan` | impact | no |
| `POST /api/v0/impact/entity-map` | impact | no |
| `POST /api/v0/impact/resource-investigation` | impact | no |
| `POST /api/v0/impact/trace-resource-to-code` | impact | no |
| `POST /api/v0/impact/explain-dependency-path` | impact | no |
| `POST /api/v0/impact/change-surface` | impact | no |
| `POST /api/v0/impact/change-surface/investigate` | impact | no |

The two trace legs' TraceContext enrichment rides the Entity pin by
explicit dual-pin cite (recorded in both pins); this lane forwards the
enrichment bytes opaquely like every other byte. The wave gate
(api/impact_wave.go, served by api/impact_terminate.go) answers every
credential shape per the entity-lane rule; the family authorizer
(authorizeImpactSharedKey) allows only the shared-key (AllScopes)
identity; the only dest-synthesized errors are the gate 401, the
future-resolver family 403, and the permanent local 503 (never
Retry-After).

## Terminated in Wave-3 (was: held on relay)

| Route | Standing | Owed item (met) |
|---|---|---|
| `POST /api/v0/impact/change-surface` | native (shared-key gate, forward transport) | none owed: target resolve + traversal run through `h.Neo4j.Run`, covered by the pin's `Run`/`RunSingle` |
| `POST /api/v0/impact/change-surface/investigate` | native (shared-key gate, forward transport) | none owed: code-surface reads run through `InvestigateCodeTopic` + `ListRepoEntitiesByPaths`, both frozen in the pin; the FetchCodeSurface seam executes core-side on the forwarded call |

The holds needed no new seam: the adapter seam (`FetchCodeSurface`) is
core-side handler wiring, and the satellite never implements core
seams — it forwards the call and core executes it. Pinning:
TestImpactWaveFrozen (all 13 rows gated),
TestImpactWaveAheadOfRelay (native lane filed explicitly), and
TestImpactAdapterTerminatedNatively (shared-key native byte-identical;
foreign classes relay explicitly; nil-core 503 on the native lane).

## Explicitly OUT

PathProbeBackend methods, every other ContentStore method, the catalog
reads, Profile, capability gates, grant-before-limit checks,
scoped-token node-by-node filtering, and fixed failure messages: not
exercised by W5 impact paths, or handler behavior proven at
byte-parity time and never frozen. No serialization anywhere in this
lane.

## Proofs

- Frozen wave: TestImpactWaveFrozen (impact_conformance_test.go).
- Ahead of relay: TestImpactWaveAheadOfRelay (dual-pin trace bodies
  included).
- Byte parity: TestImpactByteParity, TestImpactScopedEmptyGrantRelay
  (core 403 refusal untouched; relay lane filed).
- Authorizer + deny shape: TestImpactDenyBranchShapePinned, relay-
  explicit and junk-cookie suites, empty-key fail-closed, nil-core 503.
- Latency: BenchmarkImpactTerminatedRead vs
  BenchmarkImpactRelayRead, bounded by scripts/measure-w5-latency.sh;
  numbers in docs/w5-latency.md.
- Spec: the 13 POSTs mount explicitly and stay disclosed-absent until
  the follow-up spec-port lane (api/spec_scope_test.go,
  openapi/spec_w5_test.go).

## Rollback

Remove the `s.registerImpactWave(mux)` call in routes() (or this
lane's files): all 13 POSTs return to the generic relay
behind RequireAuth. No data migrates and no other family moves.
