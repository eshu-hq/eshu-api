// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSuppressionPilot_UnauthenticatedIs401 pins that the native route
// keeps the dest 401 shape: envelope callers get the envelope error,
// everyone else the plain error_code body, both with the bare Bearer
// challenge.
func TestSuppressionPilot_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions", strings.NewReader(`{}`))
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: status = %d, want 401", accept, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Accept %q: WWW-Authenticate = %q, want bare Bearer", accept, got)
		}
	}
}

// TestSuppressionPilot_AuthorizerRejectsNonAllScopes pins the family
// 403 the dest authorizer adds: a non-AllScopes identity never reaches
// the core. Dest has no scoped/session resolver, so the deny branch is
// unreachable in production (the shared key maps to AllScopes in core,
// which re-judges as backstop); the test pins the documented shape, not
// a live path.
func TestSuppressionPilot_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeSuppressionOperator(true) {
		t.Errorf("authorizeSuppressionOperator(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeSuppressionOperator(false) {
		t.Errorf("authorizeSuppressionOperator(false) = allow, want deny (family 403)")
	}
}

// TestSuppressionPilot_DenyBranchServesFamily403 pins the pilot's
// native 403 at the HTTP layer: a non-AllScopes identity is denied in
// dest with the documented shape and never reaches the core, in both
// negotiated shapes.
func TestSuppressionPilot_DenyBranchServesFamily403(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.suppressionWriteThroughAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestSuppressionPilot_WriteThroughRelaysCoreBytes pins the pilot
// write-through: method, path, query, body, and the original
// Authorization header cross to the core, and the core's status,
// headers, and body come back untouched across 201/200/400/403/503.
func TestSuppressionPilot_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"created", http.StatusCreated, `{"suppression_id":"s1","generation_id":"g1","status":"created"}`},
		{"unchanged", http.StatusOK, `{"suppression_id":"s1","generation_id":"g1","status":"unchanged"}`},
		{"bad request", http.StatusBadRequest, `{"error":"Bad Request","detail":"unknown field"}`},
		{"core 403", http.StatusForbidden, `{"error":"Forbidden","detail":"all-scopes operator authorization is required"}`},
		{"core 503", http.StatusServiceUnavailable, `{"error":"Service Unavailable","detail":"store is unavailable"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: tc.status, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			payload := `{"suppression_id":"s1","justification":"accepted_risk"}`
			req := authedRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions?dry_run=1")
			req.Body = io.NopCloser(strings.NewReader(payload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want exactly 1 (no retry on write-through)", core.hits)
			}
			if core.lastMethod != http.MethodPost {
				t.Errorf("upstream method = %q, want POST", core.lastMethod)
			}
			if core.lastPath != "/api/v0/supply-chain/impact/suppressions" {
				t.Errorf("upstream path = %q, want the core route", core.lastPath)
			}
			if core.lastQuery != "dry_run=1" {
				t.Errorf("upstream query = %q, want dry_run=1", core.lastQuery)
			}
			if core.lastBody != payload {
				t.Errorf("upstream body = %q, want the caller bytes", core.lastBody)
			}
			if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
				t.Errorf("upstream Authorization = %q, want the forwarded Bearer key", core.lastAuth)
			}
			if rec.Code != tc.status {
				t.Errorf("status = %d, want relayed %d", rec.Code, tc.status)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != tc.body {
				t.Errorf("body = %q, want the core bytes %q", body, tc.body)
			}
		})
	}
}

// TestSuppressionPilot_OtherFamiliesStillProxy pins that the pilot
// changes nothing for other families: a non-suppression route still
// crosses the generic byte-relay.
func TestSuppressionPilot_OtherFamiliesStillProxy(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest("GET", "/api/v0/repositories?limit=1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/repositories" {
		t.Errorf("hits = %d path = %q, want generic proxy to repositories", core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

// TestSuppressionPilot_DeadCoreIs503WithoutRetryAfter pins the pilot's
// dead-core shape on the native route: permanent 503
// backend_unavailable with no Retry-After, in both negotiated shapes.
func TestSuppressionPilot_DeadCoreIs503WithoutRetryAfter(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPost, "/api/v0/supply-chain/impact/suppressions")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After = %q, want absent (permanent 503)", accept, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "backend_unavailable") {
			t.Errorf("Accept %q: body = %q, want backend_unavailable", accept, body)
		}
	}
}

// TestSuppressionPilot_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite still lands the suppression route on its native handler with
// the v0 deprecation headers intact.
func TestSuppressionPilot_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusCreated, body: `{"status":"created"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/supply-chain/impact/suppressions")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/supply-chain/impact/suppressions" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want relayed 201", rec.Code)
	}
}
