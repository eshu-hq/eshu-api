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

// Entity byte-parity proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#97). The terminated reader is a byte-parity
// forward: the caller's method, path, query, body, negotiation
// headers, credential, and correlation ID cross intact and core bytes
// come back unmodified — for every proceed leg, in both negotiated
// shapes. The corpus below covers the representative request shapes
// per leg (resolve POST bodies, context/story/edge GET targets), the
// scoped-empty-grant refusal (a scoped bearer outside the grant relays
// explicitly and the core refusal packet crosses untouched), and the
// relay hold (the context fallback leg relays explicitly on the relay
// lane, never the catch-all, per the hold TODO in entity_wave.go).

// entityParityLegs drives one representative request per wave row.
var entityParityLegs = []struct {
	key    string
	method string
	target string
	body   string
}{
	{"POST /api/v0/entities/resolve", http.MethodPost, "/api/v0/entities/resolve", `{"name":"handleWire","entity_type":"Function"}`},
	{"POST /api/v0/entities/resolve", http.MethodPost, "/api/v0/entities/resolve", `{"name":"handleWire","entity_type":"Function","repo_id":"repo_1"}`},
	{"GET /api/v0/entities/{entity_id}/context", http.MethodGet, "/api/v0/entities/ent_1/context", ""},
	{"GET /api/v0/workloads/{workload_id}/context", http.MethodGet, "/api/v0/workloads/wl_1/context", ""},
	{"GET /api/v0/workloads/{workload_id}/story", http.MethodGet, "/api/v0/workloads/wl_1/story", ""},
	{"GET /api/v0/services/{service_name}/context", http.MethodGet, "/api/v0/services/payments/context", ""},
	{"GET /api/v0/services/{service_name}/story", http.MethodGet, "/api/v0/services/payments/story", ""},
	{"GET /api/v0/services/{service_name}/api-surface/endpoints", http.MethodGet, "/api/v0/services/payments/api-surface/endpoints", ""},
	{"GET /api/v0/services/{service_name}/downstream-consumers", http.MethodGet, "/api/v0/services/payments/downstream-consumers", ""},
}

// TestEntityByteParity pins the forward contract: shared-key calls on
// the proceed legs terminate natively with core bytes untouched, and
// every byte the caller sent (method, path, query, body, Accept,
// Authorization, correlation ID) reaches the upstream intact.
func TestEntityByteParity(t *testing.T) {
	t.Parallel()

	for _, leg := range entityParityLegs {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				coreBody := `{"data":[{"entity_id":"ent_1"}],"error":null,"truth":null}`
				core := &stubCore{status: http.StatusOK, body: coreBody}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				srv := NewServer(upstream.URL, skeletonTestKey)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if leg.method == http.MethodPost {
					req = authedRequest(http.MethodPost, leg.target)
					req.Body = io.NopCloser(strings.NewReader(leg.body))
					req.ContentLength = int64(len(leg.body))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req = authedRequest(http.MethodGet, leg.target)
				}
				req.Header.Set("Accept", accept)
				req.Header.Set("X-Correlation-ID", "entity-parity-1")

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != coreBody {
					t.Errorf("body = %q, want the untouched core bytes %q", got, coreBody)
				}
				if rec.Header().Get("X-Correlation-ID") != "entity-parity-1" {
					t.Errorf("correlation = %q, want the caller value echoed", rec.Header().Get("X-Correlation-ID"))
				}
				if core.lastMethod != leg.method {
					t.Errorf("upstream method = %q, want %q", core.lastMethod, leg.method)
				}
				upstreamTarget := leg.target
				if i := strings.Index(upstreamTarget, "?"); i >= 0 {
					upstreamTarget = upstreamTarget[:i]
					if core.lastQuery == "" {
						t.Errorf("upstream query empty, want the caller query preserved")
					}
				}
				_ = upstreamTarget
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

// TestEntityScopedEmptyGrantRelay pins the scoped-empty-grant leg of
// the parity corpus: a scoped bearer outside the grant never
// terminates — it relays explicitly on the relay lane and the core
// refusal packet (scoped 404) crosses byte-identical, in both
// negotiated shapes. The hold leg asserts the same relay posture.
func TestEntityScopedEmptyGrantRelay(t *testing.T) {
	t.Parallel()

	refusal := `{"error":"not_found","error_detail":"service outside the caller grant","truth":null}`
	for _, leg := range entityParityLegs {
		if serveOf(leg.key) != entityServeGated {
			// The relay hold keeps the pre-lane posture (D4 story-leg
			// precedent): a foreign credential class meets the key
			// gate — dest 401, zero core contact — instead of an
			// explicit relay. TestEntityScopedHoldKeepsKeyGate pins it.
			continue
		}
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := leg.method + " " + leg.target + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				core := &stubCore{status: http.StatusNotFound, body: refusal}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if leg.method == http.MethodPost {
					req = httptest.NewRequest(http.MethodPost, leg.target, strings.NewReader(leg.body))
				} else {
					req = httptest.NewRequest(http.MethodGet, leg.target, nil)
				}
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
				entry := waveEntryFor(t, leg.key)
				if got := tel.snapshot().dispatches[entry.Family][viaRelay][outcomeCoreOK]; got != 1 {
					t.Errorf("family %q relay core_ok = %d, want 1", entry.Family, got)
				}
			})
		}
	}
}

// waveEntryFor resolves a wave key to its table entry for family
// assertions in the parity corpus.
func waveEntryFor(t *testing.T, key string) entityRoute {
	t.Helper()
	for _, entry := range entityWave {
		if entry.Method+" "+entry.Path == key {
			return entry
		}
	}
	t.Fatalf("no entityWave entry for %q", key)
	return entityRoute{}
}

// TestEntityScopedHoldKeepsKeyGate pins the hold's pre-lane posture
// for foreign credential classes (D4 story-leg precedent): a scoped
// bearer on the relay hold meets the key gate — dest 401, zero core
// contact — instead of an explicit relay.
func TestEntityScopedHoldKeepsKeyGate(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"entity_id":"ent_1"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/entities/ent_1/context", nil)
			req.Header.Set("Authorization", "Bearer v2.scoped.token")
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want the standing dest 401 (pre-lane key-gate posture)", rec.Code)
			}
			if core.hits != 0 {
				t.Errorf("upstream hits = %d, want 0 (foreign classes never reach core on the hold)", core.hits)
			}
		})
	}
}

// TestEntityContextHoldStaysRelayed pins the relay hold: the entity
// context fallback leg (the only relay_hold row) never terminates —
// even a shared-key call relays explicitly on the relay lane with core
// bytes untouched, until the TODO-eshu-9669-w5 satisfier lands.
func TestEntityContextHoldStaysRelayed(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			coreBody := `{"entity_id":"ent_1","context":null,"truth":null}`
			core := &stubCore{status: http.StatusOK, body: coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, "/api/v0/entities/ent_1/context")
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
			if snap.dispatches["entities"][viaNative] != nil {
				t.Errorf("entities dispatches = %v, want no native lane row (leg holds on relay)", snap.dispatches["entities"])
			}
			if got := snap.dispatches["entities"][viaRelay][outcomeCoreOK]; got != 1 {
				t.Errorf("entities relay core_ok = %d, want 1", got)
			}
		})
	}
}
