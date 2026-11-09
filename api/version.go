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
// contracts/admin-read-admission.md. Reads over this port terminate
// natively since the D3 lane (api/d3_wave.go); see
// contracts/d3-admin-reads.md.
const AdminReadAdmissionContractVersion = "v0"

// AdminIdentityReadAdmissionContractVersion names the versioned
// admission port for tenant-scoped admin identity reads. It must move
// in lockstep with the in-tree
// AdminIdentityReadAdmissionContractVersion; the vendored pin lives in
// contracts/admin-identity-admission.md. Reads over this port
// terminate natively since the D3 lane (api/d3_wave.go); see
// contracts/d3-admin-reads.md.
const AdminIdentityReadAdmissionContractVersion = "v0"

// GovernanceAuditReadAdmissionContractVersion names the versioned
// admission port for the governance-audit reads backing the
// tenant-admin audit links. It must move in lockstep with the in-tree
// GovernanceAuditReadAdmissionContractVersion; the vendored pin lives
// in contracts/governance-audit-read-admission.md. Reads over this
// port terminate natively since the D3 lane (api/d3_wave.go) for the
// tenant-admin audit links; the D6 lane reuses the pin for Status +
// SupplyChain reads. See contracts/d3-admin-reads.md.
const GovernanceAuditReadAdmissionContractVersion = "v0"

// TerminalFailureReadAdmissionContractVersion names the versioned
// admission port for the operator terminal-failure reads. It must move
// in lockstep with the in-tree
// TerminalFailureReadAdmissionContractVersion; the vendored pin lives
// in contracts/terminal-failure-admission.md. Reads over this port
// terminate natively since the D3 lane (api/d3_wave.go); see
// contracts/d3-admin-reads.md.
const TerminalFailureReadAdmissionContractVersion = "v0"

// DeltaUnionCompletenessReadAdmissionContractVersion names the
// versioned admission-contract port for the delta-union completeness
// read (GET /api/v0/admin/delta-union/completeness). It must move in
// lockstep with the in-tree
// DeltaUnionCompletenessReadAdmissionContractVersion; the vendored
// pin lives in contracts/admin-trio-read-admission.md. Reads over
// this port terminate natively since the Gap 02 trio lane
// (api/d3trio_wave.go); see contracts/d3-admin-reads.md.
const DeltaUnionCompletenessReadAdmissionContractVersion = "v0"

// ReindexProgressReadAdmissionContractVersion names the versioned
// admission-contract port for the reindex progress read (GET
// /api/v0/admin/reindex/progress). It must move in lockstep with the
// in-tree ReindexProgressReadAdmissionContractVersion; the vendored
// pin lives in contracts/admin-trio-read-admission.md. Reads over
// this port terminate natively since the Gap 02 trio lane
// (api/d3trio_wave.go); see contracts/d3-admin-reads.md.
const ReindexProgressReadAdmissionContractVersion = "v0"

// SharedProjectionTuningReportContractVersion names the versioned
// admission-contract shape for the static shared-projection tuning
// report (GET /api/v0/admin/shared-projection/tuning-report). It must
// move in lockstep with the in-tree
// SharedProjectionTuningReportContractVersion; the vendored pin lives
// in contracts/admin-trio-read-admission.md. Reads over this shape
// terminate natively since the Gap 02 trio lane (api/d3trio_wave.go);
// see contracts/d3-admin-reads.md.
const SharedProjectionTuningReportContractVersion = "v0"

// AuditAppendAdmissionContractVersion names the versioned
// admission-contract port for the audit-append port behind the 18
// tier-C audit-emitting routes. It must move in lockstep with the
// in-tree AuditAppendAdmissionContractVersion; the vendored pin lives
// in contracts/audit-append-admission.md. Record-only: no handler,
// route, or caller depends on it yet.
const AuditAppendAdmissionContractVersion = "v0"

// SignInPolicyReadAdmissionContractVersion names the versioned
// admission-contract port for the sign-in-policy reads. It must move
// in lockstep with the in-tree
// SignInPolicyReadAdmissionContractVersion; the vendored pin lives in
// contracts/sign-in-policy-read-admission.md. Record-only: no
// handler, route, or caller depends on it yet.
const SignInPolicyReadAdmissionContractVersion = "v0"

// MetricsReadAdmissionContractVersion names the versioned
// admission-contract port for the metrics time-series read. It must
// move in lockstep with the in-tree
// MetricsReadAdmissionContractVersion; the vendored pin lives in
// contracts/metrics-read-admission.md. Record-only: no handler,
// route, or caller depends on it yet.
const MetricsReadAdmissionContractVersion = "v0"

// SemanticSearchReadAdmissionContractVersion names the versioned
// admission-contract port for the semantic-search read. It must move
// in lockstep with the in-tree
// SemanticSearchReadAdmissionContractVersion; the vendored pin lives
// in contracts/semantic-search-read-admission.md. Record-only: no
// handler, route, or caller depends on it yet.
const SemanticSearchReadAdmissionContractVersion = "v0"

// TerraformDriftReadAdmissionContractVersion names the versioned
// admission-contract port for the Terraform drift findings read. It
// must move in lockstep with the in-tree
// TerraformDriftReadAdmissionContractVersion; the vendored pin lives
// in contracts/terraform-drift-read-admission.md. Record-only: no
// handler, route, or caller depends on it yet.
const TerraformDriftReadAdmissionContractVersion = "v0"

// IaCReadAdmissionContractVersion names the versioned
// admission-contract port for the IaC reads. It must move in lockstep
// with the in-tree IaCReadAdmissionContractVersion; the vendored pin
// lives in contracts/iac-read-admission.md. Record-only: no handler,
// route, or caller depends on it yet.
const IaCReadAdmissionContractVersion = "v0"
