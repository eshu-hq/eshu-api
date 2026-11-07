# D6 Status + SupplyChain reads termination contract

Versioned read-termination record for the D6 wave (Refs
`eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). This lane terminates
the 29 pinned read routes natively against the D0-proven terminated
transport: dest owns the route (caller gate, family authorizer,
response mapping) and calls the versioned core read API as its port
transport. Dest holds no table or queue handle; authz stays atomic
core-side.

Scope source: `go/cmd/api/CUTOVER_OWNERSHIP.md` at the D6 pin commit
(`6b66e6b23efe226970b4571c955aed1dc42f987a`, merge of
`eshu-hq/eshu#10303`): the Status read rows admitting through
StatusReadAdmission v0 and the SupplyChain read rows admitting
through SupplyChainReadAdmission v0. Compare is deferred to a later
pin; the suppressions mutation row stays out via SuppressionAdmission.

Transport pins:

- `StatusReadAdmission v0`
  (`contracts/upstream/status_read_admission_contract_test.go` at the
  D6 pin) covers the 12 Status wave rows — every row's snapshot read
  admits through `status.Reader` (`ReadStatusSnapshot`,
  `ReadStatusSnapshotFiltered`).
- `SupplyChainReadAdmission v0`
  (`contracts/upstream/supply_read_admission_contract_test.go` at the
  D6 pin) covers the 17 SupplyChain wave rows — every row admits
  through the frozen family-owned reader ports (SBOM/attestation,
  container-image, security-alert, impact finding/aggregate/explain/
  readiness, advisory catalog/evidence stores, plus the
  runtime-inventory filters).

The minting rule forbids inventing pins beyond these two: Compare
admits only through the shared graph/content ports and needs a
deliberate later pin with arbiter review; the suppressions mutation
row stays the Never-satellite survivor via SuppressionAdmission
(already pinned, already on its tier-C write-through lane).

## Status

Termination claims stay provisional until the arbiter re-reviews the
D0 evidence (blocker B1). What this lane proves per the D-wave ruling
is the required proof list: explicit routes ahead of relay,
byte-identical fragments under `scripts/check-parity.sh`, authorizer +
deny-shape tests with scope-honesty labels, byte-parity
dispatch/conformance tests vs the relayed reference (including the
suppression-state filtering cases the Decision 2 split requires), the
no-core-import check, extended `viaNative`/`viaRelay` telemetry, the
satellite latency gate with before/after numbers, and this contract
section with rollback.

## Production binding

- Transport: the existing versioned core HTTP read API
  (`CoreReadContractsVersion v0`), bounded client (30s, no redirect,
  no retry; no dest-side byte budget — `forwardToCore` streams the
  body unbounded), forwarded credential and negotiation headers,
  guaranteed correlation ID, fail-closed local 503 (permanent, never
  `Retry-After`) when no core origin is configured.
- Envelope negotiation (eshu-hq/eshu-api#66): success shape stays
  core-owned. The edge forwards the caller `Accept` verbatim and
  relays the core bytes verbatim — it never wraps a flat core answer
  into `{data, truth}`. Where core renders flat (its `WriteJSON`
  reads), an envelope-negotiated caller still receives the flat shape
  with truth inline (`result_limits`, pagination, group fields);
  where core negotiates (its `WriteSuccess` routes), the envelope
  flows through untouched. Dest invents no truth levels (it holds no
  read model); dest-synthesized errors stay envelope-aware via
  `writeError`.
- Trust model: dest's only constructible identity is the shared key,
  which core maps to AllScopes. The dest authorizer allows it; core
  re-judges every forwarded call as backstop (tenant scope, permission
  catalog, governance mode) — which is also where the live 403s and
  mode-dependent denials come from. Dest adds no handler validation and
  emits no audit: request/response semantics stay core-owned.
- Reads emit no governance audit events on either path: the in-tree
  query handlers gate on permission features but never append, so there
  is no audit write to keep atomic; idempotency is the GET repeat
  check in the conformance matrix.

## Per-family record

Scope-honesty label (all families): the dest authorizer covers the
shared-key (AllScopes) identity only. Scoped tokens, OIDC bearers, and
browser sessions relay explicitly — core judges, core bytes come back
untouched, dest invents no 401/403/404/500 for them — until the D8
session resolver lands. The deny branch (`allScopes=false`) is pinned
for that day and unreachable on live routes.

| Family | Routes | Serve | Authorizer note |
| --- | --- | --- | --- |
| Status | 12 GETs (`collector-readiness`, `collectors`, `ingesters`, `ingesters/{ingester}` in legacy and `/status/` spellings, plus `status/pipeline`, `status/collector-readiness`, `status/freshness-causality`, `status/operator-control-plane`, `status/semantic-extraction`) | wave-gated terminate / relay-explicit | shared-key allow |
| SupplyChain | 17 GETs (`supply-chain/advisories`, `advisories/evidence`, `container-images/identities` + `count` + `inventory`, `impact/explain`, `impact/findings` + `count` + `inventory`, `sbom-attestations/attachments` + `count` + `inventory`, `security-alerts/reconciliations` + `count` + `inventory`, `vulnerabilities/{advisory_id}`, `vulnerability-scanner/contract`) | wave-gated terminate / relay-explicit | shared-key allow |

Cookie rule (D2 follow-up lesson, shared with the D3/D4 gates): only
session-cookie-shaped credentials relay — a Cookie member named
`eshu_session` (or `__Host-eshu_session`, the two names the core
caller-credential reader accepts) relays explicitly; any other Cookie
content is junk, not a browser session, and stays dest-401 with zero
core contact.

Known divergences (disclosed, not hidden):

- The six extra-port Status aspects stay relay with per-route reasons
  (the pins do not cover them, and nobody widens them here):
  `index-status` + `status/index` (Neo4j repository counts plus the
  Retirements tombstone aggregate), `status/governance` (the
  GovernanceAudit Summary — the D1 pin freezes the three-method
  List/Summarize port on `identity.GovernanceAuditReader`, not the
  one-method Summary port the Status path admits through),
  `status/operations` (the bounded LiveActivity read model),
  `status/answer-narration` (the dest-injected NarrationPosture
  override), `status/hosted-readiness` (Neo4j repository count plus
  workload leftovers).
- The `investigations/supply-chain/impact/packet` read stays relay:
  it admits through the writer `ImpactPacketResponder` plus the shared
  graph/content ports the SupplyChain pin explicitly excludes.
- Compare (`POST /api/v0/compare/environments`) stays relay: it
  admits only through the shared graph/content ports, never through
  `status.Reader`; pinning those would be a blanket pin.
- The suppressions mutation row stays on its tier-C write-through
  lane via SuppressionAdmission (the Decision 2 survivor, not a read
  port) — never in this read wave.
- Decision 2 proof obligation: parity exercises suppression-state
  filtering (`suppression_state`, `include_suppressed` on the
  findings list/count/inventory legs; `provider_state`,
  `reconciliation_status` on the reconciliation legs), and the
  forwarded-filter test pins the queries arrive core-side intact.
- Session-dependent aspects relay until D8; scoped/browser identity
  classes relay explicitly.
- The status reads fragment ports only the seven lane entries from
  the shared `routes.go` core file — never the relayed index,
  governance, operations, answer-narration, hosted-readiness, or
  mux-level openapi.json keys sharing that file.
- The two finding fragments ride dest-owned exported wrappers
  (`ImpactFindings`, `ImpactExplain`) over the core unexported consts
  and share the byte-identical `runtime_context.go` sibling.
- The served document gains the `CoordinatorSource` and
  `TerraformStateSource` schemas the terminated status entries
  reference; both blocks are line-verbatim from the pinned core
  components source.
- Unconfigured origin answers the permanent local 503 on terminated
  routes where core nil-safe handlers would answer zero-defaults —
  the standing fail-closed rule, uniform across native lanes.

## Spec posture

No D6 fragment carries a `"security": []` public-route marker: the
only served marker stays the permanently-public login-picker entry
(`GET /api/v0/auth/providers`), and the posture test matches the
marker shape (`"security":[]` re-marshaled) rather than the bare word.
The D6 fragments add no new `security` keys; the next lane that ports
another public-marked fragment revisits that test again instead of
landing silently.

## Rollback

Single-commit revert: remove the D6 wave registrations in
`api/router.go` and the lane reverts to the generic byte-relay with
zero state change (core untouched, no durable dest state, no data
migration). The served spec drops the D6 keys with the fragments; the
relay legs keep serving them from core. Verify by re-running the lane
conformance suite against the reverted tree: the ahead-of-relay
assertions go red (routes relay again) while every other suite stays
green. No Plan A rehearsal is consumed by this rollback — it restores
a proxy path, not durable state.
