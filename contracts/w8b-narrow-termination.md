# W8b-narrow satellite termination

Satellite termination of the 15 W8b-narrow reads (Refs
`eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#101`). The wave table
lives in `api/w8bnarrow_wave.go`; the serving contract in
`api/w8bnarrow_terminate.go`; the router registration in
`api/router.go` (`registerW8bNarrowWave`, ahead of the generic
proxy). The scope-honesty label rides this record: the dest
authorizer covers the shared-key (AllScopes) identity only, and core
re-judges every forwarded call as backstop (tenant scope, permission
catalog, governance mode — where the live 403s come from).

## Pins

Fourteen deliberate core pins, each recorded byte-identical at the
core HEAD `eshu-hq/eshu#76240fda9f` and re-verified in-lane with zero
drift since its mint. The vendored `.go` port plus the frozen
method-set contract test are whole-file byte-identical at that HEAD
(gated in `scripts/check-parity.sh`); the `-read-wire-shapes.txt`
excerpt carries the version-const stanza, the in-scope route list,
the interface, and the surrounding contract comment, line-verbatim
from the pin file at the same HEAD.

| Pin | Record | Upstream mirrors | Methods | Covers |
| --- | --- | --- | --- | --- |
| `KubernetesCorrelationReadAdmission v0` | `contracts/kubernetes-correlation-read-admission.md` | `contracts/upstream/kubernetes_correlation_read_admission.go` (`177f031…`), `contracts/upstream/kubernetes_correlation_read_admission_contract_test.go` (`f333cae…`) | 1 | `GET /api/v0/kubernetes/correlations` |
| `ObservabilityCoverageCorrelationReadAdmission v0` | `contracts/observability-coverage-correlation-read-admission.md` | `contracts/upstream/observability_coverage_correlation_read_admission.go` (`77d9f61…`), `contracts/upstream/observability_coverage_correlation_read_admission_contract_test.go` (`84e662c…`) | 1 | `GET /api/v0/observability/coverage/correlations` |
| `WorkItemEvidenceReadAdmission v0` | `contracts/work-item-evidence-read-admission.md` | `contracts/upstream/workitem_evidence_read_admission.go` (`cc3c56d…`), `contracts/upstream/workitem_evidence_read_admission_contract_test.go` (`316d3fd…`) | 1 | `GET /api/v0/work-items/evidence` |
| `SemanticEvidenceReadAdmission v0` | `contracts/semantic-evidence-read-admission.md` | `contracts/upstream/semantic_evidence_read_admission.go` (`ffdc66d…`), `contracts/upstream/semantic_evidence_read_admission_contract_test.go` (`9690e92…`) | 1 | `GET /api/v0/semantic/code-hints`, `GET /api/v0/semantic/documentation-observations` |
| `DependencyReadAdmission v0` | `contracts/dependency-read-admission.md` | `contracts/upstream/dependency_read_admission.go` (`5278561…`), `contracts/upstream/dependency_read_admission_contract_test.go` (`78794e2…`) | 1 | `GET /api/v0/dependencies` |
| `ImageReadAdmission v0` | `contracts/image-read-admission.md` | `contracts/upstream/image_read_admission.go` (`b14633b…`), `contracts/upstream/image_read_admission_contract_test.go` (`77ceb88…`) | 1 | `GET /api/v0/images` |
| `GraphEntityInventoryReadAdmission v0` | `contracts/graph-entity-inventory-read-admission.md` | `contracts/upstream/graph_entity_inventory_read_admission.go` (`9114eb6…`), `contracts/upstream/graph_entity_inventory_read_admission_contract_test.go` (`99cb6ad…`) | 1 | `GET /api/v0/graph/entities` |
| `TagHistoryReadAdmission v0` | `contracts/tag-history-read-admission.md` | `contracts/upstream/tag_history_read_admission.go` (`5dc8e78…`), `contracts/upstream/tag_history_read_admission_contract_test.go` (`a4b3852…`) | 1 | `GET /api/v0/images/tag-history` |
| `RuntimeDeclarationReadAdmission v0` | `contracts/runtime-declaration-read-admission.md` | `contracts/upstream/runtime_declaration_read_admission.go` (`4c4507c…`), `contracts/upstream/runtime_declaration_read_admission_contract_test.go` (`05d9393…`) | 1 | `POST /api/v0/runtime-declarations/query` |
| `CloudInventoryReadAdmission v0` | `contracts/cloud-inventory-read-admission.md` | `contracts/upstream/cloud_inventory_read_admission.go` (`a2f232e…`), `contracts/upstream/cloud_inventory_read_admission_contract_test.go` (`c5fd866…`) | 2 | `GET /api/v0/cloud/inventory` |
| `IncidentContextReadAdmission v0` | `contracts/incident-context-read-admission.md` | `contracts/upstream/incident_context_read_admission.go` (`41992d6…`), `contracts/upstream/incident_context_read_admission_contract_test.go` (`c6ddac1…`) | 2 | `GET /api/v0/incidents/{incident_id}/context` |
| `SSMHostInventoryReadAdmission v0` | `contracts/ssm-host-inventory-read-admission.md` | `contracts/upstream/ssm_host_inventory_read_admission.go` (`2163b49…`), `contracts/upstream/ssm_host_inventory_read_admission_contract_test.go` (`e7b2a97…`) | 2 | `GET /api/v0/cloud/ssm-host-inventory` |
| `ServiceCatalogCorrelationReadAdmission v0` | `contracts/service-catalog-correlation-read-admission.md` | `contracts/upstream/service_catalog_correlation_read_admission.go` (`fa97393…`), `contracts/upstream/service_catalog_correlation_read_admission_contract_test.go` (`01415c3…`) | 3 | `GET /api/v0/service-catalog/correlations` |
| `CodeownersOwnershipReadAdmission v0` | `contracts/codeowners-ownership-read-admission.md` | `contracts/upstream/codeowners_ownership_read_admission.go` (`1ff07ee…`), `contracts/upstream/codeowners_ownership_read_admission_contract_test.go` (`75e862d…`) | 3 | `GET /api/v0/codeowners/ownership` |

