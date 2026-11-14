// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net"
	"net/http"
	"time"
)

// readyProbeTimeout bounds one core-reachability check behind
// /readyz. It stays far below the 30 s relay bound (dispatchTimeout):
// the probe answers whether the relay can open the core connection,
// not whether a full relay round-trip would finish, so a slow core
// still reads not-ready instead of wedging the scrape.
const readyProbeTimeout = 2 * time.Second

// Closed readiness vocabulary for the eshu_api_readiness_total
// metric: the result names the probe verdict, the reason names why a
// not-ready verdict fired. The failure reasons reuse the relay lane's
// closed set (see dispatch.go) so operators read one vocabulary;
// reasonNone marks the ready verdict, which has no failure.
const (
	readinessReady      = "ready"
	readinessNotReady   = "not_ready"
	readinessReasonNone = "none"
)

// readyNotReadyBody is the /readyz answer when the relay cannot serve
// traffic. It keeps the probe's service/probe/status key shape and
// adds the closed failure reason in details, matching the local-error
// details contract (see localErrorDetails): unconfigured when no core
// origin is set, otherwise the dial failure classified the way the
// relay lane classifies it.
type readyNotReadyBody struct {
	Service string            `json:"service"`
	Probe   string            `json:"probe"`
	Status  string            `json:"status"`
	Details map[string]string `json:"details,omitempty"`
}

// health reports the served contract versions and the intended core origin.
// It takes no credential and never discloses the API key. Every admission
// port has its own explicit key, named for its family.
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":                                            "ok",
		"service":                                           "eshu-api",
		"core_read_contracts":                               CoreReadContractsVersion,
		"suppression_admission_contracts":                   SuppressionAdmissionContractVersion,
		"terminal_failure_admission_contracts":              TerminalFailureAdmissionContractVersion,
		"replay_ledger_admission_contracts":                 ReplayLedgerAdmissionContractVersion,
		"recovery_admission_contracts":                      RecoveryAdmissionContractVersion,
		"sign_in_policy_admission_contracts":                SignInPolicyAdmissionContractVersion,
		"retirement_admission_contracts":                    RetirementAdmissionContractVersion,
		"admin_identity_admission_contracts":                AdminIdentityAdmissionContractVersion,
		"setup_admission_contracts":                         SetupAdmissionContractVersion,
		"provider_config_mutation_admission_contracts":      ProviderConfigMutationAdmissionContractVersion,
		"provider_config_read_admission_contracts":          ProviderConfigReadAdmissionContractVersion,
		"oidc_login_admission_contracts":                    OIDCLoginAdmissionContractVersion,
		"github_login_admission_contracts":                  GitHubLoginAdmissionContractVersion,
		"saml_login_admission_contracts":                    SAMLLoginAdmissionContractVersion,
		"admin_read_admission_contracts":                    AdminReadAdmissionContractVersion,
		"admin_identity_read_admission_contracts":           AdminIdentityReadAdmissionContractVersion,
		"governance_audit_read_admission_contracts":         GovernanceAuditReadAdmissionContractVersion,
		"terminal_failure_read_admission_contracts":         TerminalFailureReadAdmissionContractVersion,
		"delta_union_completeness_read_admission_contracts": DeltaUnionCompletenessReadAdmissionContractVersion,
		"reindex_progress_read_admission_contracts":         ReindexProgressReadAdmissionContractVersion,
		"shared_projection_tuning_report_contracts":         SharedProjectionTuningReportContractVersion,
		"retire_unclaimed_admission_contracts":              RetireUnclaimedAdmissionContractVersion,
		"core":                                              s.coreBaseURL,
	})
}

// readyDialAddress resolves the TCP address one /readyz check dials:
// the configured core host with its explicit port, or the scheme
// default when the origin carries none. It reports false when no core
// origin is configured, so the probe fails closed without dialing.
func (s *Server) readyDialAddress() (string, bool) {
	if s.core == nil {
		return "", false
	}
	host := s.core.Hostname()
	if host == "" {
		return "", false
	}
	port := s.core.Port()
	if port == "" {
		port = "443"
		if s.core.Scheme == "http" {
			port = "80"
		}
	}
	return net.JoinHostPort(host, port), true
}

// checkReady reports whether the relay can currently reach its core:
// an open TCP connection within readyProbeTimeout. The dial stays at
// the transport layer on purpose — no TLS handshake, no credential,
// no core route — so a bare listener reads ready and only a truly
// unreachable origin reads not-ready. The reason is empty when ready
// and otherwise the closed relay-failure reason, so the probe body
// and the readiness metric share one vocabulary with the relay lane.
func (s *Server) checkReady() (bool, string) {
	addr, ok := s.readyDialAddress()
	if !ok {
		return false, reasonUnconfigured
	}
	conn, err := net.DialTimeout("tcp", addr, readyProbeTimeout)
	if err != nil {
		return false, dispatchFailureReason(err)
	}
	_ = conn.Close()
	return true, ""
}

// ready reports readiness in the same key shape the live probe uses:
// service, probe, status. A relay that can reach its core answers 200
// with status ok, byte-identical to the historical probe. A relay
// with no (or no reachable) core origin answers 503 with status
// unavailable and the closed failure reason in details, so the
// Kubernetes readiness probe stops routing traffic to a relay whose
// /api/* would only answer the local 503. Every verdict files the
// readiness counter (see telemetry.go) so an operator can see why the
// probe failed without reading the probe body.
func (s *Server) ready(w http.ResponseWriter, _ *http.Request) {
	ok, reason := s.checkReady()
	if tel := s.telemetry; tel != nil {
		result, metricReason := readinessReady, readinessReasonNone
		if !ok {
			result, metricReason = readinessNotReady, reason
		}
		tel.observeReadiness(result, metricReason)
	}
	if ok {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "eshu-api",
			"probe":   "readyz",
			"status":  "ok",
		})
		return
	}
	writeJSON(w, http.StatusServiceUnavailable, readyNotReadyBody{
		Service: "eshu-api",
		Probe:   "readyz",
		Status:  "unavailable",
		Details: map[string]string{"reason": reason},
	})
}
