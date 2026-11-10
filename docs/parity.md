# Parity and compatibility

Seed for `eshu-hq/eshu#8872` (part of `eshu-hq/eshu#6707`). This note
records what moves, what stays, the compatibility rule, and the proofs
still open. It is a preparation record, not a cutover claim.

## Census basis

`go/cmd/api/CUTOVER_OWNERSHIP.md` at the provenance pin maps all 278
live API routes: 228 `read` rows backed by reader ports only
(`db.ReadStore`, graph `GraphQuery`, `ContentStore`), 50 `mutation` rows
persisting through core-owned admission storage, mounted by `newRouter`
(247) or `wireAPI` (31). The OpenAPI fragments under
`go/internal/query/openapi/` publish the request/response shapes; the
spec assembly order is the compatibility surface the destination owns
after the move.

## What moves

- Read families (all 228 `read` rows): consumers of the versioned core
  read contracts recorded in `contracts/core-read-contracts.md`. No
  durable writes travel with them. The D1 read-admission pins (Admin,
  AdminIdentity, GovernanceAudit, and TerminalFailure reads, Refs
  `eshu-hq/eshu#9669`) are recorded in
  `contracts/admin-read-admission.md`,
  `contracts/admin-identity-admission.md`,
  `contracts/governance-audit-read-admission.md`, and
  `contracts/terminal-failure-admission.md`; the D3 scope (16 routes
  across those four pins, minus the families D2 already terminated)
  terminates natively per `contracts/d3-admin-reads.md`, including
  the 3 formerly unpinned Admin GETs now frozen under the D3 trio
  pins; the D4 scope (45 Native-next routes
  across CICD, Documentation, Infra, PackageRegistry, Repositories,
  Evidence, InvestigationWorkflows, and Playbooks — Compare since
  terminated in its gap-04 lane, the story GET on the generic relay
  as a relay leg) terminates
  natively per `contracts/d4-native-next.md` on the
  `CoreReadContractsVersion v0` port transport with no per-family
  admission alias; the D6 scope (12 snapshot-pure Status GETs plus 17
  SupplyChain read GETs plus the 3 phase-2 extra-port Status legs, with
  the remaining extra-port Status aspects, the impact packet,
  and the suppressions mutation row staying relay by disclosure)
  terminates natively per
  `contracts/d6-status-supplychain.md` on the `StatusReadAdmission
  v0`, `SupplyChainReadAdmission v0`, and (phase 2)
  `StatusExtraReadAdmission v0` port transports; the gap-04 Compare
  POST terminates natively per `contracts/compare-termination.md` on
  the `CompareReadAdmission v0` port transport; and the rest
  stay on the generic relay until their D waves land.
- Mutation HTTP shapes (all 50 `mutation` rows): the routes move, the
  stores do not. Each persists through an explicit versioned core
  admission contract; the versioned in-tree seams are the suppression,
  terminal-failure, replay-ledger, sign-in, retirement, admin-identity,
  setup, provider-config, OIDC login, GitHub login, and SAML login
  admissions recorded in
  `contracts/suppression-admission.md`,
  `contracts/terminal-failure-admission.md`,
  `contracts/replay-ledger-admission.md`,
  `contracts/sign-in-admission.md`,
  `contracts/retirement-admission.md`,
  `contracts/admin-identity-admission.md`,
  `contracts/setup-admission.md`,
  `contracts/provider-config-admission.md`,
  `contracts/oidc-login-admission.md`,
  `contracts/github-login-admission.md`,
  `contracts/saml-login-admission.md`, and
  `contracts/recovery-admission.md`.
