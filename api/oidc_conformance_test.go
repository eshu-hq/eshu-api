// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/eshu-hq/eshu-api/openapi"
)

// OIDC-pair termination conformance (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72). The D0 proof per group: dest calls the frozen
// OIDCLoginAdmission v0 port (two methods, StartOIDCLogin +
// CompleteOIDCLogin, exact frozen signatures) over D0-proven transport
// and takes no table or store handle; the D8 resolver stays complete
// downstream (session issuance keeps its BrowserSession shape); the
// served spec carries both ops line-verbatim.
// TDD RED: this file names the port that does not exist yet, so the
// package must fail to compile before the lane lands.

// frozenOIDCTermPort pins the exact dest-side method set behind the
// versioned #9669 admission contract for the oidc pair: the two
// admission calls the terminated login GET and callback GET admit
// through. It mirrors the upstream frozen list
// (contracts/upstream/oidc_login_admission_contract_test.go:
// StartOIDCLogin + CompleteOIDCLogin) in the dest-owned opaque
// transport shape — same method names, opaque query in, mapped answer
// out, no core import — so a renamed, added, removed, or re-signed
// method fails here until the list is deliberately updated. Bump
// OIDCLoginAdmissionContractVersion alongside (manually — the bump is
// convention, the frozen list is the enforcement).
var frozenOIDCTermPort = []string{
	"CompleteOIDCLogin func(context.Context, api.oidcPortRequest) (api.oidcPortResponse, error)",
	"StartOIDCLogin func(context.Context, api.oidcPortRequest) (api.oidcPortResponse, error)",
}

// oidcTermPortMethodSet reflects the live oidcLoginPort method set
// into sorted "name signature" strings for comparison with the frozen
// list.
func oidcTermPortMethodSet(t *testing.T) []string {
	t.Helper()
	rt := reflect.TypeOf((*oidcLoginPort)(nil)).Elem()
	var got []string
	for i := 0; i < rt.NumMethod(); i++ {
		m := rt.Method(i)
		got = append(got, m.Name+" "+m.Type.String())
	}
	sort.Strings(got)
	return got
}

// TestOIDCTermPortIsFrozen pins the D0 port: the dest-side seam is
// exactly the two frozen admission calls. The version constant stays
// v0 (api/version.go, pinned against
// contracts/oidc-login-admission.md by version_test.go); the frozen
// method set below is the enforcement.
func TestOIDCTermPortIsFrozen(t *testing.T) {
	t.Parallel()

	got := oidcTermPortMethodSet(t)
	if len(got) != len(frozenOIDCTermPort) {
		t.Fatalf("oidcLoginPort method set = %q (len %d), want frozen %q (len %d)",
			got, len(got), frozenOIDCTermPort, len(frozenOIDCTermPort))
	}
	for i := range got {
		if got[i] != frozenOIDCTermPort[i] {
			t.Errorf("oidcLoginPort differs at index %d: got %q, want %q", i, got[i], frozenOIDCTermPort[i])
		}
	}
	if OIDCLoginAdmissionContractVersion != "v0" {
		t.Errorf("OIDCLoginAdmissionContractVersion = %q, want %q: bump it deliberately with the shape change",
			OIDCLoginAdmissionContractVersion, "v0")
	}
}

// TestOIDCTermGroup_SpecCarriesBothOps pins the OpenAPI assembly: the
// served document carries both terminated OIDC ops with the GitHub and
// SAML ops alongside. Fragments stay line-verbatim under
// scripts/check-parity.sh; this test pins the assembly around them.
func TestOIDCTermGroup_SpecCarriesBothOps(t *testing.T) {
	t.Parallel()

	doc := openapi.Spec()
	for _, want := range []string{
		`"/api/v0/auth/oidc/login"`,
		`"/api/v0/auth/oidc/callback"`,
		`"/api/v0/auth/github/login"`,
		`"/api/v0/auth/saml/providers/{provider_id}/login"`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("served spec lacks %s", want)
		}
	}
}

// TestOIDCTermPathsMirrorD9Routes pins that the terminated paths are
// exactly the frozen D9 route pair: a path added, removed, or renamed
// here fails until the D9 table, the OpenAPI fragment, and this
// mirror move together.
func TestOIDCTermPathsMirrorD9Routes(t *testing.T) {
	t.Parallel()

	want := map[string]string{d9OIDCLogin: oidcLoginPath, d9OIDCCallback: oidcCallbackPath}
	for route, path := range want {
		method, routePath, _ := strings.Cut(route, " ")
		if method != http.MethodGet {
			t.Errorf("route %q: method = %q, want GET (only GETs terminate)", route, method)
		}
		if routePath != path {
			t.Errorf("route %q: path = %q, want terminated %q", route, routePath, path)
		}
	}
}

