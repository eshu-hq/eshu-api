# ImpactReadAdmission v0 (core pin record)

Refs eshu-hq/eshu#9669. W5c-e 14-read core pin, per the supplemental
arbiter ruling on eshu-hq/eshu-api#97. Minted core-side at `76240fda9f`
(merge of `eshu-hq/eshu#10705`); the satellite records this pin here. A
satellite PR consuming an unminted pin is invalid. The mint-diff CONFIRM
precedes satellite termination of the impact legs.

## Shape

Version: `ImpactReadAdmissionContractVersion = "v0"`
(go/internal/query/impact/read_admission.go).

Ceiling 14 re-declared (never embedded) methods — the two bounded graph
traversals, the eight ContentStore reads, the code-topic investigation,
the two live pod-template reads, and the ECS task read — sealed:

| # | Method | Mirrors | Serves (per pin closure) |
|---|---|---|---|
| 1 | `Run` | `querycontract.GraphQuery` | direct reads; selector/workload-candidate resolution; path decoders |
| 2 | `RunSingle` | `querycontract.GraphQuery` | single-row decoders |
| 3 | `GetEntityContent` | `ContentStore` | exposure-path anchor hydration |
| 4 | `SearchEntitiesByName` | `ContentStore` | deployment-trace resource match |
| 5 | `SearchEntitiesByExactNameAnyRepo` | `ContentStore` | service-not-found reference leg |
| 6 | `ListRepoEntities` | `ContentStore` | controller/semantic scans; provisioning chains |
| 7 | `ListRepoEntitiesByType` | `ContentStore` | ECS count scan |
| 8 | `ListRepoEntitiesByPaths` | `ContentStore` | change-surface path symbols |
| 9 | `ListRepoEntitiesByIDs` | `ContentStore` | deployment-trace rehydration |
| 10 | `ListRepoK8sSelectCandidates` | `ContentStore` | SELECTS candidate scan |
| 11 | `InvestigateCodeTopic` | `codequery.CodeTopicContentInvestigator` | change-surface root adapter |
| 12 | `HasLiveIdentityMatch` | `deployment.KubernetesPodTemplateStore` | live pod-template match |
| 13 | `ListLiveIdentityMatches` | `deployment.KubernetesPodTemplateStore` | live identity matches |
| 14 | `ListRunningTasks` | `deployment.ECSTaskStore` | running ECS task evidence |

There is deliberately no constructor on this pin: the impact seam spans
the graph reader, the content reader, the code-topic investigator, and
the live/ECS stores, and minting one constructor would pick a wiring
the satellite never runs. The satellite keeps building its backends the
way `cmd/api` wiring builds them today; the pin freezes only the read
shape the handlers may call through.

Enforcement (all in go/internal/query/impact, plus the query-level mirror):

- `read_admission_contract_test.go` — frozen method-set test plus
  version pin (`TestImpactReadAdmissionContractIsVersioned` — exactly 14
  methods), the satisfiability witness, the seeded re-signed rejection,
  and the widening-is-caught proof.
- `read_admission_wire_shapes_test.go` — the pin-satisfying canned store
  proves a satellite can consume the pin using only the pin plus the
  querycontract, codequery, and deployment shapes.
- `go/internal/query/w5_impact_read_admission_mirror.go` —
  compile-time proof that the production stores jointly serve the pin.

## Census (per route per method, per the pin closure)

In scope — the 13 W5 impact POSTs (W5c-e):

- `POST /api/v0/impact/trace-deployment-chain` (dual-pin: TraceContext
  leg rides EntityReadAdmission alongside this pin)
- `POST /api/v0/impact/blast-radius`
- `POST /api/v0/impact/change-surface`
- `POST /api/v0/impact/change-surface/investigate`
- `POST /api/v0/impact/contracts`
- `POST /api/v0/impact/deployment-config-influence` (dual-pin:
  TraceContext leg rides EntityReadAdmission alongside this pin)
- `POST /api/v0/impact/trace-exposure-path`
- `POST /api/v0/impact/pre-change`
- `POST /api/v0/impact/developer-change-plan`
- `POST /api/v0/impact/entity-map`
- `POST /api/v0/impact/resource-investigation`
- `POST /api/v0/impact/trace-resource-to-code`
- `POST /api/v0/impact/explain-dependency-path`

The graph pair serves the direct, selector, candidate, and decoder
reads on all 13; the eight content reads serve the anchor, match,
scan, symbol, rehydration, candidate, and reference legs; the
code-topic investigation serves the change-surface root adapter; the
live pair and the ECS read serve the deployment-evidence legs. The
trace-context enrichment leg is NOT served here: it requires
EntityReadAdmission alongside this pin by explicit dual-pin cite (see
contracts/entity-read-admission.md); silent reliance is forbidden and
this cite is the reliance. No other route may cite this pin as
coverage; citing Entity, CodeGraph, IaC, or CoreReadContractsVersion
here is void.

## Explicitly OUT (not widened by this pin)

- `ChangeSurfaceCodeBackend.FetchCodeSurface`,
  `DeploymentTraceContextProvider.FetchServiceTraceContext`, and
  `BuildServiceDeploymentOverview`: core logic and adapter seams, not
  reads. The topic search the code-surface leg exercises IS frozen here
  (InvestigateCodeTopic); the service-story closure the trace leg pulls
  IS frozen in the cited Entity pin.
- `PathProbeBackend` methods (ResolveAnchor, ResolveAnchorCandidates,
  TraceHops, DependencyHops, PathHasNodes, PathNodes,
  ResourceInvestigationHops): the production prober decodes raw driver
  values and only the query root may import the driver; the anchor
  resolutions read through the passed GraphQuery, already frozen here as
  Run and RunSingle.
- Every other ContentStore method: no W5 impact path calls them. The
  enrichment tail reads that fire on the trace leg (evidence files,
  framework routes, consumer search, artifact hydration, read-model
  facets) ride the cited Entity pin, not this one.
- `MatchRepositories` and `ListRepositories`: the impact selector legs
  never resolve a repository catalog entry, so there is no catalog
  shape to freeze here.
- Profile, capability gates, grant-before-limit checks, scoped-token
  node-by-node filtering, and the fixed failure messages are handler
  behavior, proven at byte-parity time and never frozen here.
- No serialization, no worker or batch reduction anywhere in this lane.

## Satellite consumption

Depend on the shape, not a constructor: serve the impact reads through
the existing graph reader, content reader, code-topic investigator,
and live/ECS stores, the same backends `cmd/api` wiring builds today.
A satellite PR consuming this pin for any other backend pairing is
invalid without arbiter review. Rollback: revert this pin. The seam is
unwired — no handler, route, or caller depends on it yet — so rollback
moves no traffic and changes no data.
