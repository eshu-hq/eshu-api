// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// pinnedVersion reads a version pin from a vendored contract document.
// It finds the declaration line for symbol (of the form SYMBOL = "vX")
// and returns the quoted value. It fails the test when the file or the
// line is missing, so a deleted or relocated pin cannot pass silently.
func pinnedVersion(t *testing.T, docPath, symbol string) string {
	t.Helper()

	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read vendored pin %s: %v", docPath, err)
	}
	for line := range strings.Lines(string(raw)) {
		idx := strings.Index(line, symbol+" = \"")
		if idx < 0 {
			continue
		}
		rest := line[idx+len(symbol+" = \""):]
		end := strings.Index(rest, "\"")
		if end < 0 {
			t.Fatalf("pin line for %s in %s has no closing quote: %q", symbol, docPath, line)
		}
		return rest[:end]
	}
	t.Fatalf("no %s pin line in %s", symbol, docPath)
	return ""
}

// tablePinnedVersion reads a version pin from a Markdown table row of
// the form | `SYMBOL` | `vX` | (docs/provenance.md). It fails the test
// when the row is missing, so a dropped pin row cannot pass silently.
func tablePinnedVersion(t *testing.T, docPath, symbol string) string {
	t.Helper()

	raw, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read vendored pin %s: %v", docPath, err)
	}
	for line := range strings.Lines(string(raw)) {
		if !strings.HasPrefix(strings.TrimSpace(line), "|") {
			continue
		}
		if !strings.Contains(line, "`"+symbol+"`") {
			continue
		}
		rest := line[strings.Index(line, "`"+symbol+"`")+len(symbol)+2:]
		start := strings.Index(rest, "`")
		if start < 0 {
			continue
		}
		rest = rest[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			t.Fatalf("pin row for %s in %s has no closing backtick: %q", symbol, docPath, line)
		}
		return rest[:end]
	}
	t.Fatalf("no %s pin row in %s", symbol, docPath)
	return ""
}

