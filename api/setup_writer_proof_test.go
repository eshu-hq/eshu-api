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

// W7-writers setup trio termination proof (Refs eshu-hq/eshu#9669,
// D13 tracker eshu-hq/eshu#10491, satellite eshu-hq/eshu-api#114;
// core writer record eshu-hq/eshu#10531, SetupAdmission v0). TDD
// record: this suite failed RED before the terminated-writer filing
// landed (the trio forwarded with no dispatchOutcome on the
// dest-local answer), and is green with it. The three pre-auth
// first-run wizard POSTs terminate natively as writers: exactly one
// forward with opaque credential bodies, core bytes relayed
// byte-for-byte, and the dest-local answer (unconfigured origin)
// filed on the native lane. The public posture is invariant — no
// gate, no authorizer, no shared-key injection — so every trust
// decision stays core-side exactly as on the tier-C lane the trio
// rides today.

// setupTrioSpellings returns every native spelling of the three
// wizard credential POSTs: each route as registered plus its
// trailing-slash twin.
func setupTrioSpellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	for _, route := range setupMutationRoutes {
		method, path, _ := strings.Cut(route, " ")
		out = append(out, struct {
			method string
			path   string
		}{method, path}, struct {
			method string
			path   string
		}{method, path + "/"})
	}
	return out
}

// TestSetupWriter_NilCore503Filed pins the fail-closed origin on the
// writer path: with no core origin configured every trio spelling
// answers the permanent 503 backend_unavailable with no Retry-After,
// and files the local_503 outcome on the native lane. The stub core
// stays uncontacted by construction (nil origin), and the test pins
// its hit counter at zero to close the zero-upstream-hits box on
// every dest-local answer.
func TestSetupWriter_NilCore503Filed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"claimed"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer("", skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range setupTrioSpellings() {
		req := httptest.NewRequest(spelling.method, spelling.path,
			strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: status = %d, want 503", spelling.method, spelling.path, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("%s %s: Retry-After = %q, want absent (permanent 503)", spelling.method, spelling.path, got)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "backend_unavailable") {
			t.Errorf("%s %s: body = %q, want backend_unavailable", spelling.method, spelling.path, body)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (unconfigured origin never dials)", core.hits)
	}
	snap := tel.snapshot()
	if got := snap.dispatches["setup"][viaNative][outcomeLocal503]; got != 6 {
		t.Errorf("family setup native local_503 = %d, want 6 (three routes times two spellings)", got)
	}
}

// TestSetupWriter_TerminatesAheadOfRelay pins that the trio is an
// explicit family registration ahead of the generic proxy: every
// trio spelling files its dispatch on the native lane (viaNative)
// with exactly one upstream hit each. This test is the
// swap-mutant detector for dispatch: if a family registration is
// removed, the leg falls back to the relay and files relay instead
// of native, and this test goes RED.
func TestSetupWriter_TerminatesAheadOfRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"claimed"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, spelling := range setupTrioSpellings() {
		req := httptest.NewRequest(spelling.method, spelling.path,
			strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200 from the stub core", spelling.method, spelling.path, rec.Code)
		}
	}
	if core.hits != 6 {
		t.Fatalf("upstream hits = %d, want 6 (three routes times two spellings, one forward each)", core.hits)
	}
	snap := tel.snapshot()
	lanes, ok := snap.dispatches["setup"]
	if !ok {
		t.Fatal("family setup has no dispatch rows, want the explicit family routes ahead of relay")
	}
	if lanes[viaNative] == nil {
		t.Errorf("family setup dispatches = %v, want a native lane row for the writer trio", lanes)
	}
	if got := lanes[viaNative][outcomeCoreOK]; got != 6 {
		t.Errorf("family setup native core_ok = %d, want 6", got)
	}
}

