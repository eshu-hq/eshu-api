// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTelemetry_AuditDropObserver(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())

	// The method value must satisfy the Leaf A observer shape from
	// eshu go/internal/query/governance_audit_admission.go:
	// GovernanceAuditDropObserver func(ctx context.Context, err error, eventCount int).
	// Blank-identifier pin: keeps the explicit shape assertion without
	// tripping ST1023 (CI: lanes/lint).
	var _ func(context.Context, error, int) = tel.ObserveAuditDrop

	tel.ObserveAuditDrop(context.Background(), errors.New("sink unavailable"), 3)
	tel.ObserveAuditDrop(context.Background(), errors.New("sink unavailable"), 2)

	snap := tel.snapshot()
	if snap.auditDrops != 2 {
		t.Errorf("auditDrops = %d, want 2", snap.auditDrops)
	}
	if snap.auditDroppedEvents != 5 {
		t.Errorf("auditDroppedEvents = %d, want 5", snap.auditDroppedEvents)
	}
}

// TestTelemetry_MetricsEndpoint pins the public /metrics surface: no
// credential required, Prometheus text content type, and the per-family
// counters rendered with closed label vocabularies.
func TestTelemetry_MetricsEndpoint(t *testing.T) {
	t.Parallel()

	tel := NewTelemetry(discardLogger())
	core := &stubCore{status: http.StatusOK, body: `{}`}
	upstream := httptest.NewServer(core.handler())
	t.Cleanup(upstream.Close)
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)
	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/capabilities"))
	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("POST", "/api/v0/supply-chain/impact/suppressions"))

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200 without a credential", rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Errorf("Content-Type = %q, want the Prometheus text prefix", contentType)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	text := string(body)
	for _, want := range []string{
		"# HELP eshu_api_requests_total",
		"# TYPE eshu_api_requests_total counter",
		`eshu_api_requests_total{family="capabilities",class="2xx"} 1`,
		"# TYPE eshu_api_request_duration_seconds histogram",
		"# TYPE eshu_api_dispatch_total counter",
		"# TYPE eshu_api_core_latency_seconds histogram",
		"# TYPE eshu_api_authorizer_total counter",
		`eshu_api_authorizer_total{family="suppression",decision="allow"} 1`,
		`eshu_api_dispatch_total{family="suppression",via="native",outcome="core_ok"} 1`,
		"# TYPE eshu_api_audit_drops_total counter",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("/metrics missing %q\nfull body:\n%s", want, text)
		}
	}
}

// TestTelemetry_HealthAndReadyzUnchanged pins that the telemetry lane
// leaves the probes alone: still public, still the exact shape, and
// still free of deprecation headers.
func TestTelemetry_HealthAndReadyzUnchanged(t *testing.T) {
	t.Parallel()

	_, handler := telemetryWithCore(t, &stubCore{status: http.StatusOK, body: `{}`}, "https://core.invalid")

	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest("GET", path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", path, rec.Code)
		}
		if got := rec.Header().Get("X-Correlation-ID"); got != "" {
			t.Errorf("GET %s X-Correlation-ID = %q, want empty on probes", path, got)
		}
	}
}

