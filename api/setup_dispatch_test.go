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

// setupNativeCases covers the four setup carve-out routes with
// concrete request paths (C-c1 APPROVED, graduated to D8-grade
// handling on the D10 wave): the terminating setup-state check and
// the three bootstrap-credential wizard steps. Method and path
// forward verbatim.
var setupNativeCases = []struct {
	name   string
	method string
	path   string
}{
	{"setup state", http.MethodGet, "/api/v0/auth/setup-state"},
	{"claim", http.MethodPost, "/api/v0/auth/setup/claim"},
	{"admin", http.MethodPost, "/api/v0/auth/setup/admin"},
	{"mfa", http.MethodPost, "/api/v0/auth/setup/mfa"},
}

// setupSpellings returns every native spelling: each route as
// registered plus its trailing-slash twin.
func setupSpellings() []struct {
	method string
	path   string
} {
	var out []struct {
		method string
		path   string
	}
	for _, tc := range setupNativeCases {
		out = append(out, struct {
			method string
			path   string
		}{tc.method, tc.path}, struct {
			method string
			path   string
		}{tc.method, tc.path + "/"})
	}
	return out
}

// TestSetupNative_PublicWithoutCredential pins the setup carve-out
// (C-c1 APPROVED, graduated to D8-grade handling on the D10 wave):
// the four setup routes serve without the shared-key gate, so a missing,
// empty-configured, or bogus credential still reaches the core and the
// core's answer relays back. The neighboring RequireAuth route denies
// with 401 under the same setups, proving no inherited global auth
// leaks onto the carve-out and no gate was widened beyond the four
// approved routes. Both negotiated shapes are covered.
func TestSetupNative_PublicWithoutCredential(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		// The shared body carries the seal shape: the terminated GET
		// resolves it while the POST legs assert reachability only
		// (status plus forward), never body bytes.
		core := &stubCore{status: http.StatusOK, body: `{"needs_setup":true,"bootstrap_mode":"generated"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		withKey := NewServer(upstream.URL, skeletonTestKey)
		withoutKey := NewServer(upstream.URL, "")
		withKeyHandler := withKey.Handler(DefaultV0Sunset)
		withoutKeyHandler := withoutKey.Handler(DefaultV0Sunset)

		// No credential on a keyed server, no credential on a keyless
		// server, and a bogus credential on a keyed server must all
		// reach the core with the same result.
		setups := []struct {
			name    string
			handler http.Handler
			auth    string
		}{
			{"no credential", withKeyHandler, ""},
			{"empty configured key", withoutKeyHandler, ""},
			{"bogus credential ignored", withKeyHandler, "Bearer bogus"},
		}
		for _, setup := range setups {
			hitsBefore := core.hits
			for _, spelling := range setupSpellings() {
				var body io.Reader
				if spelling.method != http.MethodGet {
					body = strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`)
				}
				req := httptest.NewRequest(spelling.method, spelling.path, body)
				req.Header.Set("Accept", accept)
				if setup.auth != "" {
					req.Header.Set("Authorization", setup.auth)
				}
				rec := httptest.NewRecorder()
				setup.handler.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("%s %s %q Accept %q: status = %d, want relayed 200 (no dest 401)",
						setup.name, spelling.method, spelling.path, accept, rec.Code)
				}
				if core.hits != hitsBefore+1 {
					t.Errorf("%s %s %q Accept %q: upstream hits = %d, want one more (carve-out must forward)",
						setup.name, spelling.method, spelling.path, accept, core.hits-hitsBefore)
				}
				hitsBefore = core.hits
			}
		}

		// The carve-out stops at the four routes: a neighboring
		// RequireAuth route still denies without a credential.
		req := httptest.NewRequest(http.MethodPatch, "/api/v0/auth/admin/sign-in-policy", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		withKeyHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: neighboring RequireAuth route status = %d, want 401 (carve-out is exact)",
				accept, rec.Code)
		}
	}
}

