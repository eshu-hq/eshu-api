// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package openapi

import (
	"net/http"
	"strings"

	"github.com/eshu-hq/eshu-api/openapi/paths/auth"
	"github.com/eshu-hq/eshu-api/openapi/paths/catalog"
	"github.com/eshu-hq/eshu-api/openapi/paths/cicd"
	"github.com/eshu-hq/eshu-api/openapi/paths/cli"
	"github.com/eshu-hq/eshu-api/openapi/paths/cloud"
	"github.com/eshu-hq/eshu-api/openapi/paths/evidence"
	"github.com/eshu-hq/eshu-api/openapi/paths/freshness"
	"github.com/eshu-hq/eshu-api/openapi/paths/infrastructure"
	"github.com/eshu-hq/eshu-api/openapi/paths/repository"
	"github.com/eshu-hq/eshu-api/openapi/paths/search"
	"github.com/eshu-hq/eshu-api/openapi/paths/service"
	"github.com/eshu-hq/eshu-api/openapi/paths/status"
	supplychain "github.com/eshu-hq/eshu-api/openapi/paths/supply/chain"
)

// SpecVersion names the native-spec contract: v0 serves exactly the
// families whose OpenAPI fragments have landed in this repository
// (sign-in-policy first, retirement second, terminal-failure third,
// admin-identity mutations fourth, admin-identity reads fifth,
// provider-config mutations sixth, admin-recovery seventh, suppression
// eighth, then the D2 Leaf B Native-now wave: capabilities, CLI
// support, ask, service intelligence, auth providers,
// provider-config reads, and the served-spec self entry, then the D3
// admin/identity/recovery reads wave: the admin query POSTs and the
// invitations list GET that shares its key with the relay-only create
// POST, then the D4 Native-next wave: CICD, Documentation, Infra,
// PackageRegistry, Repositories, Evidence, InvestigationWorkflows, and
// Playbooks, then the D6 Status + SupplyChain reads wave: the
// snapshot-pure Status GETs and the 17 SupplyChain read GETs (Compare,
// extra-port Status aspects, the impact packet, and the suppressions
// mutation stay relay), then the D7 admin-four tier-C clone lane: the
// backfill, dead-letter, reindex, and skip POSTs (dispatch only, not
// termination), then the D8 session-resolver tier-C clone lane: the
// browser-session lifecycle, session-list, profile, local-identity,
// API-token, and TOTP routes (dispatch only, not termination), then
// the D9 OIDC-login tier-C clone lane: the OIDC login start and
// callback GETs (dispatch only, not termination), then the D9
// GitHub-login tier-C clone lane: the GitHub login start and callback
// GETs (dispatch only, not termination).
// It advances as family lanes port their fragments; the served family
// set is pinned by the Spec tests.
const SpecVersion = "v0"

