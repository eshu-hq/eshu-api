// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"strings"
)

// Dispatch lanes: which handler served the request. The native lane is
// the explicit per-family route (today only the suppression upsert);
// the relay lane is the generic byte-for-byte proxy every other family
// crosses.
const (
	viaNative = "native"
	viaRelay  = "relay"
)

// Dispatch outcomes: the closed vocabulary for what happened at the
// dispatch step. A local_503 is synthesized by dest (unconfigured
// origin, unreachable core, unbuildable request) and never carries
// Retry-After; core_503 and core_retryable_503 are relayed core
// answers, the latter keeping the Retry-After the core sent.
const (
	outcomeCoreOK        = "core_ok"
	outcomeCore503       = "core_503"
	outcomeCoreRetryable = "core_retryable_503"
	outcomeLocal503      = "local_503"
	outcomeDenied        = "denied"
)

// Family authorizer decisions and the fallback family label.
const (
	decisionAllow = "allow"
	decisionDeny  = "deny"
	familyOther   = "other"
)

// correlationHeader is echoed on dispatch responses (caller value when
// supplied, minted otherwise) and guaranteed upstream. Trace context
// itself (traceparent/tracestate) already crosses the relay untouched;
// dest never interprets it.
const correlationHeader = "X-Correlation-ID"

// durationBuckets bounds the request-duration and core-latency
// histograms in seconds. The top bound matches the dispatch timeout,
// so a timed-out core call still lands in a finite bucket.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

// familyPrefixes maps a stripped route prefix (the path after /api/v0
// or /api/v1) to its closed family label, longest prefix first. Route
// evidence lives in contracts/upstream/*-wire-shapes.txt (mutation
// families) and the read-transport Native-now wave (read families).
// Anything unlisted collapses to "other": raw path params never become
// label values, so cardinality stays bounded by this table.
var familyPrefixes = []struct {
	prefix string
	family string
}{
	{"/supply-chain/impact/suppressions", "suppression"},
	{"/admin/collector-failures/acknowledge", "terminal_failure"},
	{"/admin/collector-failures/", "terminal_failure"},
	{"/admin/repository-retirements", "retirement"},
	{"/admin/work-items", "admin"},
	{"/admin/decisions", "admin"},
	{"/admin/dead-letter", "admin"},
	{"/admin/input-invalid-facts", "admin"},
	{"/admin/changed-since", "admin"},
	{"/admin/replay-events", "admin"},
	{"/admin/recover-generations", "recovery"},
	{"/admin/refinalize", "recovery"},
	{"/admin/reopen", "recovery"},
	{"/admin/replay", "recovery"},
	{"/auth/admin/provider-configs", "provider_config"},
	{"/auth/admin/sign-in-policy", "sign_in_policy"},
	{"/auth/admin/audit/", "governance_audit"},
	{"/auth/admin/", "admin_identity"},
	{"/auth/local/invitations", "admin_identity"},
	{"/auth/sign-in-policy", "sign_in_policy"},
	{"/auth/setup-state", "setup"},
	{"/auth/setup/", "setup"},
	{"/auth/oidc/", "oidc_login"},
	{"/auth/github/", "github_login"},
	{"/auth/saml/", "saml_login"},
	{"/auth/providers", "auth_providers"},
	{"/answer-quality/score", "cli_support"},
	{"/investigation/packet/render", "cli_support"},
	{"/report/bundle/capture", "cli_support"},
	{"/report/bundle/validate", "cli_support"},
	{"/service/report/compose", "cli_support"},
	{"/services/", "service_intel"},
	{"/capabilities", "capabilities"},
	{"/ask", "ask"},
	{"/openapi.json", "api_router"},
	{"/docs", "api_router"},
	{"/redoc", "api_router"},
}

// classifyFamily maps a request path to its closed family label. The
// /api/v0 (or /api/v1, pre-rewrite) prefix is stripped first; the
// longest listed prefix wins; anything else is "other".
func classifyFamily(path string) string {
	stripped := path
	for _, apiPrefix := range []string{"/api/v0", "/api/v1"} {
		if rest, ok := strings.CutPrefix(path, apiPrefix); ok {
			stripped = rest
			break
		}
	}
	if stripped == "" {
		stripped = "/"
	}
	for _, entry := range familyPrefixes {
		if strings.HasPrefix(stripped, entry.prefix) {
			return entry.family
		}
	}
	return familyOther
}

// statusClass maps a status code to its closed class label.
func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "unknown"
	}
}
