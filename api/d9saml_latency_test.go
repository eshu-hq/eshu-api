// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// D9 SAML-login latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The tier-C clone does the same single server-side fetch as
// the generic relay plus the explicit route match and telemetry; these
// benchmarks bound that overhead per representative route so
// scripts/measure-d9saml-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the
// pinned-core-image leg at flip prep (D11).

// d9SAMLBenchServer builds a Handler over a stub core answering body
// for every route, so clone and relay benchmarks share the same
// upstream shape.
func d9SAMLBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// d9SAMLACSLargeForm is the representative ACS body for the latency
// leg: a realistic IdP-posted form ( RelayState plus a base64 SAML
// assertion) sized near the core-side cap, proving the streaming
// forward adds no per-byte dest cost beyond the single fetch.
var d9SAMLACSLargeForm = "RelayState=" + strings.Repeat("r", 32*1024) +
	"&SAMLResponse=" + strings.Repeat("s", 256*1024)

// BenchmarkD9SAMLPublicMetadata measures the public carve-out path for
// a keyless metadata GET through the full production chain.
func BenchmarkD9SAMLPublicMetadata(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, `<EntityDescriptor entityID="https://eshu.example/saml/p1"/>`)
	handler := srv.Handler(DefaultV0Sunset)

	metadata := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", nil)

	b.Run("metadata", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, metadata.Clone(metadata.Context()))
		}
	})
}

// BenchmarkD9SAMLPublicLogin measures the public carve-out path for a
// keyless login-start GET through the full production chain: bare and
// with the provider path value plus the realistic return_to query
// (whose value must cross opaque).
func BenchmarkD9SAMLPublicLogin(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, ``)
	handler := srv.Handler(DefaultV0Sunset)

	bare := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login", nil)
	full := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)

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

// BenchmarkD9SAMLPublicACS measures the public carve-out path for a
// keyless ACS POST carrying the representative large form body
// through the full production chain.
func BenchmarkD9SAMLPublicACS(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, `{"csrf_token":"opaque-csrf"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("acs", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(d9SAMLACSLargeForm)))
		for i := 0; i < b.N; i++ {
			acs := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(d9SAMLACSLargeForm))
			acs.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, acs)
		}
	})
}

// BenchmarkD9SAMLRelayLargePost measures the generic relay path for a
// keyed POST carrying the same representative large form body the ACS
// leg moves, so the ACS clone leg compares against the same bytes —
// not against an empty-body GET. The Compare POST stays on the generic
// relay by D6 disclosure (no pin covers its shared graph/content
// ports); both legs do exactly one server-side fetch of the same body,
// so the 1.50 bound stays a real pathology check on the clone route
// match, not on body size.
func BenchmarkD9SAMLRelayLargePost(b *testing.B) {
	_, srv := d9SAMLBenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(d9SAMLACSLargeForm)))
		for i := 0; i < b.N; i++ {
			post := httptest.NewRequest(http.MethodPost, "/api/v0/compare/environments", strings.NewReader(d9SAMLACSLargeForm))
			post.Header.Set("Authorization", "Bearer "+skeletonTestKey)
			post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post)
		}
	})
}