// TestTelemetry_CorrelationEchoAndLogs pins the correlation contract:
// the auth gate echoes a caller-supplied ID and mints one when absent,
// dispatch responses carry it, structured logs key on it, and the
// credential never appears in a log line.
func TestTelemetry_CorrelationEchoAndLogs(t *testing.T) {
	t.Parallel()

	var logs strings.Builder
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	tel := NewTelemetry(logger)

	core := &stubCore{status: http.StatusOK, body: `{}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	echoed := authedRequest("GET", "/api/v0/capabilities")
	echoed.Header.Set("X-Correlation-ID", "op-trace-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, echoed)
	if got := rec.Header().Get("X-Correlation-ID"); got != "op-trace-1" {
		t.Errorf("X-Correlation-ID = %q, want the echoed caller value", got)
	}

	minted := authedRequest("GET", "/api/v0/ask")
	mintedRec := httptest.NewRecorder()
	handler.ServeHTTP(mintedRec, minted)
	mintedID := mintedRec.Header().Get("X-Correlation-ID")
	if mintedID == "" {
		t.Fatalf("X-Correlation-ID is empty, want a minted identifier")
	}

	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2 (one structured line per request)", len(lines))
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line %d is not JSON: %v", i, err)
		}
		if _, ok := entry["correlation_id"]; !ok {
			t.Errorf("log line %d = %v, want a correlation_id key", i, entry)
		}
		if _, ok := entry["family"]; !ok {
			t.Errorf("log line %d = %v, want a family key", i, entry)
		}
		if strings.Contains(line, skeletonTestKey) {
			t.Errorf("log line %d contains the credential, which must never be logged", i)
		}
	}
	var first map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &first)
	if first["correlation_id"] != "op-trace-1" {
		t.Errorf("first log correlation_id = %v, want the echoed op-trace-1", first["correlation_id"])
	}
	var second map[string]any
	_ = json.Unmarshal([]byte(lines[1]), &second)
	if second["correlation_id"] != mintedID {
		t.Errorf("second log correlation_id = %v, want the minted %q", second["correlation_id"], mintedID)
	}
}

// TestTelemetry_TraceContextPropagatesToCore pins that trace context
// crosses to the core untouched (traceparent/tracestate are never
// hop-by-hop) and that a correlation ID is always present upstream:
// the caller's when supplied, the minted one otherwise. Dest invents
// no truth levels or error codes here: it only forwards.
func TestTelemetry_TraceContextPropagatesToCore(t *testing.T) {
	t.Parallel()

	var gotTraceparent, gotTracestate, gotCorrelation string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceparent = r.Header.Get("traceparent")
		gotTracestate = r.Header.Get("tracestate")
		gotCorrelation = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = NewTelemetry(discardLogger())
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest("GET", "/api/v0/capabilities")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "rojo=00f067aa0ba902b7")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotTraceparent != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Errorf("upstream traceparent = %q, want the caller bytes untouched", gotTraceparent)
	}
	if gotTracestate != "rojo=00f067aa0ba902b7" {
		t.Errorf("upstream tracestate = %q, want the caller bytes untouched", gotTracestate)
	}
	if gotCorrelation == "" {
		t.Errorf("upstream X-Correlation-ID is empty, want the minted ID propagated")
	}

	rec := httptest.NewRecorder()
	echoed := authedRequest("GET", "/api/v0/capabilities")
	echoed.Header.Set("X-Correlation-ID", "join-the-trace")
	handler.ServeHTTP(rec, echoed)
	if gotCorrelation != "join-the-trace" {
		t.Errorf("upstream X-Correlation-ID = %q, want the caller value", gotCorrelation)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "join-the-trace" {
		t.Errorf("response X-Correlation-ID = %q, want the echoed caller value", got)
	}
}

// TestTelemetry_BoundedCardinality pins that unbounded path segments
// never become label values: param-heavy routes collapse to their
// family row or "other", never to a per-ID series.
func TestTelemetry_BoundedCardinality(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(discardLogger())
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/services/acme-payments-9f3a/intelligence-report"))
	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/auth/admin/provider-configs/8f3a2c1e-uuid/revisions"))
	handler.ServeHTTP(httptest.NewRecorder(), authedRequest("GET", "/api/v0/tenants/8f3a2c1e-uuid/whatever/deep"))

	snap := tel.snapshot()
	for family := range snap.requests {
		switch family {
		case "service_intel", "provider_config", "other", "capabilities", "ask", "suppression",
			"terminal_failure", "retirement", "sign_in_policy", "setup",
			"oidc_login", "github_login", "saml_login", "cli_support",
			"auth_providers", "api_router":
		default:
			t.Errorf("requests has family %q, outside the closed vocabulary", family)
		}
	}
	if snap.requests["service_intel"]["2xx"] != 1 {
		t.Errorf("requests[service_intel][2xx] = %d, want 1 (param stripped)", snap.requests["service_intel"]["2xx"])
	}
	rendered := tel.render()
	if strings.Contains(rendered, "8f3a2c1e") || strings.Contains(rendered, "acme-payments") {
		t.Errorf("/metrics leaks raw path params:\n%s", rendered)
	}
}

// TestTelemetry_NilTelemetryPassesThrough pins the degraded path: a
// server with no registry attached still serves dispatch requests
// untouched, with no correlation header invented and no panic.
func TestTelemetry_NilTelemetryPassesThrough(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	upstream := httptest.NewServer(core.handler())
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = nil
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/capabilities"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with nil telemetry", rec.Code)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "" {
		t.Errorf("X-Correlation-ID = %q, want empty when telemetry is detached", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want the relayed core bytes", body)
	}
}

// telemetryWithCore builds a stub-core handler wired to a fresh
