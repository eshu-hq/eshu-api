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

// SAML-triple native termination proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72, saml-triple lane, last of the
// sequential termination waves). GET
// /api/v0/auth/saml/providers/{provider_id}/metadata, GET .../login,
// and POST .../acs terminate dest-side: the three exact pre-auth
// public paths register outside every gate ahead of the generic
// relay, each makes exactly one anonymous core fetch over D0-proven
// transport with the caller bytes crossing opaquely, and dest
// synthesizes the answer from the port result (metadata XML
// byte-verbatim, login redirect status+Location byte-verbatim,
// ACS 201/303 with Set-Cookie byte-verbatim, error envelopes
// verbatim). Login-state and session stores stay core-side: dest
// holds no store handle anywhere in this lane. TDD RED first: this
// test names handlers that do not exist yet, so it must fail to
// compile before the lane lands.
func TestSAMLTerminated_MetadataSynthesizesXML(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusOK,
			header: http.Header{"Content-Type": []string{"application/samlmetadata+xml"}},
			body:   `<EntityDescriptor entityID="https://eshu.example/saml/acme-munich"/>`,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/metadata", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Accept %q: status = %d, want terminated 200", accept, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/samlmetadata+xml" {
			t.Errorf("Accept %q: Content-Type = %q, want the metadata XML type verbatim", accept, got)
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `<EntityDescriptor entityID="https://eshu.example/saml/acme-munich"/>` {
			t.Errorf("Accept %q: body = %q, want core metadata bytes verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1 (no retry on metadata)", accept, core.hits)
		}
		if core.lastMethod != http.MethodGet || core.lastPath != "/api/v0/auth/saml/providers/acme-munich/metadata" {
			t.Errorf("Accept %q: upstream = %s %s, want the same core route (swap mutant lands here)", accept, core.lastMethod, core.lastPath)
		}
	}
}

// TestSAMLTerminated_LoginSynthesizesRedirect pins the terminated
// login: the core 302 answers with the IdP SSO Location — including
// the SigAlg and Signature query values — byte-verbatim, with exactly
// one upstream attempt carrying the opaque caller query.
func TestSAMLTerminated_LoginSynthesizesRedirect(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusFound,
			header: http.Header{"Location": []string{"https://idp.example.com/sso?SAMLRequest=opaque&RelayState=opaque&SigAlg=opaque-sigalg&Signature=opaque-signature"}},
			body:   ``,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/saml/providers/acme-munich/login?return_to=%2Fconsole", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusFound {
			t.Fatalf("Accept %q: status = %d, want terminated 302", accept, rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "https://idp.example.com/sso?SAMLRequest=opaque&RelayState=opaque&SigAlg=opaque-sigalg&Signature=opaque-signature" {
			t.Errorf("Accept %q: Location = %q, want core value byte-verbatim (SigAlg/Signature intact)", accept, got)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1 (no retry on login start)", accept, core.hits)
		}
		if core.lastMethod != http.MethodGet || core.lastPath != "/api/v0/auth/saml/providers/acme-munich/login" {
			t.Errorf("Accept %q: upstream = %s %s, want the same core route (swap mutant lands here)", accept, core.lastMethod, core.lastPath)
		}
		if core.lastQuery != "return_to=%2Fconsole" {
			t.Errorf("Accept %q: upstream query = %q, want opaque caller query", accept, core.lastQuery)
		}
	}
}

// TestSAMLTerminated_ACSSynthesizesSession pins the terminated ACS:
// the core 201 (session JSON plus both session cookies) synthesizes
// byte-verbatim with exactly one upstream attempt carrying the opaque
// IdP-posted form bytes.
func TestSAMLTerminated_ACSSynthesizesSession(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{
			status: http.StatusCreated,
			header: http.Header{"Set-Cookie": []string{
				"__Host-eshu_session=opaque-session; Path=/; HttpOnly; Secure; SameSite=Strict",
				"__Host-eshu_csrf=opaque-csrf; Path=/; Secure; SameSite=Strict",
			}},
			body: `{"csrf_token":"opaque-csrf"}`,
		}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		form := "RelayState=opaque-relay& SAMLResponse=opaque-response"
		req := httptest.NewRequest(http.MethodPost, "/api/v0/auth/saml/providers/acme-munich/acs", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("Accept %q: status = %d, want terminated 201", accept, rec.Code)
		}
		cookies := rec.Header().Values("Set-Cookie")
		if len(cookies) != 2 {
			t.Fatalf("Accept %q: Set-Cookie count = %d, want 2 verbatim", accept, len(cookies))
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `{"csrf_token":"opaque-csrf"}` {
			t.Errorf("Accept %q: body = %q, want core bytes verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1 (no retry of ACS mutation)", accept, core.hits)
		}
		if core.lastMethod != http.MethodPost || core.lastPath != "/api/v0/auth/saml/providers/acme-munich/acs" {
			t.Errorf("Accept %q: upstream = %s %s, want the same core route (swap mutant lands here)", accept, core.lastMethod, core.lastPath)
		}
		if core.lastBody != form {
			t.Errorf("Accept %q: upstream body = %q, want the opaque caller form", accept, core.lastBody)
		}
	}
}

// TestSAMLTerminated_PathsAreTerminatedTriple pins the terminated
// triple through the lane-owned handler selector: the metadata route
// maps to the metadata leg, the login route to the login start leg,
// and every other d9SAMLRoutes entry to the ACS leg.
// samlTerminatedHandler does not exist yet, so this file must fail to
// compile before the lane lands.
func TestSAMLTerminated_PathsAreTerminatedTriple(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	if srv.samlTerminatedHandler(d9SAMLMetadata) == nil {
		t.Fatal("metadata route handler is nil")
	}
	if srv.samlTerminatedHandler(d9SAMLLogin) == nil {
		t.Fatal("login route handler is nil")
	}
	if srv.samlTerminatedHandler(d9SAMLACS) == nil {
		t.Fatal("acs route handler is nil")
	}
}
