# SupplyChainImpactPacketReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W5f packet-single core pin, per the supplemental
arbiter ruling on eshu-hq/eshu-api#97. Minted core-side at `76240fda9f`
(merge of `eshu-hq/eshu#10705`); the satellite records this pin here. A
satellite PR consuming an unminted pin is invalid. The mint-diff CONFIRM
precedes satellite termination of the packet leg.

## Shape

Version: `SupplyChainImpactPacketReadAdmissionContractVersion = "v0"`
(go/internal/query/supply/chain/packet_impact_read_admission.go).

Ceiling 5 re-declared (never embedded) methods — the two reducer reads
plus the shared selector closure the D6 pin explicitly excludes — sealed:

| # | Method | Mirrors | Serves (per pin closure) |
|---|---|---|---|
| 1 | `ExplainSupplyChainImpact` | `impact.ExplanationStore` | explanation read |
| 2 | `ReadSupplyChainImpactReadiness` | `impact.ReadinessStore` | readiness read |
| 3 | `Run` | selector graph fallback | selector.ResolveExactForAccess fallback |
| 4 | `RunSingle` | selector graph fallback | selector.ResolveExactForAccess fallback |
| 5 | `MatchRepositories` | `ContentStore` selector | selector.ResolveExactForAccess |

The entries re-declare the impact.ExplanationStore,
impact.ReadinessStore, and selector content/graph signatures in this
packet-scoped shape instead of embedding the shared or family ports —
do not widen the D6 pin, do not pin GraphQuery/ContentStore wholesale,
and do not cite the D6 pin alone as packet coverage.

Enforcement (all in go/internal/query/supply/chain, plus the
query-level mirror):

- `packet_impact_read_admission_contract_test.go` — frozen method-set
  test plus version pin (exactly 5 methods), the satisfiability witness,
  the seeded re-signed rejection, and the widening-is-caught proof.
- `packet_impact_read_admission_wire_shapes_test.go` — the
  pin-satisfying canned store proves a satellite can consume the pin
  using only the pin plus the querycontract, packet, and impact shapes.
- `go/internal/query/w5_supply_chain_impact_packet_read_admission_mirror.go`
  — compile-time proof that the production stores jointly serve the pin.

## Census

In scope — the W5f packet single ONLY:

- `GET /api/v0/investigations/supply-chain/impact/packet`

The repository story single rides its own story pin and neither route
may cite the other's pin. Decision 2 proof stays OWED for this route:
the owed satellite parity corpus (active winner, suppressed winner,
expired-suppression winner, ambiguous scope, no-evidence scope,
scoped-empty-grant scope-refusal packet, unconfigured-origin 503) is
proven at leaf time, not frozen here.

## Explicitly OUT (not widened by this pin)

- `ImpactPacketResponder` (RespondSupplyChainImpact,
  RespondSupplyChainScopeRefusal) and the packet composers
  (BuildSupplyChainImpactPacket, BuildNoEvidenceExplanation,
  BuildAmbiguousExplanation, BuildExplanation, FindingReadinessScope,
  TrimExplanationFilter): the production responder is a stateless
  composer and pure core logic under the forward transport. Not a pin
  method.
- `ListRepositories`: selector.ResolveExactForAccess calls
  MatchRepositories only.
- Every other ContentStore method, the CollectorReadiness probe, the
  suppressions mutation row (tier-C SuppressionAdmission, never a read
  termination), the capability gate, and the fixed failure messages:
  not exercised by this route, or handler behavior proven at
  byte-parity time and never frozen here.

## Satellite consumption

Depend on the shape, not a constructor: serve the packet reads through
the existing explanation/readiness stores plus the selector closure,
the same backends `cmd/api` wiring builds today. Rollback: revert this
pin. The seam is unwired — no handler, route, or caller depends on it
yet — so rollback moves no traffic and changes no data.
