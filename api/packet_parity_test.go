// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Supply-chain impact packet byte-parity proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#97), closing the Decision 2 obligation in
// this lane. The terminated reader is a byte-parity forward: the
// caller's method, path, query, negotiation headers, credential, and
// correlation ID cross intact and core bytes come back unmodified, in
// both negotiated shapes. The corpus below is the owed 7-case
// suppression set: the active winner, the suppressed winner, the
// expired-suppression winner, the ambiguous scope, the no-evidence
// scope, the scoped-empty-grant scope-refusal packet, and the
// unconfigured-origin 503.

// packetDecision2Cases is the Decision 2 parity corpus: one core
// packet shape per suppression state. The shared-key cases assert
// byte-identical native termination; the refusal case asserts
// byte-identical explicit relay for a scoped bearer outside the grant;
// the unconfigured case asserts the permanent local 503.
var packetDecision2Cases = []struct {
	name       string
	status     int
	body       string
	scoped     bool
	unconfig   bool
	wantStatus int
	wantVia    string
}{
	{name: "active winner", status: http.StatusOK, body: `{"packet":{"winner":{"finding_id":"f-1","state":"active"}}}`},
	{name: "suppressed winner", status: http.StatusOK, body: `{"packet":{"winner":{"finding_id":"f-2","state":"suppressed"}}}`},
	{name: "expired-suppression winner", status: http.StatusOK, body: `{"packet":{"winner":{"finding_id":"f-3","state":"active","suppression_expired":true}}}`},
	{name: "ambiguous scope", status: http.StatusOK, body: `{"packet":{"ambiguous":true,"explanation":"multiple scopes match"}}`},
	{name: "no-evidence scope", status: http.StatusOK, body: `{"packet":{"no_evidence":true,"explanation":"no findings in scope"}}`},
	{name: "scoped-empty-grant refusal", status: http.StatusForbidden, body: `{"error":"forbidden","error_detail":"scope outside the caller grant","truth":null}`, scoped: true, wantStatus: http.StatusForbidden, wantVia: viaRelay},
	{name: "unconfigured origin", unconfig: true, wantStatus: http.StatusServiceUnavailable, wantVia: viaNative},
}

// TestPacketDecision2Corpus pins the 7-case Decision 2 parity set: the
// five shared-key packet shapes terminate natively byte-identical, the
// scoped-empty-grant refusal relays explicitly byte-identical, and the
// unconfigured origin answers the permanent local 503 (never
// Retry-After) — each in both negotiated shapes.
func TestPacketDecision2Corpus(t *testing.T) {
	t.Parallel()

	for _, tc := range packetDecision2Cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := tc.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				wantStatus := http.StatusOK
				if tc.wantStatus != 0 {
					wantStatus = tc.wantStatus
				}
				wantVia := viaNative
				if tc.wantVia != "" {
					wantVia = tc.wantVia
				}

				if tc.unconfig {
					srv := NewServer("", skeletonTestKey)
					req := authedRequest(http.MethodGet, packetConcreteTarget)
					req.Header.Set("Accept", accept)
					rec := httptest.NewRecorder()
					srv.servePacketTerminatedAs(rec, req, true)

					res := rec.Result()
					if res.StatusCode != http.StatusServiceUnavailable {
						t.Fatalf("status = %d, want the permanent local 503", res.StatusCode)
					}
					if res.Header.Get("Retry-After") != "" {
						t.Errorf("Retry-After = %q, want empty (local 503 never carries it)", res.Header.Get("Retry-After"))
					}
					return
				}

				core := &stubCore{status: tc.status, body: tc.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)
				defer upstream.Close()

				tel := NewTelemetry(nil)
				srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
				handler := srv.Handler(DefaultV0Sunset)

				var req *http.Request
				if tc.scoped {
					req = httptest.NewRequest(http.MethodGet, packetConcreteTarget, nil)
					req.Header.Set("Authorization", "Bearer v2.scoped.empty-grant")
				} else {
					req = authedRequest(http.MethodGet, packetConcreteTarget)
				}
				req.Header.Set("Accept", accept)
				req.Header.Set("X-Correlation-ID", "packet-parity-1")

				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)

				if rec.Code != wantStatus {
					t.Fatalf("status = %d, want %d from the core shape", rec.Code, wantStatus)
				}
				got, _ := io.ReadAll(rec.Result().Body)
				if string(got) != tc.body {
					t.Errorf("body = %q, want the untouched core bytes %q", got, tc.body)
				}
				if !tc.scoped && rec.Header().Get("X-Correlation-ID") != "packet-parity-1" {
					t.Errorf("correlation = %q, want the caller value echoed", rec.Header().Get("X-Correlation-ID"))
				}
				if core.hits != 1 {
					t.Errorf("upstream hits = %d, want exactly 1 (one server-side fetch, no retry)", core.hits)
				}
				if got := tel.snapshot().dispatches["supply_chain"][wantVia][outcomeCoreOK]; got != 1 {
					t.Errorf("supply_chain %s core_ok = %d, want 1", wantVia, got)
				}
			})
		}
	}
}

// TestPacketByteParity pins the forward contract beyond the Decision 2
// set: the caller query, Accept, credential, and correlation ID cross
// intact on the native lane.
func TestPacketByteParity(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		t.Run(accept, func(t *testing.T) {
			t.Parallel()

			coreBody := `{"packet":{"winner":{"finding_id":"f-1","state":"active"}}}`
			core := &stubCore{status: http.StatusOK, body: coreBody}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodGet, packetConcreteTarget)
			req.Header.Set("Accept", accept)
			req.Header.Set("X-Correlation-ID", "packet-parity-1")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastQuery != "scope_id=scope_1" {
				t.Errorf("upstream query = %q, want the caller query preserved", core.lastQuery)
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
