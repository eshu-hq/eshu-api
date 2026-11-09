// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Providers-GET failure/edge termination proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, signinpolicy-group lane). Companion to
// signinpolicy_providers_test.go: every failure below closes to the
// permanent local 503 with never a Retry-After.

// TestProviders_V1AliasTerminates pins that the v1 alias rewrite
// runs ahead of the terminated providers GET.
func TestProviders_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":false}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("v1 status = %d, want terminated 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), `"providers":[]`) {
		t.Errorf("v1 body = %q, want the synthesized posture", body)
	}
}

// TestProviders_DeadCoreSingleAttempt pins the dead-core shape on the
// providers leg: the permanent 503 with a single dispatch attempt.
func TestProviders_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	var attempts int32
	srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_KeyedTwinRelaysNotSynthesizes pins the twin boundary
// from the other side: a keyed trailing-slash twin does NOT hit the
// synthesis — it crosses the generic relay with the core bytes (and
// the core headers) intact. The stub Cache-Control differs from both
// synthesized postures so a synthesis would fail this test.
func TestProviders_KeyedTwinRelaysNotSynthesizes(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`,
		header: http.Header{"Cache-Control": []string{"private"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/providers/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("keyed twin status = %d, want the relayed 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private" {
		t.Errorf("keyed twin Cache-Control = %q, want the relayed core value (not synthesis)", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("keyed twin body = %q, want the core bytes %q", body, core.body)
	}
}

// TestProviders_ExplicitNullProvidersFailsClosed pins the null-list
// mapping: an explicit JSON null providers array violates the core
// contract (the field is always a slice, never null), so dest fails
// closed with the permanent 503 rather than synthesizing a posture
// on it.
func TestProviders_ExplicitNullProvidersFailsClosed(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":null,"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":false}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_UnreadableBodyFailsClosed pins the unreadable-body
// mapping on the providers leg with the shared error-body transport.
func TestProviders_UnreadableBodyFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	srv.httpClient = &http.Client{Transport: errBodyRoundTripper{}}
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_UnbuildableFetchFailsClosed pins the defensive
// mapping on the posture fetch: an unbuildable target fails the
// fetch with the sentinel.
func TestProviders_UnbuildableFetchFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	if _, err := srv.fetchProvidersPosture(t.Context(), signInPolicyFetchParams{
		tenant:      "tenant_a",
		query:       "tenant_id=\x7f",
		correlation: "corr-1",
	}); err != errSignInPolicyFetch {
		t.Errorf("fetch error = %v, want the sentinel", err)
	}
}