The lane consumes only these narrow pins and cites no shared-port
pin; no other family may cite these pins as coverage. Recording a pin
whole never authorizes terminating routes beyond the 15: the
unconsumed pin methods on these routes (none — every pinned read is
exercised by its route) stay disclosed per pin record.

## Explicitly OUT (stay on the relay)

- The 2 drift routes (`POST /api/v0/cloud/runtime-drift/findings`,
  `GET /api/v0/investigations/drift/packet`) already terminated in
  the Wave-4 Drift lane under `eshu-hq/eshu-api#117` on the
  `CloudRuntimeDriftReadAdmission v0` pin (see
  `contracts/cloud-runtime-drift-termination.md`). This lane does not
  touch them.
- The component-extensions pair (`GET /api/v0/component-extensions`,
  `GET /api/v0/component-extensions/{component_id}/diagnostics`)
  terminated in the Wave-1b lane on the `ComponentExtensionReadAdmission v0`
  pin (see `contracts/component-extension-termination.md`).
  The TODO(w8b-followup) above is done.
- The 6 no-port routes rode the generic relay — each named an
  owed item. Four terminated in the G2 lane
  (api/g2_wave.go, contracts/g2-wave.md); the readiness pair below
  stays on the relay:
  - TODO(w8b-followup): readiness x2 (`GET
    /api/v0/collector-readiness`, `GET
    /api/v0/status/collector-readiness`) — owed: a readiness
    read-admission mint; the #130 readiness probe behavior is
    unchanged.
  - DONE (G2 wave): fact-schema x2 (`GET
    /api/v0/fact-schema-versions`, `GET
    /api/v0/fact-schema-versions/{fact_kind}`) — terminated on the
    FactSchemaReadAdmission v0 pin.
  - DONE (G2 wave): surface-inventory (`GET
    /api/v0/surface-inventory`) — terminated on the
    SurfaceInventoryReadAdmission v0 pin.
  - DONE (G2 wave): visualizations/derive (`POST
    /api/v0/visualizations/derive`) — terminated on the
    VisualizationReadAdmission v0 pin.

## Census (per route per method)

Every wave route admits its read through exactly the pinned method
named below; grant checks, fixed failure messages, packet bounds, and
QueryProfile are handler behavior OUT of every pin.

| Route | Pin method |
| --- | --- |
| `GET /api/v0/kubernetes/correlations` | `ListKubernetesCorrelations` |
| `GET /api/v0/observability/coverage/correlations` | `ListObservabilityCoverageCorrelations` |
| `GET /api/v0/work-items/evidence` | `ListWorkItemEvidence` |
| `GET /api/v0/semantic/code-hints` | `SemanticEvidence` |
| `GET /api/v0/semantic/documentation-observations` | `SemanticEvidence` |
| `GET /api/v0/dependencies` | `Run` (dependency-scoped re-declaration) |
| `GET /api/v0/graph/entities` | `Run` (graph-entity-scoped re-declaration) |
| `GET /api/v0/images` | `Run` (image-scoped re-declaration) |
| `GET /api/v0/images/tag-history` | `Run` (tag-history-scoped re-declaration) |
| `GET /api/v0/incidents/{incident_id}/context` | `ReadIncidentContext`, `ResolveDurableIncidentRepositories` |
| `POST /api/v0/runtime-declarations/query` | `Run` (declaration-scoped re-declaration) |
| `GET /api/v0/cloud/inventory` | `CloudInventoryIdentities`, `CloudInventoryPreRolloutEvidenceExists` |
| `GET /api/v0/cloud/ssm-host-inventory` | `SSMHostActiveFacts`, `Run` (SSM-scoped re-declaration) |
| `GET /api/v0/service-catalog/correlations` | `ListServiceCatalogCorrelations`, `ListServiceCatalogLocalDescriptorEvidence`, `MatchRepositories` |
| `GET /api/v0/codeowners/ownership` | `Run`, `RunSingle`, `ListServiceCatalogCorrelations` (ownership-scoped re-declaration) |

## Relay posture

Shared-key callers terminate on the native lane; foreign credential
classes (scoped token, OIDC bearer, session cookie) relay explicitly
on the relay lane so core judges them; credential-less callers — and
junk-cookie carriers — keep the standing dest 401 with zero core
contact. The only dest-synthesized errors on the terminated path are
the gate 401, the future-resolver family 403, and the permanent local
503 when no core origin is configured (never Retry-After). Rollback
is revert-to-relay: remove the `registerW8bNarrowWave` call and the
15 routes return to the generic relay behind RequireAuth.