// TestSetupWriter_MatchesRelayByteForByte pins that the writer trio
// relays core bytes exactly like the generic proxy: same status,
// same body, same relayed headers — across the pinned outcome matrix
// in both negotiated shapes. The stub answers the pinned wire
// strings where core owns the text, and opaque bytes where the relay
// (not the text) is what this test pins. The caller credential is
// the shared key on both legs: dest never injects it on this family
// (see TestSetupNative_NoSharedKeyInjection), so both legs forward
// it verbatim and the differential isolates the forward path.
func TestSetupWriter_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	const sealedBody = `{"error_code":"gone","message":"first-run setup is no longer available; this instance already has an identity","correlation_id":"seal-1"}`
	mfaHeader := http.Header{}
	mfaHeader.Set("Set-Cookie", "eshu_local_session=opaque-session; Path=/; HttpOnly")

	cases := []struct {
		name    string
		path    string
		reqBody string
		status  int
		header  http.Header
		body    string
	}{
		{
			"claim 200", "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 200, nil, `{"status":"claimed"}`,
		},
		{
			"claim denied 401", "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"wrong"}`, 401, nil, `{"error_code":"unauthenticated","message":"bootstrap credential is invalid"}`,
		},
		{
			"admin 200", "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`, 200, nil,
			`{"status":"admin_created","tenant_id":"t","workspace_id":"w"}`,
		},
		{
			"admin password floor 400", "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"short"}`, 400, nil,
			`{"error":"Bad Request","detail":"new_password must be at least 12 characters"}`,
		},
		{
			"mfa winner 200 with codes", "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 200, mfaHeader,
			`{"status":"completed","recovery_codes":["r1","r2"],"auth":{}}`,
		},
		{
			"mfa concurrent loser 409", "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 409, nil,
			`{"error_code":"conflict","message":"setup was already completed by a concurrent request"}`,
		},
		{
			"sealed claim 410", "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 410, nil, sealedBody,
		},
		{
			"sealed admin 410", "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`, 410, nil, sealedBody,
		},
		{
			"sealed mfa 410", "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 410, nil, sealedBody,
		},
		{
			"core 500", "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 500, nil,
			`{"error":"Internal Server Error","detail":"setup claim failed"}`,
		},
		{
			"core 503 permanent", "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`, 503, nil,
			`{"error":"Service Unavailable","detail":"setup store is unavailable"}`,
		},
		{
			"core 503 retryable", "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 503,
			http.Header{"Retry-After": []string{"5"}},
			`{"error":"Service Unavailable","detail":"setup store is fenced"}`,
		},
		{
			"redirect unfollowed", "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 307,
			http.Header{"Location": []string{"/console"}},
			"",
		},
	}
	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			name := "POST " + tc.path + " " + tc.name + " " + accept
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				nativeCore := &stubCore{status: tc.status, header: tc.header, body: tc.body}
				nativeMux := http.NewServeMux()
				nativeMux.Handle("/", nativeCore.handler())
				nativeUpstream := httptest.NewServer(nativeMux)
				defer nativeUpstream.Close()

				relayCore := &stubCore{status: tc.status, header: relayHeaders(tc.header), body: tc.body}
				relayMux := http.NewServeMux()
				relayMux.Handle("/", relayCore.handler())
				relayUpstream := httptest.NewServer(relayMux)
				defer relayUpstream.Close()

				build := func(target string) *http.Request {
					req := authedRequest(http.MethodPost, target)
					req.Body = io.NopCloser(strings.NewReader(tc.reqBody))
					req.Header.Set("Accept", accept)
					req.Header.Set("X-Correlation-ID", "w7write-parity-1")
					return req
				}

				nativeSrv := NewServer(nativeUpstream.URL, skeletonTestKey)
				nativeRec := httptest.NewRecorder()
				nativeSrv.Handler(DefaultV0Sunset).ServeHTTP(nativeRec, build(tc.path))

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				relayRec := httptest.NewRecorder()
				relaySrv.instrumented(viaRelay, RequireAuth(skeletonTestKey, http.HandlerFunc(relaySrv.dispatch))).ServeHTTP(relayRec, build(tc.path))

				nativeRes, relayRes := nativeRec.Result(), relayRec.Result()
				if nativeRes.StatusCode != relayRes.StatusCode {
					t.Fatalf("native status = %d, relay status = %d, want identical", nativeRes.StatusCode, relayRes.StatusCode)
				}
				nativeBody, _ := io.ReadAll(nativeRes.Body)
				relayBody, _ := io.ReadAll(relayRes.Body)
				if string(nativeBody) != string(relayBody) {
					t.Errorf("native body = %q, relay body = %q, want identical", nativeBody, relayBody)
				}
				for _, header := range []string{"Content-Type", "Set-Cookie", "Retry-After", "Location", "Cache-Control", "X-Correlation-ID"} {
					if nativeRes.Header.Get(header) != relayRes.Header.Get(header) {
						t.Errorf("header %s: native = %q, relay = %q, want identical",
							header, nativeRes.Header.Get(header), relayRes.Header.Get(header))
					}
				}
				if nativeCore.hits != 1 || relayCore.hits != 1 {
					t.Errorf("upstream hits native = %d relay = %d, want exactly 1 each (no retry on terminated writers)",
						nativeCore.hits, relayCore.hits)
				}
				if nativeCore.lastAuth == "" || nativeCore.lastAuth != relayCore.lastAuth {
					t.Errorf("upstream Authorization native = %q relay = %q, want the identical forwarded credential",
						nativeCore.lastAuth, relayCore.lastAuth)
				}
			})
		}
	}
}