// TestSetupNative_V1AliasStaysPublic pins that the v1 alias rewrite
// runs ahead of the carve-out: the v1 spelling of a setup mutation
// reaches the core without a credential, exactly like its v0 twin.
func TestSetupNative_V1AliasStaysPublic(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"claimed"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/setup/claim",
		strings.NewReader(`{"username":"bootstrap-u","password":"bootstrap-p"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("v1 setup claim status = %d, want relayed 200", rec.Code)
	}
	if core.hits != 1 || core.lastPath != "/api/v0/auth/setup/claim" {
		t.Errorf("hits = %d lastPath = %q, want one forward to the rewritten v0 path",
			core.hits, core.lastPath)
	}
}

// TestSetupNative_NoSharedKeyInjection pins that dest never injects
// its shared key on the carve-out: a caller credential (or its
// absence) forwards verbatim. The bootstrap bodies cross as opaque
// bytes alongside it.
func TestSetupNative_NoSharedKeyInjection(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		auth     string
		wantAuth string
	}{
		{"absent stays absent", "", ""},
		{"caller credential forwards verbatim", "Bearer caller-token", "Bearer caller-token"},
		{"dest key is just another caller value", "Bearer " + skeletonTestKey, "Bearer " + skeletonTestKey},
	} {
		core := &stubCore{status: http.StatusOK, body: `{"status":"claimed"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		const claimBody = `{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/setup/claim", strings.NewReader(claimBody))
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want relayed 200", tc.name, rec.Code)
		}
		if core.lastAuth != tc.wantAuth {
			t.Errorf("%s: upstream Authorization = %q, want %q (verbatim, never the dest key)",
				tc.name, core.lastAuth, tc.wantAuth)
		}
		if core.lastBody != claimBody {
			t.Errorf("%s: upstream body = %q, want the opaque credential bytes", tc.name, core.lastBody)
		}
	}
}

// setupRelayCases covers the pinned core answers the carve-out must
// relay byte-for-byte: the wizard results, the indistinguishable
// 401, the sealed 410s, and the MFA answer that carries the session
// cookie plus the one-time recovery codes. Header values the core
// sets (including Set-Cookie) relay with the body. The setup-state
// GET is covered separately: it terminates via the SetupNeeded port
// (see setup_terminate_test.go).
func TestSetupNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	const sealedBody = `{"error_code":"gone","message":"first-run setup is no longer available; this instance already has an identity","correlation_id":"seal-1"}`
	const deniedBody = `{"error_code":"unauthenticated","message":"bootstrap credential is invalid or expired; retrieve it again with ` + "`eshu admin initial-credential`" + ` or regenerate it with ` + "`eshu admin reset-initial-credential`" + `","correlation_id":"deny-1"}`
	mfaHeader := http.Header{}
	mfaHeader.Set("Set-Cookie", "eshu_local_session=opaque-session; Path=/; HttpOnly")

	cases := []struct {
		name    string
		method  string
		path    string
		reqBody string
		status  int
		header  http.Header
		body    string
	}{
		// The setup-state GET is not listed here: it terminates via
		// the SetupNeeded port (see setup_terminate_test.go), so its
		// parity is semantic, not byte-for-byte.
		{
			"claim", http.MethodPost, "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 200, nil, `{"status":"claimed"}`,
		},
		{
			"claim denied is 401", http.MethodPost, "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"wrong"}`, 401, nil, deniedBody,
		},
		{
			"admin", http.MethodPost, "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`, 200, nil,
			`{"status":"admin_created","tenant_id":"t","workspace_id":"w"}`,
		},
		{
			"mfa with session cookie and recovery codes", http.MethodPost, "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 200, mfaHeader,
			`{"status":"completed","recovery_codes":["r1","r2"],"auth":{}}`,
		},
		{
			"mfa concurrent loser is 409", http.MethodPost, "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 409, nil,
			`{"error_code":"conflict","message":"setup was already completed by a concurrent request"}`,
		},
		{
			"sealed claim is 410", http.MethodPost, "/api/v0/auth/setup/claim",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 410, nil, sealedBody,
		},
		{
			"sealed admin is 410", http.MethodPost, "/api/v0/auth/setup/admin",
			`{"username":"bootstrap-u","password":"bootstrap-p","new_password":"fresh-secret-16!!"}`, 410, nil, sealedBody,
		},
		{
			"sealed mfa is 410", http.MethodPost, "/api/v0/auth/setup/mfa",
			`{"username":"bootstrap-u","password":"bootstrap-p"}`, 410, nil, sealedBody,
		},
	}

	for _, tc := range cases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			var body io.Reader
			if tc.reqBody != "" {
				body = strings.NewReader(tc.reqBody)
			}
			req := httptest.NewRequest(tc.method, tc.path+"?wizard=1", body)
			req.Header.Set("Accept", accept)
			req.Header.Set("X-Custom", "keep-me")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Errorf("%s Accept %q: status = %d, want relayed %d",
					tc.name, accept, rec.Code, tc.status)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if string(gotBody) != tc.body {
				t.Errorf("%s Accept %q: body = %q, want the sealed core bytes %q",
					tc.name, accept, gotBody, tc.body)
			}
			if core.lastQuery != "wizard=1" || core.lastMethod != tc.method || core.lastPath != tc.path {
				t.Errorf("%s Accept %q: upstream saw %s %s?%s, want the verbatim route and query",
					tc.name, accept, core.lastMethod, core.lastPath, core.lastQuery)
			}
			if got := core.lastAccept; got != accept {
				t.Errorf("%s: upstream Accept = %q, want %q forwarded", tc.name, got, accept)
			}
			if tc.header.Get("Set-Cookie") != "" {
				if got := rec.Header().Get("Set-Cookie"); got != tc.header.Get("Set-Cookie") {
					t.Errorf("%s Accept %q: Set-Cookie = %q, want the relayed core cookie %q",
						tc.name, accept, got, tc.header.Get("Set-Cookie"))
				}
			}
			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (no dest retry)",
					tc.name, accept, core.hits)
			}
		}
	}
}
