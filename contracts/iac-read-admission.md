# IaC read admission contract

Versioned admission-contract port for the IaC reads (`GET
/api/v0/iac/resources`, `GET /api/v0/replatforming/selectors`, and the
IaC POST family) (Refs `eshu-hq/eshu#9669`, O6 pin 3, AM-D4-IAC).
Canonical reads persist through explicit core-owned admission
contracts: `eshu-api` keeps the HTTP shape and calls this versioned
port, which the core-owned graph, inventory, management, selector,
and reachability stores satisfy jointly, composed by the consumer.
`eshu-api` never takes a direct store handle.

This is the deliberate widened pin: the widened shape carries the two
POST-only stores alongside the GET legs. The pin re-declares the eight
mirrored signatures in this IaC-scoped shape instead of embedding or
aliasing `querycontract.GraphQuery`, `InventoryStore`,
`ManagementStore`, `ReplatformingSelectorStore`, or
`ReachabilityStore`, so no other family can cite it as coverage.

## Version

`IaCReadAdmissionContractVersion = "v0"`, with the port defined as:

```go
type IaCReadAdmission interface {
	Run(ctx context.Context, cypher string, params map[string]any) ([]map[string]any, error)
	Summary(ctx context.Context, access querycontract.RepositoryAccessFilter, limit int) (InventorySummary, error)
	ListReplatformingSelectors(ctx context.Context, limit int, allowedScopeIDs []string) (ReplatformingSelectorPage, error)
	ListUnmanagedCloudResources(ctx context.Context, filter ManagementFilter) ([]ManagementFindingRow, error)
	CountUnmanagedCloudResources(ctx context.Context, filter ManagementFilter) (int, error)
	ListLatestCleanupFindings(ctx context.Context, repoIDs []string, families []string, includeAmbiguous bool, limit int, offset int) ([]ReachabilityFindingRow, error)
	CountLatestCleanupFindings(ctx context.Context, repoIDs []string, families []string, includeAmbiguous bool) (int, error)
	HasLatestRows(ctx context.Context, repoIDs []string, families []string) (bool, error)
}
```

The version line, the interface, the one-directional compile-time
mirror assertion, and the surrounding contract comment (narrow by
construction, explicitly OUT, rollback) are vendored verbatim in
`contracts/upstream/iac-read-wire-shapes.txt`. The frozen method set
is `frozenIaCReadAdmission` in
`contracts/upstream/iac_read_admission_contract_test.go`
(8 methods: `Run`, `Summary`, `ListReplatformingSelectors`,
`ListUnmanagedCloudResources`, `CountUnmanagedCloudResources`,
`ListLatestCleanupFindings`, `CountLatestCleanupFindings`,
`HasLatestRows`).

## Read surface

The seam is read-only on purpose: every declared method is a bounded
read and the port carries no mutation surface, so a dest consumer
implementing only this contract can serve the IaC reads but cannot
acquire a write it did not bargain for through it.

## In scope

- The IaC routes admit their bounded inventory graph read through
  `Run` (mirroring `querycontract.GraphQuery`, with `RunSingle`
  deliberately excluded), their inventory facet read through
  `Summary` (mirroring `InventoryStore`), their selector inventory
  read through `ListReplatformingSelectors` (mirroring
  `ReplatformingSelectorStore`), their cloud-management finding reads
  through `ListUnmanagedCloudResources` and
  `CountUnmanagedCloudResources` (mirroring `ManagementStore`), and
  their materialized cleanup reads through
  `ListLatestCleanupFindings`, `CountLatestCleanupFindings`, and
  `HasLatestRows` (mirroring the `ReachabilityStore` surface).

Explicitly OUT of this pin: grant checks and fixed failure messages
are handler behavior, not port shape; `GraphQuery.RunSingle` is
deliberately excluded; the v0 `ContentStore` leg rides the frozen
`CoreReadContractsVersion v0`; no suppression leg exists on these
routes.

## Relay posture

The IaC legs stay on the generic relay until the termination lane
lands; no native route moves in this record. The pin is recorded here
first so the termination lane consumes a minted pin, never an
unminted one. The widened shape needs arbiter review per the Compare
gap-04 standing before satellite consumption.

## Enforcement

`TestIaCReadAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump
`IaCReadAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
