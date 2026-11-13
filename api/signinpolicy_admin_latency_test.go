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

// W7-read admin sign-in-policy GET latency evidence (Refs
// eshu-hq/eshu#9669, D13 tracker eshu-hq/eshu#10491, satellite
// eshu-hq/eshu-api#72). The terminated reader does the same single
// server-side fetch as the generic relay plus the family gate and
// the family authorizer; these benchmarks bound that overhead for
// both spellings so
// scripts/measure-signinpolicy-admin-read-latency.sh can gate it in
// CI. The stub core answers from memory — the numbers isolate
// dest-side cost, not core or network time. Absolute deployment
// numbers ride the pinned-core-image leg at flip prep (D11).

// adminSignInPolicyBenchServer builds a Handler over a stub core
// answering body for every route, so terminated and relay benchmarks
// share the same upstream shape.
func adminSignInPolicyBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkAdminSignInPolicyTerminatedRead measures the terminated
// path for a shared-key GET through the full production chain: one
// leg per spelling through the same family gate and family
// authorizer.
func BenchmarkAdminSignInPolicyTerminatedRead(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"tenant_id":"tenant_a","require_sso":false}`)
	handler := srv.Handler(DefaultV0Sunset)

	gets := map[string]*http.Request{
		"exact": authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy"),
		"twin":  authedRequest(http.MethodGet, "/api/v0/auth/admin/sign-in-policy/"),
	}
	for name, get := range gets {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, get.Clone(get.Context()))
			}
		})
	}
}

// BenchmarkAdminSignInPolicyRelayRead measures the generic relay
// path for the same caller and stub shape: the before leg the lane
// must not regress. The legs compare different routes by necessity
// (the admin path now terminates); both do exactly one server-side
// fetch, so the 1.50 bound stays a real pathology check.
func BenchmarkAdminSignInPolicyRelayRead(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"tenant_id":"tenant_a","require_sso":false}`)
	handler := srv.Handler(DefaultV0Sunset)

	get := authedRequest(http.MethodGet, "/api/v0/dependencies")

	b.Run("get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, get.Clone(get.Context()))
		}
	})
}

// BenchmarkAdminSignInPolicyWriteThrough measures the terminated
// writer path for a shared-key PATCH through the full production
// chain: one leg per spelling through the same family writer gate
// and family authorizer, with the opaque caller body. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).
func BenchmarkAdminSignInPolicyWriteThrough(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"require_sso":true}`)
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"require_sso":true}`
	build := func(target string) *http.Request {
		req := authedRequest(http.MethodPatch, target)
		req.Body = io.NopCloser(strings.NewReader(payload))
		return req
	}
	patches := map[string]*http.Request{
		"exact": build("/api/v0/auth/admin/sign-in-policy"),
		"twin":  build("/api/v0/auth/admin/sign-in-policy/"),
	}
	for name, patch := range patches {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, patch.Clone(patch.Context()))
			}
		})
	}
}

// BenchmarkAdminSignInPolicyRelayWrite measures the generic relay
// path for the same writer shape: PATCH /api/v0/code/bundles, which
// stays on the generic relay. The legs compare different routes by
// necessity (the admin PATCH now terminates on the family lane);
// both do exactly one server-side fetch, so the 1.50 bound stays a
// real pathology check.
func BenchmarkAdminSignInPolicyRelayWrite(b *testing.B) {
	_, srv := adminSignInPolicyBenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	relay := authedRequest(http.MethodPatch, "/api/v0/code/bundles")

	b.Run("patch", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, relay.Clone(relay.Context()))
		}
	})
}

// TestAdminSignInPolicyWriteBenchmarks_ServeShapes pins that the
// writer benchmark legs above exercise the intended transports: the
// exact and twin PATCH legs terminate natively (one stub-core hit
// each behind the shared key), and the relay PATCH leg reaches the
// stub core exactly once behind the key.
func TestAdminSignInPolicyWriteBenchmarks_ServeShapes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"/api/v0/auth/admin/sign-in-policy",
		"/api/v0/auth/admin/sign-in-policy/",
	} {
		req := authedRequest(http.MethodPatch, target)
		req.Body = io.NopCloser(strings.NewReader(`{"require_sso":true}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("PATCH %s: status = %d, want 200", target, rec.Code)
		}
	}

	relay := authedRequest(http.MethodPatch, "/api/v0/code/bundles")
	relayRec := httptest.NewRecorder()
	handler.ServeHTTP(relayRec, relay)
	if relayRec.Code != http.StatusOK {
		t.Errorf("relay PATCH status = %d, want 200", relayRec.Code)
	}

	if core.hits != 3 {
		t.Errorf("upstream hits = %d, want 3 (exact plus twin plus relay, one forward each)", core.hits)
	}
}
