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

// TestD9SAMLPublic_CarveOutServesKeyless pins the C-c1 carve-out: the
// 3 SAML routes serve credential-less callers — no dest 401 — and
// forward once to the same core route with the opaque caller bytes.
// This is the behavior change the carve-out exists for: before this
// lane these paths fell through to the generic relay inside
// RequireAuth and answered dest 401s without ever reaching core. The
// ACS row carries the IdP form shape; the GET rows carry the provider
// path value plus the optional return_to query.
func TestD9SAMLPublic_CarveOutServesKeyless(t *testing.T) {
	t.Parallel()

	for _, tc := range d9SAMLPublicCases {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			target := tc.path
			if tc.query != "" {
				target += "?" + tc.query
			}
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, target, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", tc.contentType)
			} else {
				req = httptest.NewRequest(tc.method, target, nil)
			}
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (keyless carve-out forwards)", tc.name, accept, core.hits)
			}
			if core.lastMethod != tc.method || core.lastPath != tc.path {
				t.Errorf("%s Accept %q: upstream = %s %s, want the same core route", tc.name, accept, core.lastMethod, core.lastPath)
			}
			if core.lastQuery != tc.query {
				t.Errorf("%s Accept %q: upstream query = %q, want the opaque caller query", tc.name, accept, core.lastQuery)
			}
			if core.lastBody != tc.body {
				t.Errorf("%s Accept %q: upstream body = %q, want the opaque caller body", tc.name, accept, core.lastBody)
			}
			if core.lastAuth != "" {
				t.Errorf("%s Accept %q: upstream Authorization = %q, want empty (no shared-key injection on public routes)",
					tc.name, accept, core.lastAuth)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("%s Accept %q: status = %d, want relayed 200", tc.name, accept, rec.Code)
			}
		}
	}
}

// TestD9SAMLPublic_CarveOutIsExactlyThree pins the carve-out boundary:
// only the three exact SAML paths leave the gate. Near-miss paths fall
// through to the keyed generic relay and keep the standing 401 for
// credential-less callers with zero core contact; wrong-method
// spellings of the carved paths are not carved out either (core's
// bypass is method-pinned: GET on metadata/login, POST on ACS); the
// OIDC and GitHub login paths stay on their own landed lanes.
func TestD9SAMLPublic_CarveOutIsExactlyThree(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers",
		"GET /api/v0/auth/saml/providers/p1/logins",
		"GET /api/v0/auth/saml/providers/p1/login/extra",
		"POST /api/v0/auth/saml/providers/p1/login",
		"GET /api/v0/auth/saml/providers/p1/acs",
		"PUT /api/v0/auth/saml/providers/p1/acs",
		"DELETE /api/v0/auth/saml/providers/p1/metadata",
		"GET /api/v0/auth/saml/providers/a/b/login",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (not carved out)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate denials never dispatch)", core.hits)
	}
}