// TestOIDCTerm_MatchesRelayByteForByte pins that the terminated GETs
// answer exactly like the legacy generic relay they replace: same
// status, same headers (Location, Retry-After, Content-Type,
// Set-Cookie per key in order), same body — across the pinned outcome
// matrix in both negotiated shapes. The legacy lane is replayed by
// mounting the generic dispatch for the same path in a test mux, so
// the comparison is terminated-vs-relay on the same binary and stub
// core, not a copy of the implementation.
func TestOIDCTerm_MatchesRelayByteForByte(t *testing.T) {
	t.Parallel()

	for _, route := range d9Routes {
		method, path, _ := strings.Cut(route, " ")
		query := d9ParityQuery(path)
		for _, outcome := range d9ParityOutcomes {
			for _, accept := range []string{EnvelopeMIME, "application/json"} {
				core := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				mux := http.NewServeMux()
				mux.Handle("/", core.handler())
				upstream := httptest.NewServer(mux)

				nativeSrv := NewServer(upstream.URL, skeletonTestKey)
				nativeHandler := nativeSrv.Handler(DefaultV0Sunset)
				nativeReq := httptest.NewRequest(method, path+"?"+query, nil)
				nativeReq.Header.Set("Accept", accept)
				nativeRec := httptest.NewRecorder()
				nativeHandler.ServeHTTP(nativeRec, nativeReq)
				nativeResult := nativeRec.Result()
				nativeBody, _ := io.ReadAll(nativeResult.Body)
				upstream.Close()

				if core.hits != 1 {
					t.Errorf("%s %s Accept %q: terminated upstream hits = %d, want exactly 1 (no retry on write-through)",
						route, outcome.name, accept, core.hits)
				}
				if core.lastQuery != query {
					t.Errorf("%s %s Accept %q: upstream query = %q, want the opaque caller query %q",
						route, outcome.name, accept, core.lastQuery, query)
				}

				relayCore := &stubCore{status: outcome.status, header: outcome.header, body: outcome.body}
				relayMux := http.NewServeMux()
				relayMux.Handle("/", relayCore.handler())
				relayUpstream := httptest.NewServer(relayMux)
				defer relayUpstream.Close()

				relaySrv := NewServer(relayUpstream.URL, skeletonTestKey)
				legacy := relaySrv.instrumented(viaRelay, RequireAuth(relaySrv.apiKey, http.HandlerFunc(relaySrv.dispatch)))
				relayReq := authedRequest(method, path+"?"+query)
				relayReq.Header.Set("Accept", accept)
				relayRec := httptest.NewRecorder()
				legacy.ServeHTTP(relayRec, relayReq)
				relayResult := relayRec.Result()
				relayBody, _ := io.ReadAll(relayResult.Body)

				if nativeRec.Code != relayRec.Code {
					t.Errorf("%s %s Accept %q: terminated status = %d, relay status = %d, want equal",
						route, outcome.name, accept, nativeRec.Code, relayRec.Code)
				}
				if string(nativeBody) != string(relayBody) {
					t.Errorf("%s %s Accept %q: terminated body = %q, relay body = %q, want byte-identical",
						route, outcome.name, accept, nativeBody, relayBody)
				}
				for _, key := range []string{"Location", "Retry-After", "Content-Type"} {
					if got, want := nativeResult.Header.Values(key), relayResult.Header.Values(key); !equalHeaders(got, want) {
						t.Errorf("%s %s Accept %q: terminated %s = %q, relay %s = %q, want equal",
							route, outcome.name, accept, key, got, key, want)
					}
				}
				if got, want := nativeResult.Header.Values("Set-Cookie"), relayResult.Header.Values("Set-Cookie"); !equalHeaders(got, want) {
					t.Errorf("%s %s Accept %q: terminated Set-Cookie = %q, relay Set-Cookie = %q, want every cookie verbatim",
						route, outcome.name, accept, got, want)
				}
			}
		}
	}
}

// TestOIDCTerm_RedirectUnfollowed pins the redirect contract: a core
// 302 relays as a 302 with its Location, and dest never issues the
// follow-up fetch. The Location points back at the same stub, so a
// followed redirect would land a second hit; exactly one hit plus the
// relayed 302 proves the shared dispatch client holds
// ErrUseLastResponse on this lane too.
func TestOIDCTerm_RedirectUnfollowed(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusFound,
		header: http.Header{"Location": []string{"/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/oidc/login?provider_config_id=okta-dev", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want the core's 302 unfollowed", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/api/v0/auth/oidc/callback?state=st-opaque-123&code=cd-opaque-456" {
		t.Errorf("Location = %q, want the core's value byte-verbatim", got)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no follow-up fetch)", core.hits)
	}
}

