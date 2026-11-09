// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// Signinpolicy-group termination proof (Refs eshu-hq/eshu#9669,
// satellite eshu-hq/eshu-api#72). GET /api/v0/auth/sign-in-policy and
// GET /api/v0/auth/providers terminate dest-side: the policy read
// crosses the frozen SignInPolicyReadAdmission v0 port (no table
// handle), dest synthesizes the 200, and every failure closes to the
// permanent local 503. TDD RED first: this test names handlers that do
// not exist yet, so it must fail to compile before the lane lands.
func TestPublicSignInPolicy_SynthesizesRequireSSO(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		core string
		want bool
	}{
		{"enabled", `{"require_sso":true}`, true},
		{"disabled", `{"require_sso":false}`, false},
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{status: http.StatusOK, body: tc.core}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s Accept %q: status = %d, want terminated 200", tc.name, accept, rec.Code)
			}
			var got struct {
				RequireSSO bool `json:"require_sso"`
			}
			raw, _ := io.ReadAll(rec.Result().Body)
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("%s Accept %q: decode %q: %v", tc.name, accept, raw, err)
			}
			if got.RequireSSO != tc.want {
				t.Errorf("%s Accept %q: require_sso = %v, want %v", tc.name, accept, got.RequireSSO, tc.want)
			}
		}
	}
}

// TestSigninpolicyGroup_TwinsStayRelay is the TDD RED proof for the
// termination lane: only the two exact GET paths leave RequireAuth.
// Trailing-slash twins, deeper subtrees, and non-GET spellings keep
// the standing 401 for keyless callers with zero core contact. Today
// the leafb wave registers the slash twins on the native public lane,
// so the keyless slash currently answers 200: this test fails until
// the lane lands.
func TestSigninpolicyGroup_TwinsStayRelay(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, target := range []string{
		"GET /api/v0/auth/sign-in-policy/",
		"GET /api/v0/auth/sign-in-policy/extra",
		"GET /api/v0/auth/providers/",
		"GET /api/v0/auth/providers/extra",
		"POST /api/v0/auth/sign-in-policy",
		"POST /api/v0/auth/providers",
	} {
		method, path, _ := strings.Cut(target, " ")
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want the standing 401 (twin stays relay)", target, rec.Code)
		}
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (twin denials never dispatch)", core.hits)
	}
}

// TestPublicSignInPolicy_EmptyTenantZeroContact pins the empty-tenant
// default: no tenant_id answers 200 with require_sso:false and zero
// core contact, in both shapes, matching the core handler instead of
// a cross-tenant scan.
func TestPublicSignInPolicy_EmptyTenantZeroContact(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusOK, body: `{"require_sso":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		for _, path := range []string{"/api/v0/auth/sign-in-policy", "/api/v0/auth/sign-in-policy?tenant_id="} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("%s Accept %q: status = %d, want 200", path, accept, rec.Code)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != `{"require_sso":false}` {
				t.Errorf("%s Accept %q: body = %q, want the empty-tenant default", path, accept, body)
			}
		}
		if core.hits != 0 {
			t.Errorf("Accept %q: upstream hits = %d, want 0 (no scan)", accept, core.hits)
		}
	}
}

// TestPublicSignInPolicy_FailClosed pins the fail-closed mapping: any
// non-200 core answer, an unreadable body, or a missing require_sso
// key answers the permanent local 503 in both shapes with exactly one
// upstream hit and never a Retry-After. Dest never relays a core
// failure and never invents a retry.
func TestPublicSignInPolicy_FailClosed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		coreStatus int
		coreBody   string
	}{
		{"store failure is 503", http.StatusInternalServerError, `{"error_code":"server_failure"}`},
		{"core gone is 503", http.StatusGone, `{"error_code":"gone"}`},
		{"retryable core 503 is still permanent 503", http.StatusServiceUnavailable, `{"error_code":"busy"}`},
		{"malformed body is 503", http.StatusOK, `not-json{{`},
		{"missing signal is 503", http.StatusOK, `{"bootstrap_mode":"generated"}`},
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			core := &stubCore{
				status: tc.coreStatus,
				body:   tc.coreBody,
				header: http.Header{"Retry-After": []string{"1"}},
			}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("%s Accept %q: status = %d, want fail-closed 503", tc.name, accept, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("%s Accept %q: Retry-After = %q, want absent (permanent 503)", tc.name, accept, got)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(gotBody), "backend_unavailable") {
				t.Errorf("%s Accept %q: body = %q, want backend_unavailable", tc.name, accept, gotBody)
			}
			if core.hits != 1 {
				t.Errorf("%s Accept %q: upstream hits = %d, want exactly 1 (no dest retry)", tc.name, accept, core.hits)
			}
		}
	}
}

// TestPublicSignInPolicy_DeadCoreSingleAttempt pins the dead-core
// shape: the permanent 503 in both shapes with exactly one dispatch
// attempt and no Retry-After.
func TestPublicSignInPolicy_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
		var attempts int32
		srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if got := atomic.LoadInt32(&attempts); got != 1 {
			t.Errorf("Accept %q: dispatch attempts = %d, want exactly 1 (no retry)", accept, got)
		}
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After = %q, want absent (permanent 503)", accept, got)
		}
	}
}

// TestPublicSignInPolicy_RedirectStripped pins the redirect proof:
// the dest-synthesized 200 carries no Location and no Set-Cookie,
// even when the core answer on the fetch carries both. Dest owns the
// headers here; nothing crosses except the decoded signal.
func TestPublicSignInPolicy_RedirectStripped(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"require_sso":true}`,
		header: http.Header{
			"Location":   []string{"/login?local=1"},
			"Set-Cookie": []string{"eshu_session=sess_123; Path=/; HttpOnly"},
		},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want terminated 200", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("Location = %q, want absent (synthesized dest-side)", got)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "" {
		t.Errorf("Set-Cookie = %q, want absent (synthesized dest-side)", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Errorf("Cache-Control = %q, want the synthesized posture", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != `{"require_sso":true}` {
		t.Errorf("body = %q, want the synthesized signal", body)
	}
}

// TestPublicSignInPolicy_ForwardsAcceptAndQueryWithoutCredential pins
// the D10 lesson: the anonymous port fetch forwards the caller's
// query string, Accept negotiation, correlation ID, and W3C trace
// context — and never an Authorization header. The twin spelling
// never crosses: the core always sees the canonical path.
func TestPublicSignInPolicy_ForwardsAcceptAndQueryWithoutCredential(t *testing.T) {
	t.Parallel()

	var got struct {
		hits        int
		path        string
		query       string
		accept      string
		auth        string
		traceparent string
		tracestate  string
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		got.hits++
		got.path = r.URL.Path
		got.query = r.URL.RawQuery
		got.accept = r.Header.Get("Accept")
		got.auth = r.Header.Get("Authorization")
		got.traceparent = r.Header.Get("traceparent")
		got.tracestate = r.Header.Get("tracestate")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"require_sso":true}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
	req.Header.Set("Accept", EnvelopeMIME)
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	req.Header.Set("X-Correlation-ID", "corr-9")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want terminated 200", rec.Code)
	}
	if got.hits != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", got.hits)
	}
	if got.path != "/api/v0/auth/sign-in-policy" {
		t.Errorf("upstream path = %q, want the canonical core path", got.path)
	}
	if got.query != "tenant_id=tenant_a" {
		t.Errorf("upstream query = %q, want the caller query", got.query)
	}
	if got.accept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the caller negotiation", got.accept)
	}
	if got.auth != "" {
		t.Errorf("upstream Authorization = %q, want empty (no credential crosses)", got.auth)
	}
	if got.traceparent != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
		t.Errorf("upstream traceparent = %q, want the caller context", got.traceparent)
	}
	if got.tracestate != "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE" {
		t.Errorf("upstream tracestate = %q, want the caller context", got.tracestate)
	}
}

