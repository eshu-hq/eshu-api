// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-2 Content latency evidence (Refs eshu-hq/eshu#9669, D13
// tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#72). The
// terminated reader does the same single server-side fetch as the
// generic relay plus the wave gate and the family authorizer; these
// benchmarks bound that overhead per representative route so
// scripts/measure-content-latency.sh can gate it in CI. The stub core
// answers from memory — the numbers isolate dest-side cost, not core
// or network time. Absolute deployment numbers ride the pinned-core
// image leg at flip prep (D11).

// contentBenchServer builds a Handler over a stub core answering body
// for every route, so terminated and relay benchmarks share the same
// upstream shape.
func contentBenchServer(b *testing.B, body string) (*httptest.Server, *Server) {
	b.Helper()

	core := &stubCore{status: http.StatusOK, body: body}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	b.Cleanup(upstream.Close)

	return upstream, NewServer(upstream.URL, skeletonTestKey)
}

// BenchmarkContentTerminatedRead measures the terminated path for a
// shared-key POST through the full production chain: one leg per wave
// route through the same wave gate and family authorizer.
func BenchmarkContentTerminatedRead(b *testing.B) {
	_, srv := contentBenchServer(b, `{"repo_id":"repo-1"}`)
	handler := srv.Handler(DefaultV0Sunset)

	legs := map[string]struct {
		path string
		body string
	}{
		"files-read":    {"/api/v0/content/files/read", `{"repo_id":"repo-1","relative_path":"src/app.go"}`},
		"files-lines":   {"/api/v0/content/files/lines", `{"repo_id":"repo-1","relative_path":"src/app.go","start_line":2,"end_line":4}`},
		"entities-read": {"/api/v0/content/entities/read", `{"entity_id":"entity-1"}`},
	}
	for name, leg := range legs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, contentAuthedPost(leg.path, leg.body, "application/json"))
			}
		})
	}
}

// BenchmarkContentRelayRead measures the generic relay path for the
// same caller and stub shape: the before leg the lane must not
// regress. The legs compare different routes by necessity (the wave
// paths now terminate; the W4 search lane terminated the file search
// POST too, so the relay leg rides the still-relayed code bundles POST
// no wave table covers); both do exactly one server-side fetch, so the
// 1.50 bound stays a real pathology check.
func BenchmarkContentRelayRead(b *testing.B) {
	_, srv := contentBenchServer(b, `{"results":[]}`)
	handler := srv.Handler(DefaultV0Sunset)

	b.Run("post", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, contentAuthedPost("/api/v0/code/bundles", `{"query":"render"}`, "application/json"))
		}
	})
}

// TestContentRelayLegStaysRelayed proves the relay-leg premise the
// benchmarks above rest on: POST /api/v0/code/bundles is covered by no
// wave table, so a shared-key call files its dispatch on the relay
// lane — the terminated-vs-relay comparison stays honest.
func TestContentRelayLegStaysRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, contentAuthedPost("/api/v0/code/bundles", `{"query":"render"}`, "application/json"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.lastPath != "/api/v0/code/bundles" {
		t.Errorf("upstream path = %q, want the relay leg crossed intact", core.lastPath)
	}
	snap := tel.snapshot()
	for family, lanes := range snap.dispatches {
		if lanes[viaNative] != nil {
			t.Errorf("family %q files a native lane row, want the relay leg on the relay lane only", family)
		}
	}
	found := false
	for _, lanes := range snap.dispatches {
		if lanes[viaRelay] != nil {
			found = true
		}
	}
	if !found {
		t.Errorf("dispatches = %v, want a relay lane row for the relay leg", snap.dispatches)
	}
}