// specPrefix opens the served document: the protocol version, the
// destination identity, the served prefix mounts, and the tags the landed
// fragments use. Family path entries concatenate after the open "paths"
// object; the shared components block closes it (see Spec).
const specPrefix = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Eshu API",
    "description": "Native-dispatch satellite surface. This document serves exactly the families whose OpenAPI fragments have landed in this repository (sign-in-policy, retirement, terminal-failure, admin-identity mutations, admin-identity reads, provider-config mutations, admin-recovery, suppression, plus the D2 Leaf B Native-now wave: capabilities, CLI support, ask, service intelligence, auth providers, provider-config reads, and the served-spec self entry, plus the D3 admin/identity/recovery reads wave: the admin query POSTs and the invitations list GET, plus the D4 Native-next wave: CICD, Documentation, Infra, PackageRegistry, Repositories, Evidence, InvestigationWorkflows, and Playbooks, plus the D6 Status + SupplyChain reads wave: the snapshot-pure Status GETs and the 17 SupplyChain read GETs, plus the D7 admin-four tier-C clone lane: the backfill, dead-letter, reindex, and skip POSTs (dispatch only, not termination), plus the D8 session-resolver tier-C clone lane: the browser-session lifecycle, session-list, profile, local-identity, API-token, and TOTP routes (dispatch only, not termination), plus the D9 OIDC-login tier-C clone lane: the OIDC login start and callback GETs (dispatch only, not termination), plus the D9 GitHub-login tier-C clone lane: the GitHub login start and callback GETs (dispatch only, not termination)); every other family is still relayed from core until its lane ports its fragment.",
    "version": "` + SpecVersion + `",
    "contact": {
      "name": "Eshu"
    }
  },
  "servers": [
    {
      "url": "/api/v0",
      "description": "API v0 prefix"
    },
    {
      "url": "/api/v1",
      "description": "API v1 prefix"
    }
  ],
  "tags": [
    {"name": "auth", "description": "Browser session and CSRF-safe dashboard authentication routes"},
    {"name": "admin", "description": "Operator administration routes"}
  ],
  "paths": {
`

// familyPaths renders the landed path fragments and closes the open
// "paths" object. Every fragment file ends its last entry with a
// separator comma for the next fragment in concatenation order (the core
// convention), so only the final fragment in landing order has its
// trailing comma trimmed before the close. A fragment that stops ending
// in a comma breaks the served JSON and the Spec tests fail closed.
// The D2 Leaf B wave appends the Native-now fragments after the
// suppression lane in wave-table order (capabilities, CLI support, ask,
// service intelligence, auth providers, provider-config reads, then the
// served-spec self entry last); the D4 Native-next wave appends after
// the provider-config reads in ruling family order (CICD,
// Documentation, Infra, PackageRegistry, Repositories, Evidence,
// InvestigationWorkflows, Playbooks); the D6 Status + SupplyChain
// reads wave appends after the D4 entries in wave-table order (Status
// reads, then SupplyChain reads); the D7 admin-four tier-C clone lane
// appends after the D6 entries (AdminD7); the D8 session-resolver tier-C
// clone lane appends after the D7 entries (BrowserSessions, SessionList,
// AuthProfile, AdminInvitations-carrying-the-invitations-POST,
// LocalIdentity, Tokens, TOTP); the D9 OIDC-login tier-C clone lane
// appends after the D8 entries (OIDCLogin); the D9 GitHub-login tier-C
// clone lane appends after the OIDC entry (GitHubLogin), and the
// served-spec self entry keeps the trim last.
func familyPaths() string {
	return strings.TrimSpace(auth.SignInPolicy) + "\n" +
		strings.TrimSpace(status.RepositoryRetirement) + "\n" +
		strings.TrimSpace(status.CollectorTerminalFailureAcknowledgement) + "\n" +
		strings.TrimSpace(auth.AdminMutations) + "\n" +
		strings.TrimSpace(auth.AdminReads) + "\n" +
		strings.TrimSpace(auth.AdminInvitations) + "\n" +
		strings.TrimSpace(auth.AdminProviderConfigMutations) + "\n" +
		strings.TrimSpace(status.AdminRecovery) + "\n" +
		strings.TrimSpace(status.AdminReads) + "\n" +
		strings.TrimSpace(supplychain.SuppressionMutation) + "\n" +
		strings.TrimSpace(catalog.Capabilities) + "\n" +
		strings.TrimSpace(cli.Score) + "\n" +
		strings.TrimSpace(cli.Render) + "\n" +
		strings.TrimSpace(cli.Capture) + "\n" +
		strings.TrimSpace(cli.Validate) + "\n" +
		strings.TrimSpace(cli.Compose) + "\n" +
		strings.TrimSpace(search.Ask) + "\n" +
		strings.TrimSpace(service.IntelligenceReport) + "\n" +
		strings.TrimSpace(auth.AuthProviders) + "\n" +
		strings.TrimSpace(auth.AdminProviderConfigReads) + "\n" +
		strings.TrimSpace(cicd.Routes) + "\n" +
		strings.TrimSpace(cicd.RunCorrelationAggregate) + "\n" +
		strings.TrimSpace(evidence.Documentation) + "\n" +
		strings.TrimSpace(evidence.DocumentationFindingAggregate) + "\n" +
		strings.TrimSpace(cloud.Routes) + "\n" +
		strings.TrimSpace(infrastructure.Routes) + "\n" +
		strings.TrimSpace(infrastructure.ResourceAggregate) + "\n" +
		strings.TrimSpace(repository.PackageRegistry) + "\n" +
		strings.TrimSpace(repository.PackageRegistryAggregate) + "\n" +
		strings.TrimSpace(repository.Branches) + "\n" +
		strings.TrimSpace(repository.StatsAndCoverage) + "\n" +
		strings.TrimSpace(repository.Freshness) + "\n" +
		strings.TrimSpace(repository.RepositoryRoutes) + "\n" +
		strings.TrimSpace(evidence.EvidenceRoutes) + "\n" +
		strings.TrimSpace(evidence.Bundle) + "\n" +
		strings.TrimSpace(evidence.DeployableUnitPacket) + "\n" +
		strings.TrimSpace(evidence.InvestigationWorkflows) + "\n" +
		strings.TrimSpace(catalog.Playbooks) + "\n" +
		strings.TrimSpace(status.StatusReads) + "\n" +
		strings.TrimSpace(status.CollectorReadiness) + "\n" +
		strings.TrimSpace(freshness.Causality) + "\n" +
		strings.TrimSpace(status.OperatorControlPlane) + "\n" +
		strings.TrimSpace(status.Semantic) + "\n" +
		strings.TrimSpace(supplychain.AdvisoryCatalog) + "\n" +
		strings.TrimSpace(supplychain.AdvisoryEvidence) + "\n" +
		strings.TrimSpace(supplychain.ContainerImages) + "\n" +
		strings.TrimSpace(supplychain.ContainerImageIdentityAggregate) + "\n" +
		strings.TrimSpace(supplychain.ImpactFindings) + "\n" +
		strings.TrimSpace(supplychain.ImpactExplain) + "\n" +
		strings.TrimSpace(supplychain.ImpactAggregate) + "\n" +
		strings.TrimSpace(supplychain.SBOMAttestations) + "\n" +
		strings.TrimSpace(supplychain.SBOMAttestationAttachmentAggregate) + "\n" +
		strings.TrimSpace(supplychain.SecurityAlerts) + "\n" +
		strings.TrimSpace(supplychain.SecurityAlertAggregate) + "\n" +
		strings.TrimSpace(supplychain.VulnerabilityScannerContract) + "\n" +
		strings.TrimSpace(status.AdminD7) + "\n" +
		strings.TrimSpace(auth.BrowserSessions) + "\n" +
		strings.TrimSpace(auth.SessionList) + "\n" +
		strings.TrimSpace(auth.AuthProfile) + "\n" +
		strings.TrimSpace(auth.LocalIdentity) + "\n" +
		strings.TrimSpace(auth.Tokens) + "\n" +
		strings.TrimSpace(auth.TOTP) + "\n" +
		strings.TrimSpace(auth.OIDCLogin) + "\n" +
		strings.TrimSpace(auth.GitHubLogin) + "\n" +
		strings.TrimSuffix(strings.TrimSpace(status.OpenAPISpec), ",") + "\n"
}

// Spec returns the dest-assembled OpenAPI 3.0 document for the landed
// families: the spec prefix, every ported family path fragment in landing
// order, then the minimal shared components block. The concatenation
// order is the compatibility surface the destination owns.
func Spec() string {
	return specPrefix +
		familyPaths() +
		"  },\n" +
		components +
		"\n"
}

// ServeSpec writes the assembled specification as JSON. Headers match the
// core mount at the provenance pin (go/internal/query/openapi/spec.go):
// JSON with charset, open CORS.
func ServeSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(Spec()))
}
