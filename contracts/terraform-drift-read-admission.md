# Terraform-drift read admission contract

Versioned admission-contract port for the Terraform drift findings
read (`POST /api/v0/terraform/config-state-drift/findings`) (Refs
`eshu-hq/eshu#9669`, O6 pin 5). Canonical reads persist through
explicit core-owned admission contracts: `eshu-api` keeps the HTTP
shape and calls this versioned port, which the core-owned finding
store satisfies, composed by the consumer. `eshu-api` never takes a
direct store handle.

The pin re-declares the two mirrored signatures in this drift-scoped
shape instead of embedding or aliasing `FindingStore`, so no other
family can cite it as coverage.

## Version

`TerraformDriftReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type TerraformDriftReadAdmission interface {
	ListActiveFindings(ctx context.Context, filter FindingFilter) ([]FindingRow, error)
	CountActiveFindings(ctx context.Context, filter FindingFilter) (int, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/terraform-drift-read-wire-shapes.txt`. The frozen
method set is `frozenTerraformDriftReadAdmission` in
`contracts/upstream/terraform_drift_read_admission_contract_test.go`
(2 methods: `ListActiveFindings`, `CountActiveFindings`).

## Read surface

The seam is read-only on purpose: every declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve the drift findings but
cannot acquire a write it did not bargain for through it. The
`FindingFilter` double guard (when `Scoped` is true the store
intersects every row with `AllowedScopeIDs` and returns zero rows
without querying at all when `AllowedScopeIDs` is empty) passes
through the port untouched.

## In scope

- `POST /api/v0/terraform/config-state-drift/findings` admits its
  active-finding list and count reads through `ListActiveFindings`
  and `CountActiveFindings` (mirroring `FindingStore`).

Explicitly OUT of this pin: profile and capability gating are
handler behavior, not port shape; the grant check and the fixed
failure messages are handler behavior, not port shape.

## Relay posture

The config-state-drift findings POST stays on the generic relay until
the termination lane lands; no native route moves in this record. The
pin is recorded here first so the termination lane consumes a minted
pin, never an unminted one. `POST
/api/v0/cloud/runtime-drift/findings` terminates on the v0 lane via
the mcp37#2 same-handler precedent until this pin lands, then follows
it.

## Enforcement

`TestTerraformDriftReadAdmissionContractIsVersioned` pins the version
and the frozen method set, with the seeded proof
(`TestTerraformDriftReadAdmissionSeededProof`). A renamed, added,
removed, or re-signed method fails the frozen test until the list is
deliberately updated; bump
`TerraformDriftReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
