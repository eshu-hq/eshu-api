# SupplyChain read admission contract

Versioned admission-contract port for the SupplyChain family reads
the supply-chain endpoints admit through the family-owned reader
ports (Refs `eshu-hq/eshu#9669`, D6). Canonical reads persist through
explicit core-owned admission contracts: `eshu-api` keeps the HTTP
shape and calls this versioned port, which the core-owned Postgres
stores satisfy. `eshu-api` never takes a direct table handle.

## Version

`SupplyChainReadAdmissionContractVersion = "v0"`, with the port
defined as:

```go
type SupplyChainReadAdmission interface {
	SBOMAttestationAttachmentStore
	SBOMAttestationAttachmentAggregateStore
	ContainerImageIdentityStore
	ContainerImageIdentityAggregateStore
	SecurityAlertReconciliationStore
	SecurityAlertReconciliationAggregateStore
	SecurityAlertCollectionStateStore
	CloudResourceCurrentInventoryFilter
	CloudResourceRuntimeDigestResolver
	KubernetesWorkloadCurrentInventoryFilter
	impact.FindingStore
	impact.AggregateStore
	impact.GroupedInventoryStore
	impact.ExplanationStore
	impact.ReadinessStore
	advisory.EvidenceStore
	advisory.CatalogStore
}
```

The version line, the embedded port list, and the surrounding
contract comment are vendored verbatim in
`contracts/upstream/supply-chain-read-wire-shapes.txt`. The frozen
method set is `frozenSupplyChainReadAdmission` in
`contracts/upstream/supply_read_admission_contract_test.go`
(23 methods across the 17 embedded ports).

## Read surface

The seam is read-only on purpose: every
embedded port carries bounded list/count/inventory reads and no
mutation surface, so a dest consumer implementing only this contract
can serve supply-chain reads but cannot acquire a write it did not
bargain for through it.

## Native termination (D6)

The 17 SupplyChain read GETs (advisory catalog and evidence,
container-image identities plus aggregates, impact findings plus
aggregates, impact explain, SBOM-attestation attachments plus
aggregates, security-alert reconciliations plus aggregates, the
vulnerability detail, and the static vulnerability-scanner contract)
terminate natively since the D6 lane (`api/d6_wave.go`, family
`supply_chain`): explicit gated routes ahead of the relay, the
shared-key family authorizer, byte-parity dispatch/conformance proof
vs the relayed reference — including suppression-state filtering
cases (`suppression_state`, `include_suppressed`,
`provider_state`, `reconciliation_status`) per the Decision 2 proof
obligation — and the byte-identical spec fragments
(`openapi/paths/supply/chain/*.go`, gated by
`scripts/check-parity.sh`). See
`contracts/d6-status-supplychain.md` for the termination record and
rollback. The suppressions mutation row stays out: it is the
Never-satellite survivor via SuppressionAdmission (already pinned),
not a read port.

## Enforcement

`TestSupplyChainReadAdmissionContractIsVersioned` pins the version
and the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `SupplyChainReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and this
repository records the new pin before consuming it.
