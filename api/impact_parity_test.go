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

// Impact byte-parity proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The terminated reader is a byte-parity
// forward: the caller's method, path, query, body, negotiation
// headers, credential, and correlation ID cross intact and core bytes
// come back unmodified — for every proceed leg, in both negotiated
// shapes. The corpus below covers the representative request shapes
// per leg (service/workload trace bodies, blast-radius, contracts,
// config-influence, exposure, the six rest legs, and the two
// change-surface legs), and the scoped-empty-grant refusal (a scoped
// bearer outside the grant relays explicitly and the core refusal
// packet crosses untouched).

// TestImpactByteParity pins the forward contract: shared-key calls on
// the proceed legs terminate natively with core bytes untouched, and
// every byte the caller sent reaches the upstream intact.
func TestImpactByteParity(t *testing.T) {
	t.Parallel()

	for _, leg := range impactParityLegs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.target + " " + leg.body + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				coreBody := `{"data":[],"error":null,"truth":null}`
				core := &stubCore{status: http.StatusOK, body: coreBody}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				req := authedRequest(http.MethodPost, leg.target)
				req.Body = io.NopCloser(strings.NewReader(leg.body))
				req.ContentLength = int64(len(leg.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", accept)
				req.Header.Set("X-Correlation-ID", "impact-parity-1")

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != coreBody {
					t.Errorf("body = %q, want the untouched core bytes %q", got, coreBody)
				}
				if rec.Header().Get("X-Correlation-ID") != "impact-parity-1" {
					t.Errorf("correlation = %q, want the caller value echoed", rec.Header().Get("X-Correlation-ID"))
				}
				if core.lastMethod != http.MethodPost {
					t.Errorf("upstream method = %q, want POST", core.lastMethod)
				}
				if core.lastBody != leg.body {
					t.Errorf("upstream body = %q, want the caller body %q forwarded intact", core.lastBody, leg.body)
				}
				if core.lastAccept != accept {
					t.Errorf("upstream accept = %q, want the negotiated shape %q", core.lastAccept, accept)
				}
				if core.lastAuth != "Bearer "+skeletonTestKey {
					t.Errorf("upstream auth = %q, want the shared-key credential forwarded", core.lastAuth)
				}
			})
		}
	}
}

// TestImpactScopedEmptyGrantRelay pins the scoped-empty-grant leg of
// the parity corpus: a scoped bearer outside the grant never
// terminates — it relays explicitly on the relay lane and the core
// refusal packet (scoped 403) crosses byte-identical, in both
// negotiated shapes.
func TestImpactScopedEmptyGrantRelay(t *testing.T) {
	t.Parallel()

	refusal := `{"error":"forbidden","error_detail":"grant does not cover this service","truth":null}`
	for _, leg := range impactParityLegs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusForbidden, body: refusal}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				req.Header.Set("Authorization", "Bearer v2.scoped.empty-grant")
				req.Header.Set("Accept", accept)

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if core.hits != 1 {
					t.Fatalf("upstream hits = %d, want 1 (relay explicitly, never gate-invent)", core.hits)
				}
				if rec.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want the relayed core 403 (never invented)", rec.Code)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != refusal {
					t.Errorf("body = %q, want the untouched refusal packet %q", got, refusal)
				}
				if got := tel.snapshot().dispatches["impact"][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("impact relay core_ok = %d, want 1", got)
				}
			})
		}
	}
}

// TestImpactAdapterTerminatedNatively pins the Wave-3 termination:
// both change-surface legs (gated rows) terminate natively for a
// shared-key call on the native lane with core bytes untouched. Every
// leg read (target resolve + traversal via Neo4j.Run, code-topic and
// path-symbol reads via the W5 impact pin) executes core-side on the
// forwarded call, so no new pin is owed: the legs ride the existing
// 14-method ImpactReadAdmission plus the forward transport.
func TestImpactAdapterTerminatedNatively(t *testing.T) {
	t.Parallel()

	legs := []struct {
		target string
		body   string
	}{
		{"/api/v0/impact/change-surface", `{"repo_id":"repo_1","relative_paths":["src/a.go"]}`},
		{"/api/v0/impact/change-surface/investigate", `{"repo_id":"repo_1","topic":"auth refresh"}`},
	}
	for _, leg := range legs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				coreBody := `{"data":[],"error":null,"truth":null}`
				core := &stubCore{status: http.StatusOK, body: coreBody}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				req := authedRequest(http.MethodPost, leg.target)
				req.Body = io.NopCloser(strings.NewReader(leg.body))
				req.ContentLength = int64(len(leg.body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Accept", accept)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the native core", rec.Code)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != coreBody {
					t.Errorf("body = %q, want the untouched core bytes %q", got, coreBody)
				}
				snap := tel.snapshot()
				if snap.dispatches["impact"][viaRelay] != nil {
					t.Errorf("impact dispatches = %v, want no relay lane row (legs terminate natively)", snap.dispatches["impact"])
				}
				if got := snap.dispatches["impact"][viaNative][outcomeCoreOK]; got != 1 {
					t.Errorf("impact native core_ok = %d, want 1", got)
				}

				scoped := httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				scoped.Header.Set("Authorization", "Bearer [REDACTED]")
				scoped.Header.Set("Accept", accept)
				rec = httptest.NewRecorder()
				handler.ServeHTTP(rec, scoped)

				if rec.Code != http.StatusOK {
					t.Fatalf("scoped status = %d, want core's 200 on the explicit relay lane", rec.Code)
				}
				if got := tel.snapshot().dispatches["impact"][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("impact relay core_ok = %d, want 1 (foreign class relays explicitly)", got)
				}
			})
		}
	}
}
