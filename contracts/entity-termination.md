# Entity termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates all 9 W5 entity legs natively ahead of the generic
relay. Wave-2 terminated the last 2: the context fallback leg (owed
satisfier landed core-side as `entity/content/fallback.Builder`,
#10745 + #10779) and the investigations service leg (owed pin
extension landed in the 27->30 pin, #10747).

## Pin

`EntityReadAdmission v0` (extended Mint A, 30 re-declared methods),
recorded satellite-side in contracts/entity-read-admission.md plus the
vendored upstream copies (contracts/upstream/entity_read_admission.go,
contracts/upstream/entity_read_admission_contract_test.go, byte-identical
at core `9bf29e53`) and the wire-shapes excerpt
(contracts/upstream/entity-read-wire-shapes.txt, rebuilt from the new
pin). Enforcement:
`scripts/check-parity.sh` (`SRC_SHA_W5ENTITY30` block) plus the
core-side frozen method-set test, satisfiability witness, seeded
re-signed rejection, and production-store mirror.

## Terminated (7)

| Route | Family | Gate |
|---|---|---|
| `POST /api/v0/entities/resolve` | entities | entityReadGate |
| `GET /api/v0/workloads/{workload_id}/context` | workloads | entityReadGate |
| `GET /api/v0/workloads/{workload_id}/story` | workloads | entityReadGate |
| `GET /api/v0/services/{service_name}/context` | service_intel | entityReadGate |
| `GET /api/v0/services/{service_name}/story` | service_intel | entityReadGate |
| `GET /api/v0/services/{service_name}/api-surface/endpoints` | service_intel | entityReadGate |
| `GET /api/v0/services/{service_name}/downstream-consumers` | service_intel | entityReadGate |

The wave gate (api/entity_wave.go, served by api/entity_terminate.go)
answers every credential shape: a valid shared key terminates on the
native lane; a foreign credential class (scoped token, OIDC bearer,
session cookie) relays explicitly on the relay lane so core judges it;
credential-less callers — and junk-cookie carriers — get the standing
401 with zero core contact. The family authorizer
(authorizeEntitySharedKey) allows only the shared-key (AllScopes)
identity dest can construct; core re-judges every forwarded call as
backstop. The only dest-synthesized errors on the terminated path are
the gate 401, the future-resolver family 403
("all-scopes operator authorization is required"), and the permanent
local 503 when no core origin is configured (never Retry-After).

## Terminated in Wave-2 (was: held on relay)

| Route | Standing | Owed item (met) |
|---|---|---|
| `GET /api/v0/entities/{entity_id}/context` | native (shared-key gate, forward transport) | satisfier `entity/content/fallback.Builder` merged core-side (#10745 + #10779); the pin freezes its two exercised reads |
| `GET /api/v0/investigations/services/{service_name}` | native (shared-key gate, forward transport) | pin extension 27->30 merged core-side (#10747): `ListRepoEntitiesByTypes`, `RepositoryReadModelSummary`, `SearchFileReferenceAnyRepo` |

Pinning: TestEntityWaveFrozen (all 9 rows gated),
TestEntityWaveAheadOfRelay (native lane filed explicitly),
TestEntityContextTerminatedNatively and
TestEntityInvestigationsTerminatedNatively (shared-key native,
byte-identical), TestEntityContextForeignBearerRelaysExplicitly
(foreign classes relay explicitly, never the catch-all), and the
nil-core 503 leg in TestEntityTerminatedAs_NilCore503.

## Dual-pin cite

The W5c-e impact deployment-trace enrichment legs
(trace-deployment-chain, deployment-config-influence) require this pin
alongside ImpactReadAdmission for their TraceContext leg, which pulls
the F-A service-story closure through the root adapter. The cite is
recorded in both pins; silent reliance is forbidden, and no other
route may cite this pin as coverage.

## Proofs

- Frozen wave: TestEntityWaveFrozen (entity_conformance_test.go).
- Ahead of relay: TestEntityWaveAheadOfRelay.
- Byte parity: TestEntityByteParity (method/path/query/body/Accept/
  credential/correlation forwarded; core bytes untouched; both
  negotiated shapes), TestEntityScopedEmptyGrantRelay (scoped
  bearer outside the grant relays explicitly; core 404 refusal
  untouched; relay lane filed).
- Authorizer + deny shape: TestEntityDenyBranchShapePinned (documented
  family 403, both shapes, zero core contact), relay-explicit and
  junk-cookie suites, empty-key fail-closed, nil-core 503.
- Latency: BenchmarkEntityTerminatedRead vs
  BenchmarkImpactRelayRead (explicit-relay reference; the entity relay
  bench retired with the last hold row), bounded by
  scripts/measure-w5-latency.sh (1.50 budget) and wired into
  .github/workflows/read-latency.yml; numbers in docs/w5-latency.md.
- Spec: the 9 routes mount explicitly (all 9 native) and
  stay disclosed-absent until the follow-up spec-port lane
  (api/spec_scope_test.go, openapi/spec_w5_test.go).

## Scope honesty

Handler behavior stays satellite-side and unpinned: grant checks, the
repository selector, fixed failure messages, QueryProfile. No
suppression leg exists on these routes; the Decision 2 parity
obligation stays on the supply-chain legs this pin does not touch. No
serialization, no worker or batch reduction anywhere in this lane.

## Rollback

Remove the `s.registerEntityWave(mux)` call in routes() (or this
lane's files): all 9 routes return to the generic relay
behind RequireAuth. No data migrates and no other family moves.
