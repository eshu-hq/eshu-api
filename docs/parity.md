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
  across those four pins, minus the families D2 already terminated
  and the 3 unpinned Admin GETs) terminates natively per
  `contracts/d3-admin-reads.md` and the rest stay on the generic
  relay until their D waves land.
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
  subset) — per-family fragment moves now land in their family PRs per
  satellite-design Decision 5(c) on eshu-hq/eshu#9669.

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
`SetupAdmissionContractVersion v0`). A breaking change to any
named port fails the frozen-set test next to that port in the source
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
