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

## Supplemental pin: IaCInventorySearchAdmission v0

The resources candidate page (`GET /api/v0/iac/resources`) clears on
its own supplemental pin, `IaCInventorySearchAdmission v0`
(eshu-hq/eshu#10578, merge `d9deb65433`), recorded byte-identical in
`contracts/upstream/iac_search_admission.go`,
`contracts/upstream/iac_search_admission_contract_test.go`, and
`contracts/upstream/iac_search_admission_external_test.go`. The pin
adopts exactly one read — `SearchActive` over the exported
`IaCInventorySearch` shape plus the three-const kind seam (unknown
kinds rejected at the seam), witnessed by the Postgres-composing
witness — and `IaCReadAdmission v0` stays untouched beside it. The
version line, the single-method interface, the `InventoryStore`
mirror assertion, and the surrounding contract comment (narrow by
construction, unwired, rollback) are vendored verbatim in
`contracts/upstream/iac-search-wire-shapes.txt`. The frozen method set
is `frozenIaCInventorySearchAdmission` in
`contracts/upstream/iac_search_admission_contract_test.go`
(1 method: `SearchActive`).

Explicitly OUT of the supplemental pin: grant checks and fixed failure
messages are handler behavior, not port shape; every other
`InventoryStore` method stays unadopted; no dest authorizer rides on
the forwarded call (dest carries no scoped/session resolver, so a
dest-side gate would deny scoped callers the core allows — core
re-judges every forwarded request as backstop).

## Relay posture

W3 IaC-6 (Refs eshu-hq/eshu-api#102) terminates the four pin-sufficient
POST legs ahead of the relay on native write-through routes
(`api/iac.go`, registered in `api/router.go`; family label `iac` in
`api/family.go`): `POST /api/v0/iac/unmanaged-resources`, `POST
/api/v0/iac/management-status`, `POST
/api/v0/iac/management-status/explain`, and `POST
/api/v0/iac/terraform-import-plan/candidates` — plus the resources
candidate page `GET /api/v0/iac/resources` on its own supplemental pin
(`iacResourcesRoutes` in `api/iac.go`, same write-through handler).
Termination proof: byte-parity (`api/iac_dispatch_test.go` and
`api/iac_resources_test.go`, each including the native-vs-relay
equivalence that doubles as the revert-to-relay proof),
resilience (`api/iac_resilience_test.go` plus the resources
cancel/parallel probes in `api/iac_resources_test.go`), and the
latency gate (`api/iac_latency_test.go` with
`scripts/measure-iac-latency.sh`, resources leg gated beside the POST
legs). POST /api/v0/iac/dead stays on the generic relay until the
dual-pin dead disposition lands; the remaining IaC 5 ride W8a on the
same pin. No traffic flip, no deploy, no in-tree removal in this lane:
the arbiter ruling precedes satellite consumption per the Compare
gap-04 standing.

## Enforcement

`TestIaCReadAdmissionContractIsVersioned` pins the version and the
frozen method set. A renamed, added, removed, or re-signed method
fails the frozen test until the list is deliberately updated; bump
`IaCReadAdmissionContractVersion` alongside (manually — the bump is
convention, the frozen list is the enforcement), and this repository
records the new pin before consuming it.
