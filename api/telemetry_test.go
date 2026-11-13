// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestClassifyFamily_KnownRoutes pins the bounded family table: every
// known route family classifies to its closed label, raw path params
// never leak into the label, and anything unlisted falls to "other".
func TestClassifyFamily_KnownRoutes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{"/api/v0/supply-chain/impact/suppressions", "suppression"},
		{"/api/v0/supply-chain/impact/suppressions/", "suppression"},
		{"/api/v0/admin/collector-failures/acknowledge", "terminal_failure"},
		{"/api/v0/admin/repository-retirements", "retirement"},
		{"/api/v0/admin/workflow-retire-unclaimed", "retire_unclaimed"},
		{"/api/v0/admin/workflow-retire-unclaimed/", "retire_unclaimed"},
		{"/api/v0/admin/replay", "recovery"},
		{"/api/v0/admin/reopen", "recovery"},
		{"/api/v0/admin/refinalize", "recovery"},
		{"/api/v0/admin/recover-generations", "recovery"},
		{"/api/v0/admin/backfill", "admin"},
		{"/api/v0/admin/dead-letter", "admin"},
		{"/api/v0/admin/reindex", "admin"},
		{"/api/v0/admin/skip", "admin"},
		{"/api/v0/admin/reindex/progress", "admin"},
		{"/api/v0/admin/delta-union/completeness", "admin"},
		{"/api/v0/admin/shared-projection/tuning-report", "admin"},
		{"/api/v0/auth/admin/provider-configs", "provider_config"},
		{"/api/v0/auth/admin/provider-configs/cfg-123", "provider_config"},
		{"/api/v0/auth/admin/provider-configs/cfg-123/revisions", "provider_config"},
		{"/api/v0/auth/admin/provider-configs/cfg-123/enable", "provider_config"},
		{"/api/v0/auth/admin/sign-in-policy", "sign_in_policy"},
		{"/api/v0/auth/sign-in-policy", "sign_in_policy"},
		{"/api/v0/auth/setup-state", "setup"},
		{"/api/v0/auth/setup/claim", "setup"},
		{"/api/v0/auth/setup/admin", "setup"},
		{"/api/v0/auth/setup/mfa", "setup"},
		{"/api/v0/auth/oidc/login", "oidc_login"},
		{"/api/v0/auth/oidc/callback", "oidc_login"},
		{"/api/v0/auth/github/login", "github_login"},
		{"/api/v0/auth/github/callback", "github_login"},
		{"/api/v0/auth/saml/providers/acme/acs", "saml_login"},
		{"/api/v0/auth/saml/providers/acme/login", "saml_login"},
		{"/api/v0/auth/saml/providers/acme/metadata", "saml_login"},
		{"/api/v0/capabilities", "capabilities"},
		{"/api/v0/ask", "ask"},
		{"/api/v0/answer-quality/score", "cli_support"},
		{"/api/v0/investigation/packet/render", "cli_support"},
		{"/api/v0/report/bundle/capture", "cli_support"},
		{"/api/v0/report/bundle/validate", "cli_support"},
		{"/api/v0/service/report/compose", "cli_support"},
		{"/api/v0/services/payments/intelligence-report", "service_intel"},
		{"/api/v0/auth/providers", "auth_providers"},
		{"/api/v0/openapi.json", "api_router"},
		{"/api/v0/docs", "api_router"},
		{"/api/v0/redoc", "api_router"},
		{"/api/v0/catalog", "repositories"},
		{"/api/v0/ci-cd/run-correlations", "cicd"},
		{"/api/v0/ci-cd/run-correlations/count", "cicd"},
		{"/api/v0/cloud/resources", "infra"},
		{"/api/v0/documentation/findings", "documentation"},
		{"/api/v0/ecosystem/overview", "infra"},
		{"/api/v0/evidence/bundle", "evidence"},
		{"/api/v0/evidence/relationships/res-1", "evidence"},
		{"/api/v0/infra/resources/count", "infra"},
		{"/api/v0/investigation-workflows", "investigation_workflows"},
		{"/api/v0/investigations/deployable-unit/packet", "evidence"},
		{"/api/v0/package-registry/packages", "package_registry"},
		{"/api/v0/query-playbooks", "playbooks"},
		{"/api/v0/relationships/catalog", "infra"},
		{"/api/v0/status/pipeline", "status"},
		{"/api/v0/collectors", "status"},
		{"/api/v0/supply-chain/impact/findings", "supply_chain"},
		{"/api/v0/investigations/supply-chain/impact/packet", "supply_chain"},
		{"/api/v0/iac/unmanaged-resources", "iac"},
		{"/api/v0/iac/management-status", "iac"},
		{"/api/v0/iac/management-status/explain", "iac"},
		{"/api/v0/iac/terraform-import-plan/candidates", "iac"},
		{"/api/v0/iac/resources", "iac"},
		{"/api/v0/iac/dead", "iac"},
		{"/api/v0/freshness/changed-since", "freshness"},
		{"/api/v0/freshness/generations", "freshness"},
		{"/api/v0/freshness/services/changed-since", "freshness"},
		{"/api/v0/content/files/read", "content"},
		{"/api/v0/content/files/lines", "content"},
		{"/api/v0/content/entities/read", "content"},
		{"/api/v0/content/files/search", "content"},
		{"/api/v0/repositories", "repositories"},
		{"/api/v0/repositories/repo-1/tree", "repositories"},
		{"/api/v0/secrets-iam/posture-summary", "secrets_iam"},
		{"/api/v0/secrets-iam/identity-trust-chains", "secrets_iam"},
		{"/api/v0/replatforming/selectors", "iac"},
		{"/api/v0/replatforming/rollups", "iac"},
		{"/api/v0/aws/runtime-drift/findings", "iac"},
		{"/api/v0/metrics/timeseries", "metrics"},
		{"/api/v0/search/semantic", "semantic_search"},
		{"/api/v0/terraform/config-state-drift/findings", "terraform"},
		{"/api/v0/some/future/route/with/param-123", "other"},
		{"/api/v1/supply-chain/impact/suppressions", "suppression"},
		{"/api/v0", "other"},
		{"/api/v1", "other"},
		{"/health", "other"},
	}
	for _, tc := range cases {
		if got := classifyFamily(tc.path); got != tc.want {
			t.Errorf("classifyFamily(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestStatusClass_ClosedVocabulary pins the status-class mapping,
// including the redirect class and the out-of-range fallback.
func TestStatusClass_ClosedVocabulary(t *testing.T) {
	t.Parallel()

	cases := map[int]string{
		200: "2xx", 201: "2xx",
		301: "3xx", 302: "3xx",
		400: "4xx", 401: "4xx", 403: "4xx", 404: "4xx",
		500: "5xx", 503: "5xx",
		99: "unknown", 600: "unknown",
	}
	for code, want := range cases {
		if got := statusClass(code); got != want {
			t.Errorf("statusClass(%d) = %q, want %q", code, got, want)
		}
	}
}

// TestTelemetry_RequestCountsPerFamily pins per-family request counting:
// one relayed read and one native suppression upsert land in their own
// family rows with the right status class. The relayed leg uses the
// dependencies read: the repositories read it replaced terminated
// natively in the D4 wave (it now files under repositories, not other).
func TestTelemetry_RequestCountsPerFamily(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(discardLogger())
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/dependencies?limit=5"))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("POST", "/api/v0/supply-chain/impact/suppressions"))

	snap := tel.snapshot()
	if snap.requests["other"]["2xx"] != 1 {
		t.Errorf("requests[other][2xx] = %d, want 1", snap.requests["other"]["2xx"])
	}
	if snap.requests["suppression"]["2xx"] != 1 {
		t.Errorf("requests[suppression][2xx] = %d, want 1", snap.requests["suppression"]["2xx"])
	}
}

// TestTelemetry_StatusClasses pins status-class recording across the
// gate and the lanes: 401s from the auth gate, core statuses (relayed
// or terminated), and the local 503 each land in their class row.
func TestTelemetry_StatusClasses(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusTeapot, body: `{"teapot":true}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(discardLogger())
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	unauth := httptest.NewRequest("GET", "/api/v0/capabilities", nil)
	handler.ServeHTTP(httptest.NewRecorder(), unauth)

	relay := authedRequest("GET", "/api/v0/capabilities")
	handler.ServeHTTP(httptest.NewRecorder(), relay)

	dead := NewServer("http://127.0.0.1:1", skeletonTestKey)
	dead.telemetry = tel
	deadHandler := dead.Handler(DefaultV0Sunset)
	handler503 := authedRequest("GET", "/api/v0/ask")
	deadHandler.ServeHTTP(httptest.NewRecorder(), handler503)

	snap := tel.snapshot()
	if snap.requests["capabilities"]["4xx"] != 2 {
		t.Errorf("requests[capabilities][4xx] = %d, want 2 (401 + terminated 418)", snap.requests["capabilities"]["4xx"])
	}
	if snap.requests["ask"]["5xx"] != 1 {
		t.Errorf("requests[ask][5xx] = %d, want 1 (local 503)", snap.requests["ask"]["5xx"])
	}
}

// TestTelemetry_DurationHistogram pins the duration histogram: every
// recorded request lands in the count, the sum, and the +Inf bucket.
func TestTelemetry_DurationHistogram(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	tel.observeRequest("ask", "2xx", 42*time.Millisecond)

	snap := tel.snapshot()
	hist := snap.durations["ask"]
	if hist.count != 1 {
		t.Fatalf("durations[ask].count = %d, want 1", hist.count)
	}
	if hist.sum != float64(42*time.Millisecond)/float64(time.Second) {
		t.Errorf("durations[ask].sum = %v, want 0.042", hist.sum)
	}
	last := hist.buckets[len(hist.buckets)-1]
	if last != 1 {
		t.Errorf("durations[ask] +Inf bucket = %d, want 1", last)
	}
	// Cumulative buckets: no finite bucket may exceed the +Inf bucket.
	for i, bucket := range hist.buckets {
		if bucket > last {
			t.Errorf("durations[ask] bucket[%d] = %d exceeds +Inf %d: buckets must be cumulative", i, bucket, last)
		}
	}
}

// TestTelemetry_DispatchOutcomes pins the dispatch-outcome matrix: relay
// vs native, core answers vs the local permanent 503, and relayed
// retryable 503s (Retry-After preserved) vs relayed plain 503s. The
// relay legs use the dependencies read, which still crosses the generic
// proxy (the D4 wave terminated the repositories probe they replaced).
func TestTelemetry_DispatchOutcomes(t *testing.T) {
	t.Parallel()

	t.Run("relay core_ok", func(t *testing.T) {
		t.Parallel()
		tel, handler := telemetryWithCore(t, &stubCore{status: http.StatusOK, body: `{}`}, "https://core.invalid")
		handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/dependencies"))
		if got := tel.snapshot().dispatches["other"]["relay"]["core_ok"]; got != 1 {
			t.Errorf("dispatches[other][relay][core_ok] = %d, want 1", got)
		}
	})

	t.Run("relay core_retryable_503 preserves Retry-After", func(t *testing.T) {
		t.Parallel()
		stub := &stubCore{
			status: http.StatusServiceUnavailable,
			body:   `{"error":"overloaded"}`,
			header: http.Header{"Retry-After": []string{"30"}},
		}
		tel, handler := telemetryWithCore(t, stub, "https://core.invalid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/dependencies"))
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Errorf("Retry-After = %q, want the relayed core value '30'", got)
		}
		if got := tel.snapshot().dispatches["other"]["relay"]["core_retryable_503"]; got != 1 {
			t.Errorf("dispatches[other][relay][core_retryable_503] = %d, want 1", got)
		}
	})

	t.Run("relay core_503 without Retry-After", func(t *testing.T) {
		t.Parallel()
		tel, handler := telemetryWithCore(t, &stubCore{status: http.StatusServiceUnavailable, body: `{}`}, "https://core.invalid")
		handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/dependencies"))
		if got := tel.snapshot().dispatches["other"]["relay"]["core_503"]; got != 1 {
			t.Errorf("dispatches[other][relay][core_503] = %d, want 1", got)
		}
	})

	t.Run("dead core is local_503 with no Retry-After", func(t *testing.T) {
		t.Parallel()
		tel, _ := telemetryWithCore(t, nil, "http://127.0.0.1:1")
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		srv.telemetry = tel
		handler := srv.Handler(DefaultV0Sunset)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/ask"))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Retry-After = %q, want absent on the local permanent 503", got)
		}
		if got := tel.snapshot().dispatches["ask"]["relay"]["local_503"]; got != 1 {
			t.Errorf("dispatches[ask][relay][local_503] = %d, want 1", got)
		}
	})

	t.Run("unconfigured core is local_503", func(t *testing.T) {
		t.Parallel()
		tel := NewTelemetry(discardLogger())
		srv := NewServer("", skeletonTestKey)
		srv.telemetry = tel
		handler := srv.Handler(DefaultV0Sunset)
		handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/dependencies"))
		if got := tel.snapshot().dispatches["other"]["relay"]["local_503"]; got != 1 {
			t.Errorf("dispatches[other][relay][local_503] = %d, want 1", got)
		}
	})

	t.Run("native core_ok", func(t *testing.T) {
		t.Parallel()
		tel, handler := telemetryWithCore(t, &stubCore{status: http.StatusCreated, body: `{"status":"created"}`}, "https://core.invalid")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("POST", "/api/v0/supply-chain/impact/suppressions"))
		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201", rec.Code)
		}
		if got := tel.snapshot().dispatches["suppression"]["native"]["core_ok"]; got != 1 {
			t.Errorf("dispatches[suppression][native][core_ok] = %d, want 1", got)
		}
	})

	t.Run("native deny records denied plus authorizer deny", func(t *testing.T) {
		t.Parallel()
		tel := NewTelemetry(discardLogger())
		srv := NewServer("http://core.invalid", skeletonTestKey)
		srv.telemetry = tel
		rec := httptest.NewRecorder()
		srv.suppressionWriteThroughAs(rec, authedRequest("POST", "/api/v0/supply-chain/impact/suppressions"), false)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		snap := tel.snapshot()
		if got := snap.dispatches["suppression"]["native"]["denied"]; got != 1 {
			t.Errorf("dispatches[suppression][native][denied] = %d, want 1", got)
		}
		if got := snap.authorizer["suppression"]["deny"]; got != 1 {
			t.Errorf("authorizer[suppression][deny] = %d, want 1", got)
		}
	})
}

// TestTelemetry_AuthorizerAllowDeny pins per-family authorizer counters:
// the live native allow branch counts allow, the unreachable deny branch
// counts deny, making the deny branch observable from dest signals.
func TestTelemetry_AuthorizerAllowDeny(t *testing.T) {
	t.Parallel()

	tel, handler := telemetryWithCore(t, &stubCore{status: http.StatusCreated, body: `{"status":"created"}`}, "https://core.invalid")
	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("POST", "/api/v0/supply-chain/impact/suppressions"))

	snap := tel.snapshot()
	if got := snap.authorizer["suppression"]["allow"]; got != 1 {
		t.Errorf("authorizer[suppression][allow] = %d, want 1", got)
	}
	if got := snap.authorizer["suppression"]["deny"]; got != 0 {
		t.Errorf("authorizer[suppression][deny] = %d, want 0 on the allow path", got)
	}
}

// TestTelemetry_AuditDropObserver pins the dropped-audit counter wired
// to the Leaf A observer contract: the method value satisfies the
// GovernanceAuditDropObserver shape (ctx, sink error, event count) and
// counts drops plus dropped events.

func telemetryWithCore(t *testing.T, stub *stubCore, coreURL string) (*Telemetry, http.Handler) {
	t.Helper()

	tel := NewTelemetry(discardLogger())
	if stub != nil {
		stub.t = t
		upstream := httptest.NewServer(stub.handler())
		t.Cleanup(upstream.Close)
		srv := NewServer(upstream.URL, skeletonTestKey)
		srv.telemetry = tel
		return tel, srv.Handler(DefaultV0Sunset)
	}
	srv := NewServer(coreURL, skeletonTestKey)
	srv.telemetry = tel
	return tel, srv.Handler(DefaultV0Sunset)
}

// discardLogger returns a JSON logger that drops every line: the quiet
// default for counter-only tests.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}
