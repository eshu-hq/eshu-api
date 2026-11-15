# Impact termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates the 11 W5 impact proceed legs natively ahead of the generic
relay and holds the two code-surface adapter legs on explicit relay.

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

## Terminated (11)

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

The two trace legs' TraceContext enrichment rides the Entity pin by
explicit dual-pin cite (recorded in both pins); this lane forwards the
enrichment bytes opaquely like every other byte. The wave gate
(api/impact_wave.go, served by api/impact_terminate.go) answers every
credential shape per the entity-lane rule; the family authorizer
(authorizeImpactSharedKey) allows only the shared-key (AllScopes)
identity; the only dest-synthesized errors are the gate 401, the
future-resolver family 403, and the permanent local 503 (never
Retry-After).

## Held on relay (2)

| Route | Standing | Owed |
|---|---|---|
| `POST /api/v0/impact/change-surface` | explicit relay stack (never the catch-all), same key-gate posture as the pre-lane generic relay | TODO-eshu-9669-w5: terminate natively once core provides a satellite-consumable seam for `ChangeSurfaceCodeBackend.FetchCodeSurface` |
| `POST /api/v0/impact/change-surface/investigate` | explicit relay stack (never the catch-all) | same TODO as above |

The holds keep their standing because the code-surface legs run
through the core logic adapter seam (FetchCodeSurface), not through
reads the pin could freeze: the topic search the code-surface leg
exercises IS frozen in the pin (InvestigateCodeTopic), but the seam
itself is core logic. Pinning: TestImpactWaveFrozen (relay_hold rows),
TestImpactWaveAheadOfRelay (relay lane filed explicitly), and
TestImpactAdapterHoldsStayRelayed (shared-key relay byte-identical;
foreign classes keep the key-gate 401; nil-core 503 on the relay lane).

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
lane's files): the 11 terminated POSTs return to the generic relay
behind RequireAuth and the holds keep their explicit relay. No data
migrates and no other family moves.