// TestD9SAMLPublic_EmptyProviderCleansThenDenies pins the empty
// provider_id spelling: Go's mux sanitizes the doubled slash with a
// redirect to the cleaned path (no core contact), and the cleaned path
// matches no carve-out route, so the keyless caller keeps the standing
// 401 with zero core hits end to end. Dest answers no 405 anywhere on
// this lane: uncarved spellings stay on the relay and its gate.
func TestD9SAMLPublic_EmptyProviderCleansThenDenies(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers//login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTemporaryRedirect && rec.Code != http.StatusMovedPermanently {
		t.Fatalf("GET empty-provider: status = %d, want the mux-clean redirect", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/api/v0/auth/saml/providers/login" {
		t.Fatalf("GET empty-provider: Location = %q, want the cleaned path", loc)
	}

	follow := httptest.NewRequest(http.MethodGet, loc, nil)
	followRec := httptest.NewRecorder()
	handler.ServeHTTP(followRec, follow)
	if followRec.Code != http.StatusUnauthorized {
		t.Errorf("GET cleaned path keyless: status = %d, want the standing 401", followRec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (sanitize plus gate denial never dispatch)", core.hits)
	}
}

// TestD9SAMLPublic_EscapedSlashAgreesWithCore pins the %2F
// provider_id row: Go matches the escaped spelling as one wildcard
// segment, so the native route forwards it opaquely (RequestURI
// untouched) with exactly one upstream hit and the relayed answer —
// dest and core see the same bytes, and dest judges none of them. A
// 405 or a dest-side decode would fail this row.
func TestD9SAMLPublic_EscapedSlashAgreesWithCore(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/a%2Fb/login?return_to=%2Fconsole", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1 (escaped slash forwards opaquely)", core.hits)
	}
	if rec.Code != http.StatusFound {
		t.Errorf("status = %d, want the relayed 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://idp.example.com/sso?SAMLRequest=opaque" {
		t.Errorf("Location = %q, want the core's value relayed", got)
	}
}

// TestD9SAMLPublic_HeadOnGetForwardsOnce pins the HEAD-on-GET rows:
// Go routes HEAD to the GET pattern, so the native lane forwards once
// and relays core's answer with core re-judging — dest adds no
// HEAD-specific branch, and a retried or refused HEAD still costs
// exactly one upstream attempt.
func TestD9SAMLPublic_HeadOnGetForwardsOnce(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{
		"/api/v0/auth/saml/providers/p1/metadata",
		"/api/v0/auth/saml/providers/p1/login",
	} {
		before := core.hits
		req := httptest.NewRequest(http.MethodHead, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits != before+1 {
			t.Errorf("HEAD %s: upstream hits = %d, want one forward (before %d)", path, core.hits, before)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("HEAD %s: status = %d, want relayed 200", path, rec.Code)
		}
		if core.lastMethod != http.MethodHead {
			t.Errorf("HEAD %s: upstream method = %q, want HEAD forwarded for core to judge", path, core.lastMethod)
		}
	}
}

// TestD9SAMLPublic_AuthedCallerKeyCrossesUntouched pins that a
// shared-key caller on a public route forwards its own key as an
// ordinary header: dest neither strips it nor judges it there — core
// judges every credential it receives.
func TestD9SAMLPublic_AuthedCallerKeyCrossesUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/sso"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization = %q, want the caller key untouched", core.lastAuth)
	}
	if core.lastQuery != "return_to=%2Fconsole" {
		t.Errorf("upstream query = %q, want the opaque caller query", core.lastQuery)
	}

	acs := authedRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs")
	acsRec := httptest.NewRecorder()
	handler.ServeHTTP(acsRec, acs)
	if core.hits != 2 {
		t.Fatalf("acs upstream hits = %d, want 2 total", core.hits)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("acs upstream Authorization = %q, want the caller key untouched", core.lastAuth)
	}
}

