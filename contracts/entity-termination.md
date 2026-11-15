# Entity termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates the 7 W5 entity proceed legs natively ahead of the generic
relay and holds the context fallback leg on explicit relay; the 9th
admitted route (`GET /api/v0/graph/entities`) terminates in the merged
W8b-narrow lane and is deferred here.

## Pin

`EntityReadAdmission v0` (extended Mint A, 27 re-declared methods),
recorded satellite-side in contracts/entity-read-admission.md plus the
vendored upstream copies (contracts/upstream/entity_read_admission.go,
contracts/upstream/entity_read_admission_contract_test.go, byte-identical
at core `76240fda9f`) and the wire-shapes excerpt
(contracts/upstream/entity-read-wire-shapes.txt). Enforcement:
`scripts/check-parity.sh` (`SRC_SHA_W5ENTITYIMPACT` block) plus the
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

## Deferred to the merged W8b-narrow lane (1)

`GET /api/v0/graph/entities` terminates natively in merged
eshu-hq/eshu-api#143 (`contracts/graph-entity-inventory-read-admission.md`);
this lane records no second registration for it (a duplicate
`ServeMux` pattern would panic at startup).

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

## Held on relay (1)

| Route | Standing | Owed |
|---|---|---|
| `GET /api/v0/entities/{entity_id}/context` | explicit relay stack (never the catch-all), same key-gate posture as the pre-lane generic relay | TODO-eshu-9669-w5: terminate natively once core provides a satellite-consumable satisfier for `ContentRelationshipBuilder.BuildContentRelationships` |

The hold keeps its standing because the only production satisfier for
the content-fallback builder is root's ContentIndexRelationshipBuilder
— a stateless adapter over root's unexported per-language extraction
closure with no satellite-consumable constructor — while the pin
freezes only its two exercised reads. Pinning: TestEntityWaveFrozen
(relay_hold row), TestEntityWaveAheadOfRelay (relay lane filed
explicitly), TestEntityContextHoldStaysRelayed (shared-key relay,
byte-identical), TestEntityScopedHoldKeepsKeyGate (foreign classes
keep the key-gate 401), and the nil-core 503 leg in
TestEntityTerminatedAs_NilCore503.

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
  BenchmarkEntityRelayRead, bounded by scripts/measure-w5-latency.sh
  (1.50 budget) and wired into .github/workflows/read-latency.yml;
  numbers in docs/w5-latency.md.
- Spec: the 8 routes mount explicitly and stay disclosed-absent until
  the follow-up spec-port lane (api/spec_scope_test.go,
  openapi/spec_w5_test.go); the graph_entities row stays disclosed-absent
  under the W8b-narrow lane (see the wave comment).

## Scope honesty

Handler behavior stays satellite-side and unpinned: grant checks, the
repository selector, fixed failure messages, QueryProfile. No
suppression leg exists on these routes; the Decision 2 parity
obligation stays on the supply-chain legs this pin does not touch. No
serialization, no worker or batch reduction anywhere in this lane.

## Rollback

Remove the `s.registerEntityWave(mux)` call in routes() (or this
lane's files): the 8 terminated routes return to the generic relay
behind RequireAuth and the hold keeps its explicit relay. No data
migrates and no other family moves.
