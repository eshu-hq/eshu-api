// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

// CoreReadContractsVersion names the core-owned read-contract set this
// server consumes (Refs eshu-hq/eshu#8872): the GraphQuery and ContentStore
// ports plus the Postgres ReadStore surface. It must move in lockstep with
// querycontract.CoreReadContractsVersion in eshu; the vendored pin lives
// in contracts/core-read-contracts.md and the source pin in
// docs/provenance.md. A breaking change to any named port fails the
// version test until this constant moves in the same diff.
const CoreReadContractsVersion = "v0"

// SuppressionAdmissionContractVersion names the versioned admission port
// for the single bounded in-API mutation (the operator suppression
// upsert). It must move in lockstep with the in-tree
// SuppressionAdmissionContractVersion; the vendored pin lives in
// contracts/suppression-admission.md. Route dispatch over this port lands
// with core dispatch, not in the skeleton.
const SuppressionAdmissionContractVersion = "v0"

// TerminalFailureAdmissionContractVersion names the versioned admission
// port for the operator terminal-failure acknowledgement mutation. It
// must move in lockstep with the in-tree
// TerminalFailureAdmissionContractVersion; the vendored pin lives in
// contracts/terminal-failure-admission.md. Route dispatch over this port
// lands with core dispatch, not in the skeleton.
const TerminalFailureAdmissionContractVersion = "v0"

// ReplayLedgerAdmissionContractVersion names the versioned admission port
// for the shared replay idempotency ledger. It must move in lockstep
// with the in-tree ReplayLedgerAdmissionContractVersion; the vendored
// pin lives in contracts/replay-ledger-admission.md. Ledger calls over
// this port land with core dispatch, not in the skeleton.
const ReplayLedgerAdmissionContractVersion = "v0"

// SignInPolicyAdmissionContractVersion names the versioned admission port
// for the admin sign-in policy mutation. It must move in lockstep with
// the in-tree SignInPolicyAdmissionContractVersion; the vendored pin
// lives in contracts/sign-in-admission.md. Route dispatch over this port
// lands with core dispatch, not in the skeleton.
const SignInPolicyAdmissionContractVersion = "v0"

// RetirementAdmissionContractVersion names the versioned admission port
// for the operator repository retirement trigger. It must move in
// lockstep with the in-tree RetirementAdmissionContractVersion; the
// vendored pin lives in contracts/retirement-admission.md. Route
// dispatch over this port lands with core dispatch, not in the
// skeleton.
const RetirementAdmissionContractVersion = "v0"

// AdminIdentityAdmissionContractVersion names the versioned admission
// port for tenant-scoped admin identity mutations. It must move in
// lockstep with the in-tree AdminIdentityAdmissionContractVersion; the
// vendored pin lives in contracts/admin-identity-admission.md. Route
// dispatch over this port lands with core dispatch, not in the
// skeleton.
const AdminIdentityAdmissionContractVersion = "v0"

// SetupAdmissionContractVersion names the versioned admission port for
// the first-run setup wizard mutations. It must move in lockstep with
// the in-tree SetupAdmissionContractVersion; the vendored pin lives in
// contracts/setup-admission.md. Route dispatch over this port lands
// with core dispatch, not in the skeleton.
const SetupAdmissionContractVersion = "v0"

// ProviderConfigMutationAdmissionContractVersion names the versioned
// admission port for admin provider-config mutations. It must move in
// lockstep with the in-tree
// ProviderConfigMutationAdmissionContractVersion; the vendored pin lives
// in contracts/provider-config-admission.md. Route dispatch over this
// port lands with core dispatch, not in the skeleton.
const ProviderConfigMutationAdmissionContractVersion = "v0"

// ProviderConfigReadAdmissionContractVersion names the versioned
// admission port for the reads backing the provider-config enable-time
// guard. It must move in lockstep with the in-tree
// ProviderConfigReadAdmissionContractVersion; the vendored pin lives in
// contracts/provider-config-admission.md. Reads over this port land with
// core dispatch, not in the skeleton.
const ProviderConfigReadAdmissionContractVersion = "v0"

// OIDCLoginAdmissionContractVersion names the versioned admission port
// for the OIDC login flow. It must move in lockstep with the in-tree
// OIDCLoginAdmissionContractVersion; the vendored pin lives in
// contracts/oidc-login-admission.md. Route dispatch over this port lands
// with core dispatch, not in the skeleton.
const OIDCLoginAdmissionContractVersion = "v0"

// GitHubLoginAdmissionContractVersion names the versioned admission port
// for the GitHub login flow. It must move in lockstep with the in-tree
// GitHubLoginAdmissionContractVersion; the vendored pin lives in
// contracts/github-login-admission.md. Route dispatch over this port
// lands with core dispatch, not in the skeleton.
const GitHubLoginAdmissionContractVersion = "v0"

// SAMLLoginAdmissionContractVersion names the versioned admission port
// for the SAML login flow. It must move in lockstep with the in-tree
// SAMLLoginAdmissionContractVersion; the vendored pin lives in
// contracts/saml-login-admission.md. Route dispatch over this port
// lands with core dispatch, not in the skeleton.
const SAMLLoginAdmissionContractVersion = "v0"

// RecoveryAdmissionContractVersion names the versioned admission port
// for the core recovery operations (refinalize and replay-failed) the
// admin recovery mutations admit through. It must move in lockstep
// with the in-tree RecoveryAdmissionContractVersion; the vendored pin
// lives in contracts/recovery-admission.md. Route dispatch over this
// port lands with core dispatch, not in the skeleton. The replay,
// reopen, and recover-generations routes share the already-pinned
// ReplayLedgerAdmissionContractVersion for their claim/complete pair.
const RecoveryAdmissionContractVersion = "v0"

// AdminReadAdmissionContractVersion names the versioned admission port
// for the operator admin reads the admin endpoints admit through
// ReadStore. It must move in lockstep with the in-tree
// AdminReadAdmissionContractVersion; the vendored pin lives in
// contracts/admin-read-admission.md. Reads over this port stay on the
// generic relay until the D3 lane lands; this record moves no route.
const AdminReadAdmissionContractVersion = "v0"

// AdminIdentityReadAdmissionContractVersion names the versioned
// admission port for tenant-scoped admin identity reads. It must move
// in lockstep with the in-tree
// AdminIdentityReadAdmissionContractVersion; the vendored pin lives in
// contracts/admin-identity-admission.md. Reads over this port stay on
// the generic relay until the D3 lane lands; this record moves no
// route.
const AdminIdentityReadAdmissionContractVersion = "v0"

// GovernanceAuditReadAdmissionContractVersion names the versioned
// admission port for the governance-audit reads backing the
// tenant-admin audit links. It must move in lockstep with the in-tree
// GovernanceAuditReadAdmissionContractVersion; the vendored pin lives
// in contracts/governance-audit-read-admission.md. Reads over this
// port stay on the generic relay until the D3 and D6 lanes land; this
// record moves no route.
const GovernanceAuditReadAdmissionContractVersion = "v0"

// TerminalFailureReadAdmissionContractVersion names the versioned
// admission port for the operator terminal-failure reads. It must move
// in lockstep with the in-tree
// TerminalFailureReadAdmissionContractVersion; the vendored pin lives
// in contracts/terminal-failure-admission.md. Reads over this port
// stay on the generic relay until the D3 lane lands; this record moves
// no route.
const TerminalFailureReadAdmissionContractVersion = "v0"
