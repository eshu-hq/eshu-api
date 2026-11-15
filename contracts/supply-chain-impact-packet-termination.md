# Supply-chain impact packet termination (W5 satellite record)

Refs eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#97. This lane
terminates the W5f packet single natively ahead of the generic relay
and closes the Decision 2 parity obligation in-lane.

## Pin

`SupplyChainImpactPacketReadAdmission v0` (5 re-declared methods),
recorded satellite-side in
contracts/supply-chain-impact-packet-read-admission.md plus the
vendored upstream copies
(contracts/upstream/supply_chain_impact_packet_read_admission.go,
contracts/upstream/supply_chain_impact_packet_read_admission_contract_test.go,
byte-identical at core `76240fda9f`) and the wire-shapes excerpt
(contracts/upstream/supply-chain-impact-packet-read-wire-shapes.txt).
Enforcement: `scripts/check-parity.sh` (`SRC_SHA_W5ENTITYIMPACT`
block) plus the core-side frozen method-set test, satisfiability
witness, seeded re-signed rejection, and production-store mirror. The
pin does not widen the D6 pin, does not pin GraphQuery/ContentStore
wholesale, and the D6 pin alone is not packet coverage.

## Terminated (1)

| Route | Family | Gate |
|---|---|---|
| `GET /api/v0/investigations/supply-chain/impact/packet` | supply_chain | packetReadGate |

The wave gate answers every credential shape per the entity/impact/
story lane rule; the family authorizer (authorizePacketSharedKey)
allows only the shared-key (AllScopes) identity; the only
dest-synthesized errors are the gate 401, the future-resolver family
403, and the permanent local 503 (never Retry-After).

## Decision 2 (closed in-lane)

The owed satellite parity corpus is proven at leaf time in
TestPacketDecision2Corpus (packet_parity_test.go), in both negotiated
shapes:

1. active winner — shared-key 200, native, byte-identical.
2. suppressed winner — shared-key 200, native, byte-identical.
3. expired-suppression winner — shared-key 200, native,
   byte-identical.
4. ambiguous scope — shared-key 200, native, byte-identical.
5. no-evidence scope — shared-key 200, native, byte-identical.
6. scoped-empty-grant scope-refusal packet — scoped bearer, explicit
   relay, core refusal bytes untouched, relay lane filed.
7. unconfigured-origin 503 — permanent local 503, never Retry-After.

No hold was needed: every case closes on the forward-transport shape
with no new satisfier. The responder
(RespondSupplyChainImpact/RespondSupplyChainScopeRefusal) and the
composers stay core-side by design — pure core logic under the forward
transport, not pin methods.

## Proofs

- Frozen wave: TestPacketWaveFrozen (packet_conformance_test.go).
- Ahead of relay: TestPacketWaveAheadOfRelay (method/path/query
  intact; native lane filed).
- Byte parity: TestPacketDecision2Corpus + TestPacketByteParity
  (query/Accept/credential/correlation forwarded intact).
- Authorizer + deny shape: TestPacketDenyBranchShapePinned, relay-
  explicit and junk-cookie suites, empty-key fail-closed, nil-core 503.
- Latency: BenchmarkPacketTerminatedRead, bounded by
  scripts/measure-w5-latency.sh; numbers in docs/w5-latency.md.
- Spec: the packet GET mounts explicitly and stays disclosed-absent
  until the follow-up spec-port lane (moved out of
  TestSpec_D4DisclosedOmissions; carried by openapi/spec_w5_test.go).

## Rollback

Remove the `s.registerPacketWave(mux)` call in routes() (or this
lane's files): the packet GET returns to the generic relay behind
RequireAuth. The spec absence stays disclosed either way. No data
migrates and no other family moves.
