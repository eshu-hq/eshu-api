# W8b-101 census + per-family pin designs (PACKET-ONLY, blocked)

Refs `eshu-hq/eshu#9669`, satellite `eshu-hq/eshu-api#101` (S4b class:
pin-first narrow/no-port families, 25 routes exactly, rising port-width
order). Base `origin/main@7675119`. Worktree
`/home/ubuntu/personal-repos/eshu-worktrees/eshu-api-w8b-101`, branch
`drive/9669-w8b-101`. Part of `eshu-hq/eshu#9669`.

## Satellite census (origin/main)

23 of the 25 ride the generic relay (`mux.Handle("/api/", ...)` at the
tail of `api/router.go`, via `forwardToCore` in `api/dispatch.go`);
`classifyFamily` in `api/family.go` collapses all 23 to `other` (no
prefix entry matches). Zero satellite-side route, wave, pin, or spec
hits for the 23 (`rg` exit 1 across `api openapi internal contracts`).

2 of the 25 already terminate natively in the Wave-4 Drift lane
(`api/drift_wave.go`, `contracts/cloud-runtime-drift-termination.md`):

| Route | Status |
|---|---|
| `POST /api/v0/cloud/runtime-drift/findings` | NATIVE (drift wave, `CloudRuntimeDriftReadAdmission v0` @ `d81bd12802`) |
| `GET /api/v0/investigations/drift/packet` | NATIVE (drift wave, same pin) |

The drift pin is sealed to exactly these 2 (citing it for any other
route is a NO-GO per the arbiter W8b ruling); the issue's "do NOT cite
TerraformDrift or IaC pins" rule is already honored by that record.
`GET /api/v0/dependencies` is relay-only (used as the relay-leg
benchmark route in `api/drift_latency_test.go` / `api/w8a_latency_test.go`).

## Core method-level census (core `HEAD 820f9b6`, handlers under `go/internal/query`)

No-port families (zero store/graph reads on the leg):

| Family | Routes | Core read surface |
|---|---|---|
| visualization-derive | `POST /api/v0/visualizations/derive` | NONE (pure function of request body, `visualization/handler.go:deriveVisualizationPacket`) |
| collector-extraction-readiness | `GET /api/v0/collector-extraction-readiness`, `.../{family}` | NONE (static `extraction.Catalog()` / `extraction.Lookup`, `collector_extraction_readiness.go:59-82`) |
| component-extensions | `GET /api/v0/component-extensions`, `.../{component_id}/diagnostics` | NONE port-shaped (local FS `component.NewRegistry(h.ComponentHome).Readback`, `component_extensions.go:249`) |
| fact-schema-versions | `GET /api/v0/fact-schema-versions`, `.../{fact_kind}` | NONE (static `facts.SchemaVersion` / `ClassifySchemaVersion` / `SupportedSchemaVersions`, `fact_schema_version.go:105-151`) |
| surface-inventory | `GET /api/v0/surface-inventory` | NONE (`capabilitycatalog.LoadSurfaceInventory()`, `surface_inventory.go:51`) |

Narrow families (bounded read(s) per leg, rising width):

| Family | Routes | Core read surface |
|---|---|---|
| semantic | `GET /api/v0/semantic/code-hints`, `.../documentation-observations` | `SemanticEvidence(ctx, filter)` x1 shared, factKind-param (`semantic_evidence.go:181`) |
| work-items | `GET /api/v0/work-items/evidence` | `ListWorkItemEvidence(ctx, filter)` x1 (`workitem/handler.go:110`) |
| kubernetes | `GET /api/v0/kubernetes/correlations` | `ListKubernetesCorrelations(ctx, filter)` x1 (`kubernetes/handler.go:138`) |
| observability-coverage | `GET /api/v0/observability/coverage/correlations` | `ListObservabilityCoverageCorrelations(ctx, filter)` x1 (`observability/coverage/handler.go:142`) |
| service-catalog | `GET /api/v0/service-catalog/correlations` | `ListServiceCatalogCorrelations(ctx, filter)` x1 + access selector `ResolveForRequestWithAccess` (`service/catalog.go:128-156`) |
| incident-context | `GET /api/v0/incidents/{incident_id}/context` | `ReadIncidentContext(ctx, filter)` x1 (`incident/handler.go:118`) |
| dependencies | `GET /api/v0/dependencies` | `Neo4j.Run` x1 (`dependenciesCypher`, `dependency/handler.go`) |
| images | `GET /api/v0/images` | `Neo4j.Run` x1 (`imageListCypher`, `images.go`) |
| tag-history | `GET /api/v0/images/tag-history` | `taghistory.ReadWindow` / `ReadOffsetWindow` x1 (`tag_history.go:461-463`) |
| cloud-inventory | `GET /api/v0/cloud/inventory` | `CloudInventoryIdentities(ctx, filter)` x1 (`cloud_inventory_readback.go:199`; `CloudInventoryPreRolloutEvidenceExists` lives in the same iface but serves the rollout-signal leg, OUT) |
| graph-entities | `GET /api/v0/graph/entities` | `Neo4j.Run` x2 (kind-counts + kind-list, `graph_entity_inventory.go`) |
| ssm-host-inventory | `GET /api/v0/cloud/ssm-host-inventory` | `SSMHostActiveFacts(ctx, filter)` x1 + graph overlay via `ReadSSMHostInventory(ctx, h.Graph, store, ...)` (`ssm_host_inventory_handler.go:141`, `ssm_host_inventory_read_model.go:143`) |
| codeowners | `GET /api/v0/codeowners/ownership` | `loadCodeownersOwnershipRows` (Neo4j) + `resolveEffectiveRepositoryOwner` (Neo4j + `ListServiceCatalogCorrelations`) (`codeowners/ownership.go`, `ownership_rows.go:18`, `ownership_precedence.go:61`) |
| runtime-declarations | `POST /api/v0/runtime-declarations/query` | `Neo4j.Run` x2 (page + excluded) + `QueryRepositoryNamesByID` (`runtime/declaration/handler.go:152-195`) |