// TestContractVersionsArePinned locks the skeleton to the contract
// versions vendored under contracts/. Each assertion reads the pin from
// the document and compares it to the package constant, so the test
// fails in both drift directions: a bumped document with a stale
// constant, and a bumped constant with a stale document. Consuming a
// newer core contract requires recording the new pin in
// docs/provenance.md first and bumping the constant in the same diff.
func TestContractVersionsArePinned(t *testing.T) {
	t.Parallel()

	if got := pinnedVersion(t, "../contracts/core-read-contracts.md", "CoreReadContractsVersion"); got != CoreReadContractsVersion {
		t.Errorf("contracts/core-read-contracts.md pins %q, constant is %q: move them in the same diff", got, CoreReadContractsVersion)
	}
	if got := pinnedVersion(t, "../contracts/suppression-admission.md", "SuppressionAdmissionContractVersion"); got != SuppressionAdmissionContractVersion {
		t.Errorf("contracts/suppression-admission.md pins %q, constant is %q: move them in the same diff", got, SuppressionAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/terminal-failure-admission.md", "TerminalFailureAdmissionContractVersion"); got != TerminalFailureAdmissionContractVersion {
		t.Errorf("contracts/terminal-failure-admission.md pins %q, constant is %q: move them in the same diff", got, TerminalFailureAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/replay-ledger-admission.md", "ReplayLedgerAdmissionContractVersion"); got != ReplayLedgerAdmissionContractVersion {
		t.Errorf("contracts/replay-ledger-admission.md pins %q, constant is %q: move them in the same diff", got, ReplayLedgerAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/sign-in-admission.md", "SignInPolicyAdmissionContractVersion"); got != SignInPolicyAdmissionContractVersion {
		t.Errorf("contracts/sign-in-admission.md pins %q, constant is %q: move them in the same diff", got, SignInPolicyAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/retirement-admission.md", "RetirementAdmissionContractVersion"); got != RetirementAdmissionContractVersion {
		t.Errorf("contracts/retirement-admission.md pins %q, constant is %q: move them in the same diff", got, RetirementAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-identity-admission.md", "AdminIdentityAdmissionContractVersion"); got != AdminIdentityAdmissionContractVersion {
		t.Errorf("contracts/admin-identity-admission.md pins %q, constant is %q: move them in the same diff", got, AdminIdentityAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/setup-admission.md", "SetupAdmissionContractVersion"); got != SetupAdmissionContractVersion {
		t.Errorf("contracts/setup-admission.md pins %q, constant is %q: move them in the same diff", got, SetupAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/provider-config-admission.md", "ProviderConfigMutationAdmissionContractVersion"); got != ProviderConfigMutationAdmissionContractVersion {
		t.Errorf("contracts/provider-config-admission.md pins %q, constant is %q: move them in the same diff", got, ProviderConfigMutationAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/provider-config-admission.md", "ProviderConfigReadAdmissionContractVersion"); got != ProviderConfigReadAdmissionContractVersion {
		t.Errorf("contracts/provider-config-admission.md pins %q, constant is %q: move them in the same diff", got, ProviderConfigReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/oidc-login-admission.md", "OIDCLoginAdmissionContractVersion"); got != OIDCLoginAdmissionContractVersion {
		t.Errorf("contracts/oidc-login-admission.md pins %q, constant is %q: move them in the same diff", got, OIDCLoginAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/github-login-admission.md", "GitHubLoginAdmissionContractVersion"); got != GitHubLoginAdmissionContractVersion {
		t.Errorf("contracts/github-login-admission.md pins %q, constant is %q: move them in the same diff", got, GitHubLoginAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/saml-login-admission.md", "SAMLLoginAdmissionContractVersion"); got != SAMLLoginAdmissionContractVersion {
		t.Errorf("contracts/saml-login-admission.md pins %q, constant is %q: move them in the same diff", got, SAMLLoginAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/recovery-admission.md", "RecoveryAdmissionContractVersion"); got != RecoveryAdmissionContractVersion {
		t.Errorf("contracts/recovery-admission.md pins %q, constant is %q: move them in the same diff", got, RecoveryAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-read-admission.md", "AdminReadAdmissionContractVersion"); got != AdminReadAdmissionContractVersion {
		t.Errorf("contracts/admin-read-admission.md pins %q, constant is %q: move them in the same diff", got, AdminReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-identity-admission.md", "AdminIdentityReadAdmissionContractVersion"); got != AdminIdentityReadAdmissionContractVersion {
		t.Errorf("contracts/admin-identity-admission.md pins %q, constant is %q: move them in the same diff", got, AdminIdentityReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/governance-audit-read-admission.md", "GovernanceAuditReadAdmissionContractVersion"); got != GovernanceAuditReadAdmissionContractVersion {
		t.Errorf("contracts/governance-audit-read-admission.md pins %q, constant is %q: move them in the same diff", got, GovernanceAuditReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/terminal-failure-admission.md", "TerminalFailureReadAdmissionContractVersion"); got != TerminalFailureReadAdmissionContractVersion {
		t.Errorf("contracts/terminal-failure-admission.md pins %q, constant is %q: move them in the same diff", got, TerminalFailureReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-trio-read-admission.md", "DeltaUnionCompletenessReadAdmissionContractVersion"); got != DeltaUnionCompletenessReadAdmissionContractVersion {
		t.Errorf("contracts/admin-trio-read-admission.md pins %q, constant is %q: move them in the same diff", got, DeltaUnionCompletenessReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-trio-read-admission.md", "ReindexProgressReadAdmissionContractVersion"); got != ReindexProgressReadAdmissionContractVersion {
		t.Errorf("contracts/admin-trio-read-admission.md pins %q, constant is %q: move them in the same diff", got, ReindexProgressReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-trio-read-admission.md", "SharedProjectionTuningReportContractVersion"); got != SharedProjectionTuningReportContractVersion {
		t.Errorf("contracts/admin-trio-read-admission.md pins %q, constant is %q: move them in the same diff", got, SharedProjectionTuningReportContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/audit-append-admission.md", "AuditAppendAdmissionContractVersion"); got != AuditAppendAdmissionContractVersion {
		t.Errorf("contracts/audit-append-admission.md pins %q, constant is %q: move them in the same diff", got, AuditAppendAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/sign-in-policy-read-admission.md", "SignInPolicyReadAdmissionContractVersion"); got != SignInPolicyReadAdmissionContractVersion {
		t.Errorf("contracts/sign-in-policy-read-admission.md pins %q, constant is %q: move them in the same diff", got, SignInPolicyReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/metrics-read-admission.md", "MetricsReadAdmissionContractVersion"); got != MetricsReadAdmissionContractVersion {
		t.Errorf("contracts/metrics-read-admission.md pins %q, constant is %q: move them in the same diff", got, MetricsReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/semantic-search-read-admission.md", "SemanticSearchReadAdmissionContractVersion"); got != SemanticSearchReadAdmissionContractVersion {
		t.Errorf("contracts/semantic-search-read-admission.md pins %q, constant is %q: move them in the same diff", got, SemanticSearchReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/terraform-drift-read-admission.md", "TerraformDriftReadAdmissionContractVersion"); got != TerraformDriftReadAdmissionContractVersion {
		t.Errorf("contracts/terraform-drift-read-admission.md pins %q, constant is %q: move them in the same diff", got, TerraformDriftReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/iac-read-admission.md", "IaCReadAdmissionContractVersion"); got != IaCReadAdmissionContractVersion {
		t.Errorf("contracts/iac-read-admission.md pins %q, constant is %q: move them in the same diff", got, IaCReadAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/retirement-admission.md", "RetirementAdmissionContractVersion"); got != RetirementAdmissionContractVersion {
		t.Errorf("contracts/retirement-admission.md pins %q, constant is %q: move them in the same diff", got, RetirementAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/admin-identity-admission.md", "AdminIdentityAdmissionContractVersion"); got != AdminIdentityAdmissionContractVersion {
		t.Errorf("contracts/admin-identity-admission.md pins %q, constant is %q: move them in the same diff", got, AdminIdentityAdmissionContractVersion)
	}
	if got := pinnedVersion(t, "../contracts/setup-admission.md", "SetupAdmissionContractVersion"); got != SetupAdmissionContractVersion {
		t.Errorf("contracts/setup-admission.md pins %q, constant is %q: move them in the same diff", got, SetupAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "CoreReadContractsVersion"); got != CoreReadContractsVersion {
		t.Errorf("docs/provenance.md pins core %q, constant is %q: record the new pin before consuming it", got, CoreReadContractsVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SuppressionAdmissionContractVersion"); got != SuppressionAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins admission %q, constant is %q: record the new pin before consuming it", got, SuppressionAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "TerminalFailureAdmissionContractVersion"); got != TerminalFailureAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins terminal-failure %q, constant is %q: record the new pin before consuming it", got, TerminalFailureAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "ReplayLedgerAdmissionContractVersion"); got != ReplayLedgerAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins replay-ledger %q, constant is %q: record the new pin before consuming it", got, ReplayLedgerAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SignInPolicyAdmissionContractVersion"); got != SignInPolicyAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins sign-in %q, constant is %q: record the new pin before consuming it", got, SignInPolicyAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "RetirementAdmissionContractVersion"); got != RetirementAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins retirement %q, constant is %q: record the new pin before consuming it", got, RetirementAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "AdminIdentityAdmissionContractVersion"); got != AdminIdentityAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins admin-identity %q, constant is %q: record the new pin before consuming it", got, AdminIdentityAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SetupAdmissionContractVersion"); got != SetupAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins setup %q, constant is %q: record the new pin before consuming it", got, SetupAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "ProviderConfigMutationAdmissionContractVersion"); got != ProviderConfigMutationAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins provider-config mutation %q, constant is %q: record the new pin before consuming it", got, ProviderConfigMutationAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "ProviderConfigReadAdmissionContractVersion"); got != ProviderConfigReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins provider-config read %q, constant is %q: record the new pin before consuming it", got, ProviderConfigReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "OIDCLoginAdmissionContractVersion"); got != OIDCLoginAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins OIDC login %q, constant is %q: record the new pin before consuming it", got, OIDCLoginAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "GitHubLoginAdmissionContractVersion"); got != GitHubLoginAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins GitHub login %q, constant is %q: record the new pin before consuming it", got, GitHubLoginAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SAMLLoginAdmissionContractVersion"); got != SAMLLoginAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins SAML login %q, constant is %q: record the new pin before consuming it", got, SAMLLoginAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "RecoveryAdmissionContractVersion"); got != RecoveryAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins recovery %q, constant is %q: record the new pin before consuming it", got, RecoveryAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "AdminReadAdmissionContractVersion"); got != AdminReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins admin reads %q, constant is %q: record the new pin before consuming it", got, AdminReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "AdminIdentityReadAdmissionContractVersion"); got != AdminIdentityReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins admin-identity reads %q, constant is %q: record the new pin before consuming it", got, AdminIdentityReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "GovernanceAuditReadAdmissionContractVersion"); got != GovernanceAuditReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins governance-audit reads %q, constant is %q: record the new pin before consuming it", got, GovernanceAuditReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "TerminalFailureReadAdmissionContractVersion"); got != TerminalFailureReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins terminal-failure reads %q, constant is %q: record the new pin before consuming it", got, TerminalFailureReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "DeltaUnionCompletenessReadAdmissionContractVersion"); got != DeltaUnionCompletenessReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins delta-union completeness reads %q, constant is %q: record the new pin before consuming it", got, DeltaUnionCompletenessReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "ReindexProgressReadAdmissionContractVersion"); got != ReindexProgressReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins reindex progress reads %q, constant is %q: record the new pin before consuming it", got, ReindexProgressReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SharedProjectionTuningReportContractVersion"); got != SharedProjectionTuningReportContractVersion {
		t.Errorf("docs/provenance.md pins shared-projection tuning reads %q, constant is %q: record the new pin before consuming it", got, SharedProjectionTuningReportContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "AuditAppendAdmissionContractVersion"); got != AuditAppendAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins audit-append %q, constant is %q: record the new pin before consuming it", got, AuditAppendAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SignInPolicyReadAdmissionContractVersion"); got != SignInPolicyReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins sign-in-policy reads %q, constant is %q: record the new pin before consuming it", got, SignInPolicyReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "MetricsReadAdmissionContractVersion"); got != MetricsReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins metrics reads %q, constant is %q: record the new pin before consuming it", got, MetricsReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "SemanticSearchReadAdmissionContractVersion"); got != SemanticSearchReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins semantic-search reads %q, constant is %q: record the new pin before consuming it", got, SemanticSearchReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "TerraformDriftReadAdmissionContractVersion"); got != TerraformDriftReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins terraform-drift reads %q, constant is %q: record the new pin before consuming it", got, TerraformDriftReadAdmissionContractVersion)
	}
	if got := tablePinnedVersion(t, "../docs/provenance.md", "IaCReadAdmissionContractVersion"); got != IaCReadAdmissionContractVersion {
		t.Errorf("docs/provenance.md pins IaC reads %q, constant is %q: record the new pin before consuming it", got, IaCReadAdmissionContractVersion)
	}
}

// TestHealthReportsPinnedContractVersions pins the /health version
// keys to the package constants: every served admission version,
// including the three newest ports, must match its constant, so a
// bumped constant with a stale handler (or vice versa) fails here.
func TestHealthReportsPinnedContractVersions(t *testing.T) {
	t.Parallel()

	srv := NewServer("https://core:8443", skeletonTestKey)
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler(DefaultV0Sunset).ServeHTTP(rec, req)

	health := decodeMap(t, rec)
	want := map[string]string{
		"core_read_contracts":                               CoreReadContractsVersion,
		"admission_contracts":                               SuppressionAdmissionContractVersion,
		"suppression_admission_contracts":                   SuppressionAdmissionContractVersion,
		"terminal_failure_admission_contracts":              TerminalFailureAdmissionContractVersion,
		"replay_ledger_admission_contracts":                 ReplayLedgerAdmissionContractVersion,
		"sign_in_policy_admission_contracts":                SignInPolicyAdmissionContractVersion,
		"retirement_admission_contracts":                    RetirementAdmissionContractVersion,
		"admin_identity_admission_contracts":                AdminIdentityAdmissionContractVersion,
		"setup_admission_contracts":                         SetupAdmissionContractVersion,
		"provider_config_mutation_admission_contracts":      ProviderConfigMutationAdmissionContractVersion,
		"provider_config_read_admission_contracts":          ProviderConfigReadAdmissionContractVersion,
		"oidc_login_admission_contracts":                    OIDCLoginAdmissionContractVersion,
		"github_login_admission_contracts":                  GitHubLoginAdmissionContractVersion,
		"saml_login_admission_contracts":                    SAMLLoginAdmissionContractVersion,
		"recovery_admission_contracts":                      RecoveryAdmissionContractVersion,
		"admin_read_admission_contracts":                    AdminReadAdmissionContractVersion,
		"admin_identity_read_admission_contracts":           AdminIdentityReadAdmissionContractVersion,
		"governance_audit_read_admission_contracts":         GovernanceAuditReadAdmissionContractVersion,
		"terminal_failure_read_admission_contracts":         TerminalFailureReadAdmissionContractVersion,
		"delta_union_completeness_read_admission_contracts": DeltaUnionCompletenessReadAdmissionContractVersion,
		"reindex_progress_read_admission_contracts":         ReindexProgressReadAdmissionContractVersion,
		"shared_projection_tuning_report_contracts":         SharedProjectionTuningReportContractVersion,
	}
	for key, wantVersion := range want {
		if health[key] != wantVersion {
			t.Errorf("/health %q = %q, want pinned %q", key, health[key], wantVersion)
		}
	}
}
