// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// D9 OIDC-login latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The tier-C clone does the same single server-side fetch as
// the generic relay plus the explicit route match and telemetry; these
// benchmarks bound that overhead per representative route so
// scripts/measure-d9-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// d9BenchServer builds a Handler over a stub core answering body for
// every route, so clone and relay benchmarks share the same upstream
// shape.
func d9BenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkD9PublicLogin measures the public carve-out path for a
// keyless login-start GET through the full production chain: bare and
// with the full provider-IDs-plus-return_to query (the realistic
// caller shape whose values must cross opaque).
func BenchmarkD9PublicLogin(b *testing.B) {
	_, srv := d9BenchServer(b, ``)
	handler := srv.Handler(DefaultV0Sunset)

	bare := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login", nil)
	full := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev&tenant_id=tenant_a&workspace_id=workspace_a&return_to=%2Fconsole", nil)

	b.Run("bare", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, bare.Clone(bare.Context()))
		}
	})
	b.Run("query", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, full.Clone(full.Context()))
		}
	})
}

// BenchmarkD9PublicCallback measures the public carve-out path for a
// keyless callback GET carrying the credential state + code through
// the full production chain.
func BenchmarkD9PublicCallback(b *testing.B) {
	_, srv := d9BenchServer(b, `{"csrf_token":"opaque-csrf"}`)
	handler := srv.Handler(DefaultV0Sunset)

	callback := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456", nil)

	b.Run("callback", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, callback.Clone(callback.Context()))
		}
	})
}
