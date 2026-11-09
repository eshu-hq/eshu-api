# Compare read admission contract

Versioned admission-contract port for the Compare POST read (`POST
/api/v0/compare/environments`) (Refs `eshu-hq/eshu#9669`, gap 04).
Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned graph and content stores satisfy jointly,
composed by the consumer. `eshu-api` never takes a direct store
handle.

This is the deliberate later pin the D6 wave discloses: Compare
admits only through the shared graph/content ports, never through
`status.Reader`, and pinning those shared ports would be a blanket
pin the minting rule forbids permanently. The pin re-declares the
four mirrored signatures in this Compare-scoped shape instead of
embedding or aliasing the shared ports, so no other family can cite
it as coverage.

## Version

`CompareReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type CompareReadAdmission interface {
	Run(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
	RunSingle(ctx context.Context, cypher string, params map[string]any) (map[string]any, error)
	ListRepoFiles(ctx context.Context, repoID string, limit int) ([]querycontract.FileContent, error)
	GetFileContent(ctx context.Context, repoID, relativePath string) (*querycontract.FileContent, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/compare-read-wire-shapes.txt`. The frozen method
set is `frozenCompareReadAdmission` in
`contracts/upstream/compare_read_admission_contract_test.go`
(4 methods: `Run`, `RunSingle`, `ListRepoFiles`,
`GetFileContent`).

## Read surface

The seam is read-only on purpose: every declared method is a
bounded read and the port carries no mutation surface, so a dest
consumer implementing only this contract can serve the comparison
but cannot acquire a write it did not bargain for through it.

## In scope

- `POST /api/v0/compare/environments` admits its workload and
  environment-snapshot graph reads through `Run` and `RunSingle`
  (mirroring `querycontract.GraphQuery`) and its service-evidence
  content reads through `ListRepoFiles` and `GetFileContent`
  (mirroring `service.EvidenceReader` as exercised through
  `service.LoadServiceQueryEvidence`).

Explicitly OUT of this pin: the grant check
(`RepositoryAccessFilterFromContext` plus the repo grant comparison)
and the fixed failure messages are handler behavior, not port
shape; no suppression leg exists on this route, so the
suppressions mutation row stays the Decision 2 tier-C survivor via
`SuppressionAdmission` and the Decision 2
suppression-state-filtering parity obligation stays on the
SupplyChain finding/reconciliation legs this pin does not touch.

## Relay posture

The Compare POST terminated in its gap-04 lane (see
`contracts/compare-termination.md`): shared-key callers terminate on
the native lane ahead of the generic proxy with byte parity to the
relay, while scoped-token, OIDC-bearer, and browser-session classes
relay explicitly until D8-grade parity. The pin was recorded here
first so the termination lane consumed a minted pin, never an
unminted one.

## Enforcement

`TestCompareReadAdmissionContractIsVersioned` pins the version and
the frozen method set. A renamed, added, removed, or re-signed
method fails the frozen test until the list is deliberately updated;
bump `CompareReadAdmissionContractVersion` alongside (manually —
the bump is convention, the frozen list is the enforcement), and
this repository records the new pin before consuming it.
