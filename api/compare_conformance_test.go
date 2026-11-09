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

// Compare relay-leg termination conformance (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, gap 04). TDD record: the frozen wave
// below failed RED before the implementation landed (compareWave
// undefined), and the ahead-of-relay assertions failed next (the
// Compare POST still crossed the generic relay); both are green with
// the wave gate registered ahead of the generic proxy. Until the lane
// lands, every Compare call crosses the generic byte-relay (or the
// key gate), so the ahead-of-relay assertions fail first. That red is
// the recorded expectation, not a defect.

// TestCompareWaveFrozen pins the exact surface the satellite
// terminates: the single Compare POST the gap-04 deliberate later pin
// (CompareReadAdmission v0, eshu-hq/eshu#10425) covers. A wave change
// moves the ownership note, the core-side fragment reference, and
// this mirror together — the same frozen-set pattern as
// TestD6WaveFrozen.
func TestCompareWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/compare/environments compare gated go/internal/query/openapi/paths/status/compare.go",
	}
	var got []string
	for _, entry := range compareWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("compareWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("compareWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// compareConcretePath is the mux match for the single wave entry. The
// trailing-slash spelling rides the same native registration
// (never-split), pinned by TestCompareSlashSpellingTerminatesNatively.
const compareConcretePath = "/api/v0/compare/environments"

// compareParityBody is the request body the behavior tests drive
// through the lane: a well-formed compare request with the three
// required keys the served fragment names.
const compareParityBody = `{"workload_id":"w1","left":"prod","right":"staging"}`

// compareAuthedPost builds a shared-key Compare POST with the parity
// body, the negotiated shape, and a pinned correlation ID.
func compareAuthedPost(accept string) *http.Request {
	req := authedRequest(http.MethodPost, compareConcretePath)
	req.Body = io.NopCloser(strings.NewReader(compareParityBody))
	req.ContentLength = int64(len(compareParityBody))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "compare-parity-1")
	return req
}

// TestCompareWaveAheadOfRelay pins that the wave route is an explicit
// registration ahead of the generic proxy: a shared-key call must
// file its dispatch on the native lane (viaNative), not the relay
// lane. Until the lane lands, the call crosses the generic relay and
// this test is RED.
func TestCompareWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"workload":"w1","story":"same"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, compareAuthedPost("application/json"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.lastBody != compareParityBody {
		t.Errorf("upstream body = %q, want the compare request %q forwarded intact", core.lastBody, compareParityBody)
	}
	snap := tel.snapshot()
	lanes, ok := snap.dispatches["compare"]
	if !ok {
		t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", "compare")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", "compare", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("family %q native core_ok = %d, want 1", "compare", got)
	}
}

// TestCompareSlashSpellingTerminatesNatively pins the never-split
// rule for the wave: the trailing-slash spelling terminates on the
// native lane exactly like the bare spelling.
func TestCompareSlashSpellingTerminatesNatively(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"workload":"w1","story":"same"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, compareConcretePath+"/")
	req.Body = io.NopCloser(strings.NewReader(compareParityBody))
	req.ContentLength = int64(len(compareParityBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST %s/: status = %d, want 200 from the stub core", compareConcretePath, rec.Code)
	}
	snap := tel.snapshot()
	if got := snap.dispatches["compare"][viaNative][outcomeCoreOK]; got != 1 {
		t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", "compare", got)
	}
}

// TestCompareTerminatedMatchesRelayByteForByte pins that the
// terminated Compare reader relays core bytes exactly like the
// generic proxy: same status, same headers (modulo per-hop framing),
// same body — across the pinned outcome matrix in both negotiated
// shapes — and forwards method, path, query, and the request body
// intact. This guards the dest-owned response mapping
// (forwardToCore on the native lane) against future divergence from
// the relay contract it mirrors. The matrix covers the fragment's
// documented outcomes: the retryable 503 with Retry-After (this
// route has no configuration 503, so every 503 carries it), the
// 504 graph-deadline read, the fixed-message 500, and the standard
// 400/403/404 denials.
func TestCompareTerminatedMatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	outcomes := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"plain 200", http.StatusOK, http.Header{"Content-Type": []string{"application/json; charset=utf-8"}}, `{"workload":"w1","story":"same"}`},
		{"validation 400", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"workload_id is required"}`},
		{"core all-scope 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"all-scope admin authentication is required"}`},
		{"workload grant 404", http.StatusNotFound, nil, `{"error":"Not Found","detail":"workload not found"}`},
		{"fixed-message 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"workload read failed"}`},
		{"graph deadline 504", http.StatusGatewayTimeout, nil, `{"error":"Gateway Timeout","detail":"graph read exceeded its deadline"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"graph backend is unavailable"}`},
		{"redirect unfollowed", http.StatusTemporaryRedirect, http.Header{"Location": []string{"/api/v0/auth/oidc/login"}}, ""},
	}
	for _, outcome := range outcomes {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + compareConcretePath + " " + outcome.name + " " + accept
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
				nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, compareAuthedPost(accept))

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				relayRec := httptest.NewRecorder()
				// The relay side crosses the same middleware chain
				// the router builds for the generic proxy
				// (instrumented + key gate), so correlation echo,
				// deprecation headers, and lane filing match the
				// production relay path exactly.
				relayReq := compareAuthedPost(accept)
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
