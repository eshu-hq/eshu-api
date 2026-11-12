// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// Code-flow relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#98, W6a). TDD record: the frozen wave
// failed RED before the implementation landed (codeFlowWave
// undefined), and the ahead-of-relay assertions failed next (the
// Code-flow POSTs still crossed the generic relay); both are green
// with the wave gate registered ahead of the generic proxy. Until the
// lane lands, every Code-flow call crosses the generic byte-relay (or
// the key gate), so the ahead-of-relay assertions fail first.

// TestCodeFlowWaveFrozen pins the exact surface the satellite
// terminates: the four Code-flow POSTs the W6a deliberate pin
// (CodeFlowReadAdmission v0, eshu-hq/eshu#10552) covers. A wave change
// moves the ownership note, the core-side fragment reference, and
// this mirror together — the same frozen-set pattern as
// TestCompareWaveFrozen.
func TestCodeFlowWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/flow/cfg-summary code_flow gated go/internal/query/openapi/paths/code/flow.go",
		"POST /api/v0/code/flow/pdg-summary code_flow gated go/internal/query/openapi/paths/code/flow.go",
		"POST /api/v0/code/flow/reaching-def code_flow gated go/internal/query/openapi/paths/code/flow.go",
		"POST /api/v0/code/flow/taint-path code_flow gated go/internal/query/openapi/paths/code/flow.go",
	}
	var got []string
	for _, entry := range codeFlowWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeFlowWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeFlowWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// codeFlowConcretePaths lists one mux match per wave entry. Each
// trailing-slash spelling rides the same native registration
// (never-split), pinned by
// TestCodeFlowSlashSpellingsTerminateNatively.
var codeFlowConcretePaths = []string{
	"/api/v0/code/flow/cfg-summary",
	"/api/v0/code/flow/pdg-summary",
	"/api/v0/code/flow/reaching-def",
	"/api/v0/code/flow/taint-path",
}

// codeFlowParityBody is the request body the behavior tests drive
// through the lane: a minimal Code-flow read with the required
// repo_id the served fragment names.
const codeFlowParityBody = `{"repo_id":"r1"}`

// codeFlowAuthedPost builds a shared-key Code-flow POST with the
// parity body, the negotiated shape, and a pinned correlation ID.
func codeFlowAuthedPost(path, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(codeFlowParityBody))
	req.ContentLength = int64(len(codeFlowParityBody))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "code-flow-parity-1")
	return req
}

// TestCodeFlowWaveAheadOfRelay pins that the wave routes are explicit
// registrations ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, the calls cross the generic relay and
// this test is RED.
func TestCodeFlowWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, path := range codeFlowConcretePaths {
		t.Run("POST "+path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"truth":{"coverage":"exact"}}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeFlowAuthedPost(path, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastBody != codeFlowParityBody {
				t.Errorf("upstream body = %q, want the code-flow request %q forwarded intact", core.lastBody, codeFlowParityBody)
			}
			if core.lastPath != path {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, path)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches["code_flow"]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "code_flow")
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "code_flow", lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", "code_flow", got)
			}
		})
	}
}

// TestCodeFlowSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: each trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeFlowSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, path := range codeFlowConcretePaths {
		t.Run("POST "+path+"/", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"data":[],"truth":{"coverage":"exact"}}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeFlowAuthedPost(path+"/", "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches["code_flow"][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", "code_flow", got)
			}
		})
	}
}

// TestCodeFlowTerminatedMatchesRelayByteForByte pins byte parity
// between the terminated wave lane and the generic relay for every
// terminated route: status, headers, and body cross identically in
// both negotiated shapes. The stub core answers each documented
// Code-flow outcome (200 bounded evidence, 400 repo-required and
// malformed, core 403 grant-denied, 501 unsupported profile, fixed
// 500 store-read failure, 503 store unavailable, graph-deadline 504,
// retryable 503 with Retry-After, unfollowed redirect), so the lane
// proves parity on the error body/code/message surface the arbiter
// bar names without inventing any handler behavior dest-side.
func TestCodeFlowTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"data":[],"truth":{"coverage":"exact"}}`},
		{"validation 400 repo-required", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"repo_id is required"}`},
		{"validation 400 malformed", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"unexpected end of JSON input"}`},
		{"core grant 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"unsupported profile 501", http.StatusNotImplemented, nil, `{"error":"Not Implemented","detail":"code-flow reads require a supported query profile"}`},
		{"fixed-message 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"code-flow read failed"}`},
		{"store unavailable 503", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"code-flow store is unavailable"}`},
		{"graph deadline 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"graph read exceeded its deadline"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, path := range codeFlowConcretePaths {
		for _, outcome := range outcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				name := "POST " + path + " " + outcome.name + " " + accept
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					nativeCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
					nativeMux := http.NewServeMux()
					nativeMux.Handle("/", nativeCore.handler())
					nativeUpstream := httptest.NewServer(nativeMux)
					defer nativeUpstream.Close()

					relayCore := &stubCore{status: outcome.status, header: relayHeaders(outcome.header), body: outcome.body}
					relayMux := http.NewServeMux()
					relayMux.Handle("/", relayCore.handler())
					relayUpstream := httptest.NewServer(relayMux)
					defer relayUpstream.Close()

					nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
					nativeRec := httptest.NewRecorder()
					nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, codeFlowAuthedPost(path, accept))

					relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
					relayRec := httptest.NewRecorder()
					relayReq := codeFlowAuthedPost(path, accept)
					relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, relayReq)

					nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
					if nativeRes.StatusCode != relayRes.StatusCode {
						t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
					}
					nativeBody, _ := io.ReadAll(nativeRes.Body)
					relayBody, _ := io.ReadAll(relayRes.Body)
					if string(nativeBody) != string(relayBody) {
						t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
					}
					for _, header := range []string{"Content-Type", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
						if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
							t.Errorf("header %s: native = %q, relay = %q, want identical", header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
						}
					}
					for _, field := range []struct {
						name   string
						native string
						relay  string
					}{
						{"method", nativeCore.lastMethod, relayCore.lastMethod},
						{"path", nativeCore.lastPath, relayCore.lastPath},
						{"query", nativeCore.lastQuery, relayCore.lastQuery},
						{"body", nativeCore.lastBody, relayCore.lastBody},
					} {
						if field.native != field.relay {
							t.Errorf("upstream %s: native = %q, relay = %q, want identical", field.name, field.native, field.relay)
						}
					}
				})
			}
		}
	}
}