// TestOIDCTerm_NoRetryOnUnstable pins the single-attempt rule: the
// core 500, the retryable 503 (whose Retry-After relays untouched),
// and the dead core each cost exactly one upstream attempt. A retried
// login start would create a second server-side state; a retried
// callback would double-consume it — one forward is the whole
// exactly-once story dest-side.
func TestOIDCTerm_NoRetryOnUnstable(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"core 500", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"oidc login failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"request checkpoint failed"}`},
		{"core 503 permanent", http.StatusServiceUnavailable, nil, `{"error":"Service Unavailable","detail":"the session store is not configured"}`},
	} {
		core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, path := range []string{oidcLoginPath, oidcCallbackPath} {
			req := httptest.NewRequest(http.MethodGet, path+"?"+d9ParityQuery(path), nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Errorf("%s %s: upstream hits = %d, want exactly 1 (no dest retry)", tc.name, path, core.hits)
			}
			if rec.Code != tc.status {
				t.Errorf("%s %s: status = %d, want core status %d verbatim", tc.name, path, rec.Code, tc.status)
			}
			core.hits = 0
		}
	}
}

// TestOIDCTerm_DeadCoreSingleAttempt pins the dead-core shape: an
// unreachable core answers the permanent local 503
// backend_unavailable with no Retry-After, in both negotiated
// shapes, after exactly one attempt that fails inside the client.
func TestOIDCTerm_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	deadURL := upstream.URL
	upstream.Close()

	srv := NewServer(deadURL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{oidcLoginPath, oidcCallbackPath} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want permanent local 503", path, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", path, accept, got)
			}
		}
	}
}

// TestOIDCTerm_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: with no core origin configured both terminated spellings
// answer the permanent 503 backend_unavailable in both negotiated
// shapes, with zero core contact to make.
func TestOIDCTerm_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{oidcLoginPath, oidcCallbackPath} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("GET %s Accept %q: panicked with %v, want 503", path, accept, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("GET %s Accept %q: status = %d, want the permanent 503", path, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("GET %s Accept %q: Retry-After = %q, want empty (permanent, never retryable)", path, accept, got)
			}
		}
	}
}

// TestOIDCTerm_DisabledProviderDenyShape pins the disabled-provider
// answer: a core that serves no OIDC provider refuses with its 503
// unavailable shape, and the terminated login relays that shape
// verbatim instead of inventing a dest-side refusal or leaking
// provider detail.
func TestOIDCTerm_DisabledProviderDenyShape(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusServiceUnavailable, body: `{"error":"Service Unavailable","detail":"oidc login is unavailable"}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, oidcLoginPath+"?provider_config_id=okta-dev", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("Accept %q: status = %d, want the disabled-provider 503", accept, rec.Code)
		}
		raw, _ := io.ReadAll(rec.Result().Body)
		if string(raw) != `{"error":"Service Unavailable","detail":"oidc login is unavailable"}` {
			t.Errorf("Accept %q: body = %q, want the deny shape verbatim", accept, raw)
		}
		if core.hits != 1 {
			t.Errorf("Accept %q: upstream hits = %d, want exactly 1", accept, core.hits)
		}
	}
}

// TestOIDCTerm_TerminatedFilesNoAuthorizer pins the telemetry
// extension without renames: the terminated OIDC GETs file their
// family on the existing native lane with the closed outcome
// vocabulary and time the port fetches — and file no authorizer
// judgment, because the routes are public and there is no gate to
// judge.
func TestOIDCTerm_TerminatedFilesNoAuthorizer(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	tel := NewTelemetry(nil)
	srv.WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	for _, path := range []string{oidcLoginPath + "?provider_config_id=okta-dev", oidcCallbackPath + "?state=opaque&code=opaque"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusFound {
			t.Fatalf("GET %s status = %d, want 302", path, rec.Code)
		}
	}

	snap := tel.snapshot()
	if snap.dispatches["oidc_login"]["native"]["core_ok"] != 2 {
		t.Errorf("dispatch rows = %v, want two oidc_login/native/core_ok", snap.dispatches["oidc_login"])
	}
	if _, ok := snap.authorizer["oidc_login"]; ok {
		t.Errorf("authorizer rows = %v, want none for public oidc_login (no gate to judge)", snap.authorizer["oidc_login"])
	}
}
