// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// D10 setup latency evidence (Refs eshu-hq/eshu#9669, part of
// #6707). The setup-state GET terminates dest-side via the
// SetupNeeded port while the three credential POSTs stay tier-C
// write-through; these benchmarks bound the per-route overhead of
// the new transport against the generic relay on the same
// in-process stub core, so scripts/measure-setup-latency.sh can
// gate it in CI. The stub core answers from memory — the numbers
// isolate dest-side cost, not core or network time. Absolute
// deployment numbers ride the pinned-core-image leg at flip prep
// (D11).

// setupBenchServer builds a Handler over a stub core answering body
// for every route, so the setup legs and the relay legs share the
// same upstream shape.
func setupBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkSetupState measures the setup-state GET leg through the
// full production chain: keyless, no credential, one server-side
// fetch against the stub core.
func BenchmarkSetupState(b *testing.B) {
	_, srv := setupBenchServer(b, `{"needs_setup":true,"bootstrap_mode":"generated"}`)
	handler := srv.Handler(DefaultV0Sunset)

	state := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)

	b.Run("state", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, state.Clone(state.Context()))
		}
	})
}

// BenchmarkSetupWrite measures the setup claim POST leg through the
// full production chain: keyless with the opaque bootstrap-credential
// body, one server-side fetch against the stub core.
func BenchmarkSetupWrite(b *testing.B) {
	_, srv := setupBenchServer(b, `{"status":"claimed"}`)
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"username":"bootstrap-u","password":"bootstrap-p"}`
	claim := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim",
		strings.NewReader(payload))

	b.Run("claim", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, claim.Clone(claim.Context()))
		}
	})
}

// BenchmarkSetupRelayWrite measures the generic relay path for the
// same caller shape: the Compare POST, which stays on the generic
// relay by D6 disclosure (no pin covers its shared graph/content
// ports). The legs compare different routes by necessity (the setup
// paths left the relay); both do exactly one server-side fetch, so
// the 1.50 bound stays a real pathology check.
func BenchmarkSetupRelayWrite(b *testing.B) {
	_, srv := setupBenchServer(b, `{"status":"accepted"}`)
	handler := srv.Handler(DefaultV0Sunset)

	post := authedRequest(http.MethodPost, "/api/v0/compare/environments")

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, post.Clone(post.Context()))
		}
	})
}

// BenchmarkSetupRelayRead measures the generic relay path for the
// read shape: GET /api/v0/dependencies, which stays on the generic
// relay. Both legs do exactly one server-side fetch, so the 1.50
// bound stays a real pathology check.
func BenchmarkSetupRelayRead(b *testing.B) {
	_, srv := setupBenchServer(b, `{"dependencies":[]}`)
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

// TestSetupBenchmarks_ServeShapes pins that the benchmark legs above
// exercise the intended transports: the state and claim legs reach
// the stub core exactly once (public, no dest 401), and the relay
// legs reach it exactly once behind the key.
func TestSetupBenchmarks_ServeShapes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	state := httptest.NewRequest(http.MethodGet, "/api/v0/auth/setup-state", nil)
	stateRec := httptest.NewRecorder()
	handler.ServeHTTP(stateRec, state)
	if stateRec.Code != http.StatusOK {
		t.Errorf("setup-state status = %d, want 200", stateRec.Code)
	}

	claim := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim",
		strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
	claimRec := httptest.NewRecorder()
	handler.ServeHTTP(claimRec, claim)
	if claimRec.Code != http.StatusOK {
		t.Errorf("setup claim status = %d, want 200", claimRec.Code)
	}

	if core.hits != 2 {
		t.Errorf("upstream hits = %d, want 2 (one fetch per setup leg)", core.hits)
	}
}