## Per-family pin designs (proposed mint order, narrow-first)

0. No-port group (8 routes, 5 families): NO pin. Record a static-source
   attestation per family (catalog/registry/pure-function, no store to
   freeze); satellite terminates behind the shared-key gate with
   byte-parity + latency proof only. Requires arbiter confirm that
   "no pin" satisfies "per-family core pins" for zero-read legs.
1. One-method correlation/readback pins (7 routes): `KubernetesCorrelationReadAdmission`,
   `ObservabilityCoverageCorrelationReadAdmission`,
   `ServiceCatalogCorrelationReadAdmission`, `IncidentContextReadAdmission`,
   `WorkItemEvidenceReadAdmission`, `SemanticEvidenceReadAdmission`
   (shared across both semantic routes, factKind-param),
   `CloudInventoryReadAdmission` (`CloudInventoryIdentities` only).
2. One-cypher graph pins (3 routes): `DependencyReadAdmission`,
   `ImageReadAdmission`, `TagHistoryReadAdmission` (re-declare the
   helper, not raw `Neo4j.Run`).
3. Two-read pins (3 routes): `GraphEntityInventoryReadAdmission`
   (counts + list), `SSMHostInventoryReadAdmission` (`SSMHostActiveFacts`
   + graph overlay), `CodeownersOwnershipReadAdmission` (ownership rows +
   effective-owner incl. the service-catalog correlation leg).
4. Widest last (1 route): `RuntimeDeclarationReadAdmission` (page +
   excluded + name resolution).
5. Already done (2 routes): `CloudRuntimeDriftReadAdmission v0` minted
   at `d81bd12802` — no new mint, no citation elsewhere.

Every pin: `v0`, re-declared (never embedded) methods, `Run`/`RunSingle`-
style twins only where the leg uses them, no transport constructor (dest
serves through the existing Postgres/graph adapters the way `cmd/api`
wiring builds them today), handler behavior (grant checks, failure
messages, bounds, truth envelope, QueryProfile) stays unpinned.

## Arbiter packet (questions owed)

1. Does the no-port group (0) terminate with NO pin (static attestation
   only), or must each family still mint a vacuous pin?
2. Is the family split above (esp. separating the three correlation
   families vs. one shared correlation pin) the ruled shape?
3. Is `component-extensions` (local-FS readback) in scope for
   satellite termination at all, given the readback is
   environment-local (`ESHU_COMPONENT_HOME`)?
4. Confirm `CloudInventoryPreRolloutEvidenceExists` stays OUT of the
   cloud-inventory pin, and the service-catalog leg inside
   `resolveEffectiveRepositoryOwner` rides the codeowners pin (not the
   service-catalog pin).

No arbiter model is reachable from this leaf; rulings are owed
arbiter-first before any core mint or satellite record.

## Blocked state

- Core mints: NONE yet for the 23 (owning pin NONE, per the issue).
- Arbiter: no APPROVED per-family design ruling recorded on #101.
- Satellite: no record/terminate work starts until mints merge +
  APPROVED (arbiter-first). Next: mint core pins in the order above,
  then satellite record + terminate + byte-parity + latency proof, then
  tick exactly these 25 in `eshu-hq/eshu#10491`.
- Acceptance sweep vs `origin/main@7675119`: all three boxes unchecked;
  tracker `eshu-hq/eshu#10491` OPEN. No new issues created; no sibling
  packets touched (125/124/122 intact).

## Proof

- `go build ./...` ok (worktree, unchanged tree + this doc).
- Focused: no code touched, so no `scripts/check-parity.sh` drift (touched
  pins: none) and no test rerun owed beyond the build.
- No `make pre-pr` / `make pre-push` (coordinator-only, never run).
