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
// config-influence, exposure, and the six rest legs), the
// scoped-empty-grant refusal (a scoped bearer outside the grant relays
// explicitly and the core refusal packet crosses untouched), and the
// two adapter holds (they relay explicitly on the relay lane, never
// the catch-all, per the hold TODO in impact_wave.go).

// TestImpactByteParity pins the forward contract: shared-key calls on
// the proceed legs terminate natively with core bytes untouched, and
// every byte the caller sent reaches the upstream intact.
func TestImpactByteParity(t *testing.T) {
	t.Parallel()

	for _, leg := range impactParityLegs {
		if impactWaveEntryFor(t, leg.key).Serve != impactServeGated {
			continue
		}
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
		if impactWaveEntryFor(t, leg.key).Serve != impactServeGated {
			continue
		}
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

// TestImpactAdapterHoldsStayRelayed pins the two adapter holds: the
// change-surface legs (the only relay_hold rows) never terminate —
// even a shared-key call relays explicitly on the relay lane with core
// bytes untouched, until the TODO-eshu-9669-w5 seam lands. Foreign
// credential classes keep the pre-lane key-gate posture (dest 401,
// zero core contact).
func TestImpactAdapterHoldsStayRelayed(t *testing.T) {
	t.Parallel()

	holds := []struct {
		target string
		body   string
	}{
		{"/api/v0/impact/change-surface", `{"repo_id":"repo_1","relative_paths":["src/a.go"]}`},
		{"/api/v0/impact/change-surface/investigate", `{"repo_id":"repo_1","topic":"auth refresh"}`},
	}
	for _, leg := range holds {
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
					t.Fatalf("status = %d, want 200 from the relayed core", rec.Code)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != coreBody {
					t.Errorf("body = %q, want the untouched core bytes %q", got, coreBody)
				}
				snap := tel.snapshot()
				if snap.dispatches["impact"][viaNative] != nil {
					t.Errorf("impact dispatches = %v, want no native lane row (legs hold on relay)", snap.dispatches["impact"])
				}
				if got := snap.dispatches["impact"][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("impact relay core_ok = %d, want 1", got)
				}

				scoped := httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				scoped.Header.Set("Authorization", "Bearer v2.scoped.token")
				scoped.Header.Set("Accept", accept)
				rec = httptest.NewRecorder()
				handler.ServeHTTP(rec, scoped)

				if rec.Code != http.StatusUnauthorized {
					t.Errorf("scoped status = %d, want the standing dest 401 (pre-lane key-gate posture)", rec.Code)
				}
			})
		}
	}
}