// TestPublicSignInPolicy_V1AliasTerminates pins that the v1 alias
// rewrite runs ahead of the terminated GET: the v1 spelling serves
// from the port with a single fetch of the rewritten v0 core path.
func TestPublicSignInPolicy_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"require_sso":false}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sign-in-policy?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("v1 status = %d, want terminated 200", rec.Code)
	}
	if core.hits != 1 || core.lastPath != "/api/v0/auth/sign-in-policy" {
		t.Errorf("hits = %d lastPath = %q, want one fetch of the rewritten v0 path", core.hits, core.lastPath)
	}
}

// TestSignInPolicyFetchErrorPinsSentinel pins the fetch sentinel: the
// error the port fetch returns on every failure path carries the
// stable message and never core detail.
func TestSignInPolicyFetchErrorPinsSentinel(t *testing.T) {
	t.Parallel()

	if got := errSignInPolicyFetch.Error(); got == "" {
		t.Error("fetch sentinel Error() is empty, want the stable message")
	}
	if !strings.Contains(errSignInPolicyFetch.Error(), "sign-in-policy fetch failed") {
		t.Errorf("fetch sentinel = %q, want the stable message", errSignInPolicyFetch.Error())
	}
}

// errBodyRoundTripper answers 200 with an unreadable body: it proves
// the unreadable-body fail-closed mapping without a live outage.
type errBodyRoundTripper struct{}

func (errBodyRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(errReader{}),
	}, nil
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (errReader) Close() error             { return nil }

// TestPublicSignInPolicy_UnreadableBodyFailsClosed pins the
// unreadable-body mapping: a 200 whose body cannot be read still
// fails closed with the permanent 503 and never a Retry-After.
func TestPublicSignInPolicy_UnreadableBodyFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	srv.httpClient = &http.Client{Transport: errBodyRoundTripper{}}
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/sign-in-policy?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestPublicSignInPolicy_UnbuildableFetchFailsClosed pins the
// defensive mapping: a fetch target the transport cannot build (an
// attacker-influenced query the URL parser rejects) fails the port
// with the sentinel, and the handler closes to the permanent 503.
func TestPublicSignInPolicy_UnbuildableFetchFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	if _, err := srv.fetchSignInPolicy(t.Context(), signInPolicyFetchParams{
		tenant:      "tenant_a",
		query:       "tenant_id=\x7f",
		correlation: "corr-1",
	}); err != errSignInPolicyFetch {
		t.Errorf("fetch error = %v, want the sentinel", err)
	}
}