- OpenAPI contract fragments and the `api/` server, router, auth, v1
  alias, and deprecation middleware: move in the follow-up extraction
  PR, reviewed row-for-row against the ownership map. This seed carries
  only the versioned seams, not the fragments. Landed since: the
  sign-in-policy fragment (`openapi/paths/auth/sign_in_policy.go`, #38),
  the retirement fragment
  (`openapi/paths/status/repository_retirement.go`, #39), the
  terminal-failure fragment
  (`openapi/paths/status/collector_terminal_failure_acknowledgement.go`,
  #40), the admin-identity mutation fragment
  (`openapi/paths/auth/admin_mutations.go`, #42), the
  provider-config mutation fragment
  (`openapi/paths/auth/admin_provider_config_mutations.go`, #46),
  and the admin-recovery mutation fragment
  (`openapi/paths/status/admin_recovery.go`, C-c2 lane #47), and the
  suppression fragment
  (`openapi/paths/supply/chain/suppression_mutation.go`, D0 lane —
  line-verbatim subset port closing the Decision 5(c) gap the C-b
  ruling left open for the pilot family),
  and the D2 Leaf B Native-now wave (this lane): the capabilities,
  CLI support (score/render/capture/validate/compose), ask, service
  intelligence, auth providers, provider-config reads, and served-spec
  self-entry fragments (whole-file ports plus three line-verbatim
  subsets), with the shared components block extended by the
  ServiceName parameter, AdminProviderConfig/WriteRequest schemas, and
  Unauthorized/NotImplemented responses the wave entries reference,
  and the D3 admin/identity/recovery reads wave: the six admin query
  POSTs (`openapi/paths/status/admin_reads.go`, line-verbatim subset)
  and the invitations list GET
  (`openapi/paths/auth/admin_invitations.go`, line-verbatim get-only
  subset),
  and the D4 Native-next wave: the CICD routes and run-correlation
  aggregates, the documentation facts/findings entries and finding
  aggregates, the cloud resources entry, the infrastructure
  ecosystem/relationship entries and resource aggregates, the
  package-registry entries and aggregates, the repository catalog,
  list, language, branches, content, context, coverage, freshness,
  stats, and tree entries, the evidence admission-decision,
  relationship, citation, bundle, and deployable-unit entries, the
  investigation-workflow entries, and the query-playbook entries
  (fifteen whole-file ports plus three line-verbatim subsets: the
  evidence routes without core's dangling 501, the deployable-unit
  packet entry alone, and the repository routes without the
  mux-health and schema-interpolated story entries), with the shared
  components block extended by the RepoId parameter, the
  CollectorReadinessEnvelope, Repository, RepositoryRef,
  CatalogWorkload, InvestigationEvidencePacket, Relationship, and
  ActiveWorkSource schemas, and the GatewayTimeout response the wave
  entries reference,
  and the D6 Status + SupplyChain reads wave: the status reads, status
  extra reads, operations, status collector-readiness,
  operator-control-plane, semantic-extraction, and freshness causality
  entries (two line-verbatim status subsets plus five whole-file ports),
  the supply-chain advisory, container-image,
  impact aggregate, SBOM, security-alert, and vulnerability-scanner
  contract entries (ten whole-file ports plus the runtime-context
  sibling), and the impact findings and impact explain entries (two
  line-verbatim subsets sharing that sibling), with the shared
  components block extended by the CoordinatorSource and
  TerraformStateSource schemas the status entries reference,
  and the D7 admin-four tier-C clone lane: the backfill, dead-letter,
  reindex, and skip entries (`openapi/paths/status/admin_d7.go`, one
  line-verbatim subset, no new schemas),
  and the D8 session-resolver tier-C clone lane: the browser-session
  lifecycle, session-list, profile, and local-identity entries (five
  line-verbatim `routes.go` subsets, one merged into the D3
  invitations key) plus the API-token and TOTP entries (two
  whole-file ports), with the shared components block extended by the
  nine LocalIdentity schemas and the BrowserSessionAuth and
  BrowserSessionResponse schemas the lane entries reference —
  per-family fragment moves now land in their
  family PRs per satellite-design Decision 5(c) on eshu-hq/eshu#9669.

## Native-now termination (D2)

The frozen 18 routes / 8 families terminate natively per
`contracts/leafb-native-now.md`: explicit per-route registrations
ahead of the generic relay, the shared-key family authorizer with the
documented deny shape, byte-parity dispatch/conformance proof vs the
relayed reference, extended `viaNative`/`viaRelay` telemetry, and the
satellite-owned read-latency gate (`docs/leafb-latency.md`). Data
still comes from the versioned core read API (dest holds no
table/queue handle); identity classes dest cannot resolve relay
explicitly. Termination claims stay provisional until the arbiter
re-reviews the D0 evidence (blocker B1).

## Native termination (D3)

The 16 D1-pinned admin/identity/recovery read routes terminate
natively per `contracts/d3-admin-reads.md`: explicit gated routes
ahead of the generic relay, the shared-key family authorizer with the
documented deny shape (plus the D2 follow-up cookie lesson — only
session-cookie-shaped credentials relay, junk cookies stay dest-401),
byte-parity dispatch/conformance proof vs the relayed reference,
extended `viaNative`/`viaRelay` telemetry, and the satellite-owned
read-latency gate (`docs/d3-latency.md`: GET 0.55x, POST 0.79x vs the
1.50 budget). Data still comes from the versioned core read API (dest
holds no table/queue handle). Termination claims stay provisional
until the arbiter re-reviews the D0 evidence (blocker B1).

## Native termination (D4)

The 45 Native-next read routes (CICD, Documentation, Infra,
PackageRegistry, Repositories, Evidence, InvestigationWorkflows,
Playbooks) terminate natively per `contracts/d4-native-next.md`:
explicit gated routes ahead of the generic relay, the shared-key
family authorizer with the documented deny shape, byte-parity
dispatch/conformance proof vs the relayed reference, extended
`viaNative`/`viaRelay` telemetry, and the satellite-owned
read-latency gate (`docs/d4-latency.md` with the relay-route
disclosure). The transport pin is `CoreReadContractsVersion v0` with
no per-family admission alias — the minting rule forbids inventing
one, and all 45 rows are reader-port reads. The story GET stays on
the generic relay as a carried leg (its spec entry waits on the
schema helper port). Divergences stay disclosed in the contract: the
admission-decisions 501 waits for core to define the response, four
literals skip the duplicate slash registration (their slash spellings
still terminate under the parent subtree), and the D2/D3 relay legs
moved to the still-relaying dependencies and code-bundle routes. Termination claims stay provisional until the
arbiter re-reviews the D0 evidence (blocker B1).

## Native termination (D6)

The 32 pinned Status + SupplyChain read routes (12 snapshot-pure
Status GETs, 17 SupplyChain read GETs, plus the 3 phase-2 extra-port
Status legs) terminate natively per
`contracts/d6-status-supplychain.md`: explicit gated routes ahead of
the generic relay, the shared-key family authorizer with the
documented deny shape, byte-parity dispatch/conformance proof vs the
relayed reference, extended `viaNative`/`viaRelay` telemetry, and the
satellite-owned read-latency gate (`docs/d6-latency.md` with the
relay-route disclosure). The transport pins are `StatusReadAdmission
v0` (aliases `status.Reader`, 2 methods), `SupplyChainReadAdmission
v0` (embeds the 17 family-owned reader ports, 23 methods), and
`StatusExtraReadAdmission v0` (embeds the LiveActivity and retirement
readers, 2 methods). Decision 2 stays a proven split: parity exercises
suppression-state filtering (`suppression_state`,
`include_suppressed`, `provider_state`, `reconciliation_status`), and
the suppressions mutation row stays the Never-satellite survivor via
SuppressionAdmission. Divergences stay disclosed in the contract: the
three remaining extra-port Status aspects (governance,
answer-narration, hosted-readiness) stay relay with per-route reasons,
the impact packet stays relay (writer responder plus shared ports),
Compare terminates in its gap-04 lane on the now-recorded deliberate
later pin (`CompareReadAdmission v0` in
`contracts/compare-read-admission.md`; see
`contracts/compare-termination.md`), the status reads
fragment ports only the seven snapshot lane keys (the phase-2 index
and operations keys ride their own fragments), and the D2-D4 relay
legs moved to the still-relaying dependencies route. Termination
claims stay provisional until the arbiter re-reviews the D0 evidence
(blocker B1).

## Native termination (Wave-1 Freshness)

The 3 pinned Freshness GETs (`freshness/changed-since`,
`freshness/generations`, `freshness/services/changed-since`) terminate
natively per `contracts/freshness-wave.md`: explicit gated routes ahead
of the generic relay, the shared-key family authorizer with the
documented deny shape, byte-parity dispatch/conformance proof vs the
relayed reference (including the filter-query forwarding cases the pin
shape requires), lane-file hygiene (no dest-side store or parse) plus
filter-value opacity in logs and metrics, extended
`viaNative`/`viaRelay` telemetry, and the satellite-owned read-latency
gate (`docs/freshness-latency.md`: 1.02x, 1.00x, 1.03x vs the 1.50
budget). The transport pin is `FreshnessReadAdmission v0`
(re-declares the 3 StatusStore signatures, sealed — no ledger,
summary-reader, or snapshot surface); the three spec fragments port
whole-file byte-identical (all standalone entries, so no disclosed
absence was needed). Divergences stay disclosed in the contract: the
earlier-lane freshness-path reads (repository freshness, documentation
freshness, freshness causality) keep their own lanes and families, and
the out-of-pin legs stay out with no wire change. Termination claims
stay provisional until the arbiter re-reviews the D0 evidence
(blocker B1).

## Native termination (Wave-2 Content)

The 3 pinned Content POSTs (`content/files/read`,
`content/files/lines`, `content/entities/read`) terminate natively per
`contracts/content-point-reads-wave.md`: explicit gated routes ahead
of the generic relay, the shared-key family authorizer with the
documented deny shape, byte-parity dispatch/conformance proof vs the
relayed reference (including the line-range slicing-edge bodies the
pin shape requires), lane-file hygiene (no dest-side store or parse)
plus body-value opacity in logs and metrics, extended
`viaNative`/`viaRelay` telemetry, and the satellite-owned read-latency
gate (`docs/content-latency.md`: 0.98x, 0.99x, 1.04x vs the 1.50
budget). The transport pin is `ContentPointReadAdmission v0`
(re-declares the 3 ContentStore signatures, sealed — no search, no
list, no ledger/summary/snapshot surface); the three spec entries port
as a line-verbatim subset (mid-list entries, so no disclosed absence
was needed). Divergences stay disclosed in the contract: the
out-of-pin search legs keep relaying with their fragments core-side,
the list and scoped-entity legs keep their own standing, and non-POST
spellings stay unregistered. Termination claims stay provisional until
the arbiter re-reviews the D0 evidence (blocker B1).

## Tier-C dispatch clones (D7)

The four D7-pinned admin-four mutations (backfill, dead-letter,
reindex, skip) cross as tier-C dispatch clones per
`contracts/d7-admin-four.md`: explicit routes inside `RequireAuth`
ahead of the generic relay with the AllScopes family authorizer,
method, path, query, headers, and body forwarded to the same core
route, core bytes relayed back unmodified. Dest mints no idempotency
key, keeps no ledger row, appends no audit event, and runs no
finalize — the claim/complete pair (where the core handler has one)
runs core-side on every forwarded request. Proof per clone:
failing-first conformance, idempotency-key bytes forwarded untouched
(validity stays core's judgment), zero dest retries with the
unstable-core matrix (500, retryable and permanent 503s, dead core,
pre-cancel and mid-flight cancel — exactly one upstream hit each),
response-envelope parity vs the legacy relay (byte-identical status,
headers, body across the outcome matrix in both shapes), duplicate
deliveries no worse than direct core (2 hits, recorded outcome
replayed; backfill records a second row — not even state-idempotent,
disclosed), no-core-import, the contract with revert-to-relay
rollback, and the satellite-owned clone-latency gate
(`docs/d7-latency.md`: dead-letter 0.91x, reindex 0.90x vs the 1.50
budget). Scope honesty: tier-C dispatch only — not termination, not
removal readiness. The ledger deficit stays open; serialization
dest-side is not a substitute for it. Termination stays last among
mutations on D0-proven transport.

## Tier-C dispatch clones (D8)

The 22 D8-pinned session/token-lifecycle routes (BrowserSessions 4,
SessionList, Profile, LocalIdentity 16; pins re-anchored byte-identical
at `aa58debd` from the stale `91f2df28db` pin — re-anchor, no content
change) cross as tier-C dispatch clones per
`contracts/d8-session-resolver.md`: the 18 authed routes
inside the credential-class gate ahead of the generic relay with the
family authorizer (shared-key callers ride the native write-through;
scoped tokens, OIDC bearers, and session-cookie carriers relay
explicitly so core judges them; credential-less callers keep the
standing 401), and the 4 pre-auth public routes (local login,
invitations accept, break-glass session, password rotate) on exact
paths outside every gate with credential bodies crossing as opaque
bytes. Method, path, query, headers, and body forward to the same
core route; core bytes come back unmodified. Dest holds no session,
identity, token, or TOTP state, mints nothing, keeps no ledger,
appends no audit, and runs no finalize. Proof per family:
failing-first conformance, the credential-class gate (foreign
credentials relay, junk cookies stay 401), per-exact-route
public-carve-out proof (only the four paths leave the gate; no
shared-key injection; slash spellings stay relayed), the bootstrap
per-route review (gated half, operator seal stays core-side),
zero dest retries with the unstable-core matrix (500, retryable and
permanent 503s, dead core, pre-cancel and mid-flight cancel —
exactly one upstream hit each), response-envelope parity vs the
legacy relay (byte-identical status, headers, body across the outcome
matrix in both shapes), duplicate deliveries no worse than direct
core (2 hits, matching landed statuses), no-core-import, the
contract with revert-to-relay rollback, and the satellite-owned
clone-latency gate (`docs/d8-latency.md`: five legs 0.98–1.01x vs
the 1.50 budget). Scope honesty: tier-C dispatch only — not
termination, not removal readiness. The resolver, ledger,
client-stable keys, and finalize/audit stack stay open termination
obligations for the later lane. Termination stays last on
D0-proven transport only.

## Tier-C dispatch clones (D9 OIDC)

The 2 D9-pinned OIDC-login routes (login start, callback) cross as
tier-C dispatch clones per `contracts/d9-oidc-login.md` — the first
of three sequential family lanes (OIDC, then GitHub, then SAML):
both pre-auth public GETs on exact paths outside every gate ahead of
the generic relay, with the credential query values (state, code,
provider IDs, return_to) crossing as opaque bytes. Method, path,
query, headers, and (empty) body forward to the same core route;
core bytes come back unmodified — including the 302/303 redirect
status with its Location (never followed dest-side), the multiple
Set-Cookie session cookies, and the 503/400/401/403 error envelopes.
Dest holds no OIDC state, session, or provider material, mints
nothing, keeps no ledger, appends no audit, and runs no finalize.
Proof per family: failing-first conformance, per-exact-route
public-carve-out proof (only the two GET paths leave the gate; no
shared-key injection; slash spellings and non-GET spellings stay
relayed; GitHub/SAML stay gated for their own lanes), the
redirect-unfollowed pin (dispatchClient ErrUseLastResponse, proved by
a Location that points back at the stub with exactly one hit), zero
dest retries with the unstable-core matrix (500, retryable and
permanent 503s, dead core, pre-cancel and mid-flight cancel —
exactly one upstream hit each; these GETs are census mutations, so
one forward is the whole exactly-once story dest-side),
response-envelope parity vs the legacy relay (byte-identical status,
headers, body across the outcome matrix in both shapes), duplicate
deliveries no worse than direct core (2 hits, matching landed
statuses), the disabled-provider row (core conditionally mounts —
the relay carries core's real unmounted 404, synthesizing nothing),
credential-hygiene proof (dest reads no query values; no RequestURI,
Location, or Set-Cookie value in logs, metrics, or spans),
no-core-import, the contract with revert-to-relay rollback, and the
satellite-owned clone-latency gate (`docs/d9-latency.md`: five legs
0.73–0.95x vs the 1.50 budget). Scope honesty: tier-C dispatch only
— not termination, not removal readiness. The state store, replay
cache, session mint, and finalize/audit stack stay open termination
obligations for a later lane. Termination stays last on D0-proven
transport only.

## Tier-C dispatch clones (D9 GitHub)

The 2 D9-pinned GitHub-login routes (login start, callback) cross as
tier-C dispatch clones per `contracts/d9-github-login.md` — the
second of three sequential family lanes (OIDC, then GitHub, then
SAML): both pre-auth public GETs on exact paths outside every gate
ahead of the generic relay, with the credential query values (state,
code, provider IDs, return_to) crossing as opaque bytes. Method,
path, query, headers, and (empty) body forward to the same core
route; core bytes come back unmodified — including the 302/303
redirect status with its Location (never followed dest-side), the
multiple Set-Cookie session cookies, and the 503/400/401/403 error
envelopes. Dest holds no GitHub state, session, or provider
material, mints nothing, keeps no ledger, appends no audit, and runs
no finalize. Proof per family: failing-first conformance,
per-exact-route public-carve-out proof (only the two GET paths leave
the gate; no shared-key injection; slash spellings and non-GET
spellings stay relayed; OIDC stays on its own landed lane and SAML
stays gated for its own lane), the redirect-unfollowed pin
(dispatchClient ErrUseLastResponse, proved by a Location that points
back at the stub with exactly one hit), zero dest retries with the
unstable-core matrix (500, retryable and permanent 503s, dead core,
pre-cancel and mid-flight cancel — exactly one upstream hit each;
these GETs are census mutations, so one forward is the whole
exactly-once story dest-side), response-envelope parity vs the legacy
relay (byte-identical status, headers, body across the outcome matrix
in both shapes), duplicate deliveries no worse than direct core (2
hits, matching landed statuses), the disabled-provider row (core
conditionally mounts — the relay carries core's real unmounted 404,
synthesizing nothing), credential-hygiene proof (dest reads no query
values; no RequestURI, Location, or Set-Cookie value in logs,
metrics, or spans), no-core-import, the contract with revert-to-relay
rollback, and the satellite-owned clone-latency gate
(`docs/d9-latency.md`: five legs 0.90–0.99x vs the 1.50 budget).
Scope honesty: tier-C dispatch only — not termination, not removal
readiness. The state store, replay cache, session mint, and
finalize/audit stack stay open termination obligations for a later
lane. Termination stays last on D0-proven transport only.

## Tier-C dispatch clones (D9 SAML)

The 3 D9-pinned SAML-login routes (metadata GET, login-start GET,
ACS POST) cross as tier-C dispatch clones per
`contracts/d9-saml-login.md` — the third of three sequential family
lanes (OIDC, then GitHub, then SAML): the two pre-auth public GETs
plus the pre-auth public ACS POST on exact paths outside every gate
ahead of the generic relay, with the provider path value, the
`return_to` query value, and the IdP-posted form fields
(`RelayState`, `SAMLResponse`) crossing as opaque bytes. Method,
path, query, headers, and body forward to the same core route; core
bytes come back unmodified — including the metadata XML with its
content type, the 302 login-start redirect with its Location (never
followed dest-side), the multiple Set-Cookie session cookies, and the
400/401/404 error envelopes. Dest holds no provider material,
session, replay reservation, or request record, mints nothing, keeps
no ledger, appends no audit, runs no finalize, parses no form, and
applies no body cap of its own. Proof per family: failing-first
conformance, per-exact-route public-carve-out proof (only the three
paths leave the gate; no shared-key injection; slash spellings,
wrong-method spellings, the empty-provider and multi-segment
spellings, and the escaped-slash agreement row all stay relayed or
denied with zero core contact; HEAD rows forward once with core
re-judging; OIDC and GitHub stay on their own landed lanes), the
redirect-unfollowed pin (dispatchClient ErrUseLastResponse, proved by
a Location that points back at the stub with exactly one hit), zero
dest retries with the unstable-core matrix (500, retryable and
permanent 503s, dead core, pre-cancel and mid-flight cancel — exactly
one upstream attempt each, including POST ACS on every shape; start
and ACS are census mutations, so one forward is the whole
exactly-once story dest-side), response-envelope parity vs the legacy
relay (byte-identical status, headers, body across the outcome matrix
in both negotiated shapes, including the metadata content type and
the ACS form echo), duplicate deliveries no worse than direct core (2
hits, matching landed statuses), the disabled-provider row (core
conditionally mounts only `if a.SAML != nil` — the relay carries
core's real unmounted 404, synthesizing nothing), body-opacity proof
(near-1MiB and over-1MiB ACS bodies forward byte-intact with one
attempt each and core's answer relayed unmodified; grep plus source
tests prove zero form/body reads dest-side; telemetry capture proves
no `RelayState`, `SAMLResponse`, `Location`, `Set-Cookie`, or
`RequestURI` in logs, metrics, or spans), key-material absence (the
no-core-import proof plus the negative grep over the decision-3
symbols), no-core-import, the contract with revert-to-relay rollback,
and the satellite-owned clone-latency gate (`docs/d9-latency.md`:
five legs 0.92–1.00x vs the 1.50 budget, the ACS leg against a
same-bytes relay POST). Scope honesty: tier-C dispatch only — not
termination, not removal readiness. The provider-material store,
replay reservation, session mint, and finalize/audit stack stay open
termination obligations for a later lane. Termination stays last on
D0-proven transport only. The D9 wave is complete with this lane.

## Record-only pins (O2/O6)

The O2 audit-append pin (`AuditAppendAdmission v0` in
`contracts/audit-append-admission.md`) and the five O6 read pins
(`SignInPolicyReadAdmission v0`,
`contracts/sign-in-policy-read-admission.md`;
`MetricsReadAdmission v0`, `contracts/metrics-read-admission.md`;
`SemanticSearchReadAdmission v0`,
`contracts/semantic-search-read-admission.md`;
`TerraformDriftReadAdmission v0`,
`contracts/terraform-drift-read-admission.md`; widened
`IaCReadAdmission v0`, `contracts/iac-read-admission.md`) are
recorded byte-identical at `14a5bf8a52` with no wiring, no behavior
change, and no route moves: every leg stays on its current relay or
tier-C lane until its termination lane lands with arbiter review.

## What stays in the core

Fact admission, canonical identity, projection, retraction, recovery,
graph truth, queue and lease state, schema bootstrap, and one-shot
repair. `wireAPI`-mounted families keep their keyring and resolver
composition documented as required construction inputs, not hidden
globals.

## Compatibility policy

`eshu-api` supports the exact pinned core contract versions in
`docs/provenance.md` (`CoreReadContractsVersion v0`,
`SuppressionAdmissionContractVersion v0`,
`TerminalFailureAdmissionContractVersion v0`,
`ReplayLedgerAdmissionContractVersion v0`,
`SignInPolicyAdmissionContractVersion v0`,
`RetirementAdmissionContractVersion v0`,
`AdminIdentityAdmissionContractVersion v0`,
`SetupAdmissionContractVersion v0`,
`StatusReadAdmissionContractVersion v0`,
`SupplyChainReadAdmissionContractVersion v0`,
`SessionResolverAdmissionContractVersion v0`,
`BrowserSessionAdmissionContractVersion v0`,
`LocalIdentityAdmissionContractVersion v0`,
`APITokenAdmissionContractVersion v0`). A breaking change to
any named port fails the frozen-set test next to that port in the source
(`TestCoreReadContractsVersioned`,
`TestSuppressionAdmissionContractIsVersioned`,
`TestReadStoreContractIsVersioned`,
`TestTerminalFailureAdmissionContractIsVersioned`,
`TestReplayLedgerAdmissionContractIsVersioned`,
`TestSignInPolicyAdmissionContractIsVersioned`,
`TestRetirementAdmissionContractIsVersioned`,
`TestAdminIdentityAdmissionContractIsVersioned`,
`TestSetupAdmissionContractIsVersioned`) until the frozen list is
deliberately updated; the version bump rides alongside as manual
convention and this repository records the new pin before consuming it.
No shared-table shortcut is ever a compatible change.

## Open proofs (NOT_CHECKED)

Representative and hostile fixtures, auth and authz, pagination,
truth and error envelopes, and OpenAPI agreement are proven per family
by the in-tree suite cited in review. Still required before any
cutover: extracted-path parity on QA EKS, concurrent
retry and duplicate-delivery behavior, ordering, cancellation, revoked
authority, stale claims, partial and ambiguous failures, resource
bounds, no-regression measurements, health and readiness signals,
credential handling, artifact provenance, supported version pairs,
independent build and release, rollback rehearsal, and independent
review. The in-tree implementation is removed after production
acceptance, not before.