// TestD9SAMLPublic_SlashSpellingStaysRelayed pins the exact-path
// carve-out boundary: the slash spellings of the SAML routes are NOT
// carved out (a slash twin would be a subtree swallowing past the
// bypass). Keyless callers keep the standing 401 with zero core
// contact; keyed callers ride the generic relay and get core's answer
// for the spelled path.
func TestD9SAMLPublic_SlashSpellingStaysRelayed(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers/p1/metadata/",
		"GET /api/v0/auth/saml/providers/p1/login/",
		"POST /api/v0/auth/saml/providers/p1/acs/",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s keyless: status = %d, want the standing 401", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (slash spellings never bypass)", core.hits)
	}

	for _, target := range []string{
		"GET /api/v0/auth/saml/providers/p1/metadata/",
		"GET /api/v0/auth/saml/providers/p1/login/",
		"POST /api/v0/auth/saml/providers/p1/acs/",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := authedRequest(method, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if core.hits == 0 {
			t.Fatalf("%s keyed: no upstream hit, want the generic relay to forward", target)
		}
		if core.lastPath != path {
			t.Errorf("%s keyed: upstream path = %q, want the spelled path relayed", target, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("%s keyed: status = %d, want relayed 200", target, rec.Code)
		}
	}
}

// TestD9SAMLNative_V1AliasOrderingUnchanged pins that the v1 alias
// rewrite lands the SAML routes on their native handlers: the keyless
// v1 login and the keyless v1 ACS reach the public carve-out with the
// rewritten /api/v0/* paths.
func TestD9SAMLNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	public := httptest.NewRequest(http.MethodGet, "/api/v1/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, public)

	if core.hits != 1 || core.lastPath != "/api/v0/auth/saml/providers/acme-munich/login" {
		t.Errorf("public: hits = %d path = %q, want alias rewrite to the carve-out v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("public: Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("public: status = %d, want relayed 200", rec.Code)
	}

	acsBody := strings.NewReader("RelayState=opaque-relay& SAMLResponse=opaque-response")
	acs := httptest.NewRequest(http.MethodPost, "/api/v1/auth/saml/providers/acme-munich/acs", acsBody)
	acs.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	acsRec := httptest.NewRecorder()
	handler.ServeHTTP(acsRec, acs)
	if core.hits != 2 || core.lastPath != "/api/v0/auth/saml/providers/acme-munich/acs" {
		t.Errorf("acs: hits = %d path = %q, want alias rewrite to the carve-out v0 route", core.hits, core.lastPath)
	}
}

// TestD9SAMLNative_NeighborLanes pins the lane boundaries around the
// SAML registrations: the OIDC and GitHub login routes stay on their
// own landed native lanes (keyless forwards once with the opaque
// query), and the SAML paths share no registration key with those
// lanes beyond /api/v0/auth — Go matches method and path together, so
// the lanes never merge. This is the inverted replacement for the
// relay-only SAML assertions the OIDC and GitHub suites carried before
// this lane landed (see TestD9Native_NeighborLanesStillRelay and
// TestD9GitHubNative_NeighborLanes): keyless SAML now forwards
// natively instead of keeping the standing 401.
func TestD9SAMLNative_NeighborLanes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"status":"accepted"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	oidc := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, oidc)
	if core.hits != 1 || core.lastPath != "/api/v0/auth/oidc/login" {
		t.Errorf("keyless oidc login: hits = %d path = %q, want the landed OIDC lane to forward once untouched",
			core.hits, core.lastPath)
	}

	github := httptest.NewRequest(http.MethodGet, "/api/v0/auth/github/login?provider_config_id=github-dev", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, github)
	if core.hits != 2 || core.lastPath != "/api/v0/auth/github/login" {
		t.Errorf("keyless github login: hits = %d path = %q, want the landed GitHub lane to forward once untouched",
			core.hits, core.lastPath)
	}

	saml := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/p1/login?return_to=%2Fconsole", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, saml)
	if core.hits != 3 || core.lastPath != "/api/v0/auth/saml/providers/p1/login" {
		t.Errorf("keyless saml login: hits = %d path = %q, want this lane to forward once untouched",
			core.hits, core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("keyless saml login: status = %d, want 200", rec.Code)
	}
}

// TestD9SAMLPublic_QueryCarriesNoDestBody pins that the login GETs
// forward no caller body: the credential material rides the path and
// query string only, and dest invents no body of its own. The ACS POST
// is the lane's only body carrier and is covered byte-identical in
// the dispatch suite.
func TestD9SAMLPublic_QueryCarriesNoDestBody(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusFound, header: http.Header{"Location": []string{"https://idp.example.com/sso"}}, body: ``}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastBody != "" {
		t.Errorf("upstream body = %q, want empty (login GETs carry no body)", core.lastBody)
	}
}

// TestD9SAMLNative_ServedEnvelopeParity pins that both negotiated
// shapes relay the SAML bytes identically: the metadata XML keeps its
// content type and the ACS JSON keeps its body in the envelope shape
// and the plain shape alike.
func TestD9SAMLNative_ServedEnvelopeParity(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusCreated,
			header: http.Header{"Set-Cookie": []string{"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict"}},
			body:   `{"csrf_token":"opaque-csrf"}`,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		acsBody := strings.NewReader("RelayState=opaque-relay& SAMLResponse=opaque-response")
		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", acsBody)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 {
			t.Fatalf("Accept %q: upstream hits = %d, want exactly 1", accept, core.hits)
		}
		if rec.Code != http.StatusCreated {
			t.Errorf("Accept %q: status = %d, want relayed 201", accept, rec.Code)
		}
		respBody, _ := io.ReadAll(rec.Result().Body)
		if string(respBody) != `{"csrf_token":"opaque-csrf"}` {
			t.Errorf("Accept %q: body = %q, want the core bytes", accept, respBody)
		}
		if cookies := rec.Result().Header.Values("Set-Cookie"); len(cookies) != 1 {
			t.Errorf("Accept %q: Set-Cookie = %q, want the core cookie relayed", accept, cookies)
		}
	}
}
