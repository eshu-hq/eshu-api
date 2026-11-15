// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Repository story byte-parity proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97). The terminated reader is a
// byte-parity forward: the caller's method, path, negotiation headers,
// credential, and correlation ID cross intact and core bytes come back
// unmodified, in both negotiated shapes. The corpus below covers the
// shared-key happy path, the scoped-empty-grant refusal (a scoped
// bearer outside the grant relays explicitly and the core refusal
// packet crosses untouched), and the retired-relay contrast (the old
// D4 relay leg now terminates — the superseded relay posture is gone).

// TestStoryByteParity pins the forward contract: a shared-key story
// GET terminates natively with core bytes untouched, and every byte
// the caller sent reaches the upstream intact.
func TestStoryByteParity(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			coreBody := `{"story":"repo narrative","evidence_boundaries":null,"truth":null}`
			core := &stubCore{status: http.StatusOK, body: coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, storyConcretePath)
			req.Header.Set("Accept", accept)
			req.Header.Set("X-Correlation-ID", "story-parity-1")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			got, _ := io.ReadAll(rec.Result().Body)
			if string(got) != coreBody {
				t.Errorf("body = %q, want the untouched core bytes %q", got, coreBody)
			}
			if rec.Header().Get("X-Correlation-ID") != "story-parity-1" {
				t.Errorf("correlation = %q, want the caller value echoed", rec.Header().Get("X-Correlation-ID"))
			}
			if core.lastPath != storyConcretePath {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, storyConcretePath)
			}
			if core.lastAccept != accept {
				t.Errorf("upstream accept = %q, want the negotiated shape %q", core.lastAccept, accept)
			}
			if core.lastAuth != "Bearer "+skeletonTestKey {
				t.Errorf("upstream auth = %q, want the shared-key credential forwarded", core.lastAuth)
			}
			if core.hits != 1 {
				t.Errorf("upstream hits = %d, want exactly 1 (one server-side fetch, no retry)", core.hits)
			}
		})
	}
}

// TestStoryScopedEmptyGrantRelay pins the scoped-empty-grant leg of
// the parity corpus: a scoped bearer outside the grant never
// terminates — it relays explicitly on the relay lane and the core
// refusal packet crosses byte-identical, in both negotiated shapes.
func TestStoryScopedEmptyGrantRelay(t *testing.T) {
	t.Parallel()

	refusal := `{"error":"not_found","error_detail":"repository outside the caller grant","truth":null}`
	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusNotFound, body: refusal}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, storyConcretePath, nil)
			req.Header.Set("Authorization", "Bearer v2.scoped.empty-grant")
			req.Header.Set("Accept", accept)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want 1 (relay explicitly, never gate-invent)", core.hits)
			}
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want the relayed core 404 (never invented)", rec.Code)
			}
			got, _ := io.ReadAll(rec.Result().Body)
			if string(got) != refusal {
				t.Errorf("body = %q, want the untouched refusal packet %q", got, refusal)
			}
			if got := tel.snapshot().dispatches["repositories"][viaRelay][outcomeCoreOK]; got != 1 {
				t.Errorf("repositories relay core_ok = %d, want 1", got)
			}
		})
	}
}
