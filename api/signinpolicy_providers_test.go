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

// Providers-GET termination proof (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#72, signinpolicy-group lane). The providers GET
// terminates dest-side: the provider list comes from a single
// anonymous fetch of the canonical core providers route and the
// local_login_offered bit comes from the SignInPolicyReadAdmission v0
// port; dest synthesizes the 200 with the core Cache-Control posture
// (public, max-age=60). Every failure closes to the permanent local
// 503 with never a Retry-After.

// TestProviders_SynthesizesPosture pins the terminated providers
// shape: a configured tenant (enabled and disabled policy) and the
// no-provider tenant each answer the dest-synthesized 200 whose
// decoded keys match the posture contract, in both negotiated
// shapes, with the provider list passed through and local_login
// derived from the port — not from the fetched offered bit.
func TestProviders_SynthesizesPosture(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		policyBody string
		listBody   string
		wantLocal  bool
	}{
		{
			"enabled with providers",
			`{"require_sso":true}`,
			`{"providers":[{"provider_config_id":"prov_1","display_label":"Single sign-on (OIDC)","provider_kind":"oidc","icon_hint":"oidc"}],"local_login_offered":false,"self_service_tokens_offered":true}`,
			false,
		},
		{
			"disabled with providers",
			`{"require_sso":false}`,
			`{"providers":[{"provider_config_id":"prov_1","display_label":"Single sign-on (OIDC)","provider_kind":"oidc","icon_hint":"oidc"}],"local_login_offered":true,"self_service_tokens_offered":true}`,
			true,
		},
		{
			"no providers",
			`{"require_sso":false}`,
			`{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`,
			true,
		},
	} {
		for _, accept := range []string{EnvelopeMIME, "application/json"} {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if r.URL.Path == "/api/v0/auth/providers" {
					_, _ = io.WriteString(w, tc.listBody)
					return
				}
				_, _ = io.WriteString(w, tc.policyBody)
			})
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			srv := NewServer(upstream.URL, skeletonTestKey)
			handler := srv.Handler(DefaultV0Sunset)

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
			req.Header.Set("Accept", accept)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s Accept %q: status = %d, want terminated 200", tc.name, accept, rec.Code)
			}
			var got struct {
				Providers []map[string]any `json:"providers"`
				Local     bool             `json:"local_login_offered"`
				SelfServe bool             `json:"self_service_tokens_offered"`
			}
			raw, _ := io.ReadAll(rec.Result().Body)
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("%s Accept %q: decode %q: %v", tc.name, accept, raw, err)
			}
			if got.Local != tc.wantLocal {
				t.Errorf("%s Accept %q: local_login_offered = %v, want %v (from the port)", tc.name, accept, got.Local, tc.wantLocal)
			}
			if !got.SelfServe {
				t.Errorf("%s Accept %q: self_service_tokens_offered = false, want true", tc.name, accept)
			}
			if got.Providers == nil {
				t.Errorf("%s Accept %q: providers is null, want an array", tc.name, accept)
			}
			if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
				t.Errorf("%s Accept %q: Cache-Control = %q, want the synthesized posture", tc.name, accept, got)
			}
		}
	}
}

// TestProviders_EmptyTenantZeroContact pins the empty-tenant
// default: no tenant_id answers the safe zero-configuration default
// with zero core contact, in both shapes.
func TestProviders_EmptyTenantZeroContact(t *testing.T) {
	t.Parallel()

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		core := &stubCore{status: http.StatusOK, body: `{"providers":[{"provider_config_id":"p"}],"local_login_offered":false,"self_service_tokens_offered":true}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers", nil)
		req.Header.Set("Accept", accept)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Accept %q: status = %d, want 200", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if string(body) != `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}` {
			t.Errorf("Accept %q: body = %q, want the zero default", accept, body)
		}
		if core.hits != 0 {
			t.Errorf("Accept %q: upstream hits = %d, want 0 (no scan)", accept, core.hits)
		}
	}
}

// TestProviders_ListFailureFailsClosed pins the fail-closed list
// mapping: a non-200 core answer on the providers fetch (including
// the core 500 on a provider-list failure) answers the permanent
// local 503 in both shapes with exactly one upstream hit and never a
// Retry-After. Dest never relays the core 500.
func TestProviders_ListFailureFailsClosed(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		coreStatus int
		coreBody   string
	}{
		{"list failure is 503 never 500", http.StatusInternalServerError, `{"error_code":"server_failure"}`},
		{"retryable core 503 is still permanent 503", http.StatusServiceUnavailable, `{"error_code":"busy"}`},
		{"malformed posture is 503", http.StatusOK, `not-json{{`},
		{"missing providers key is 503", http.StatusOK, `{"local_login_offered":true,"self_service_tokens_offered":true}`},
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

			req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
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

// TestProviders_PortFailureFailsClosed pins that a policy-fetch
// failure after a good list still fails closed: the list fetch
// succeeds but the port fetch answers non-200, so dest answers the
// permanent 503 rather than an offered-bit it cannot verify.
func TestProviders_PortFailureFailsClosed(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v0/auth/providers" {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error_code":"server_failure"}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_RedirectStripped pins the redirect proof for the
// providers leg: the dest-synthesized 200 carries no Location and no
// Set-Cookie even when a fetched answer carries them.
func TestProviders_RedirectStripped(t *testing.T) {
	t.Parallel()

	const listBody = `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`
	const policyBody = `{"require_sso":false}`
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/login?local=1")
		w.Header().Set("Set-Cookie", "eshu_session=sess_123; Path=/; HttpOnly")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, listBody)
			return
		}
		_, _ = io.WriteString(w, policyBody)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
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
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
		t.Errorf("Cache-Control = %q, want the synthesized posture", got)
	}
}

// TestProviders_FetchesForwardNegotiationWithoutCredential pins that
// both anonymous fetches forward the caller's query, Accept,
// correlation ID, and trace context — and never an Authorization
// header — against the canonical paths.
func TestProviders_FetchesForwardNegotiationWithoutCredential(t *testing.T) {
	t.Parallel()

	type seen struct {
		path, query, accept, auth, traceparent, tracestate string
	}
	var got []seen
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		got = append(got, seen{
			path:        r.URL.Path,
			query:       r.URL.RawQuery,
			accept:      r.Header.Get("Accept"),
			auth:        r.Header.Get("Authorization"),
			traceparent: r.Header.Get("traceparent"),
			tracestate:  r.Header.Get("tracestate"),
		})
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[{"provider_config_id":"prov_1","display_label":"Single sign-on (OIDC)","provider_kind":"oidc","icon_hint":"oidc"}],"local_login_offered":false,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":true}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	req.Header.Set("Accept", EnvelopeMIME)
	req.Header.Set("Authorization", "Bearer "+skeletonTestKey)
	req.Header.Set("X-Correlation-ID", "corr-7")
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("tracestate", "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want terminated 200", rec.Code)
	}
	if len(got) != 2 {
		t.Fatalf("upstream fetches = %d, want exactly 2 (list plus port, no retry)", len(got))
	}
	if got[0].path != "/api/v0/auth/providers" {
		t.Errorf("first fetch path = %q, want the canonical list path", got[0].path)
	}
	if got[1].path != "/api/v0/auth/sign-in-policy" {
		t.Errorf("second fetch path = %q, want the canonical policy path", got[1].path)
	}
	for i, f := range got {
		if f.query != "tenant_id=tenant_a" {
			t.Errorf("fetch %d query = %q, want the caller query", i, f.query)
		}
		if f.accept != EnvelopeMIME {
			t.Errorf("fetch %d Accept = %q, want the caller negotiation", i, f.accept)
		}
		if f.auth != "" {
			t.Errorf("fetch %d Authorization = %q, want empty (no credential crosses)", i, f.auth)
		}
		if f.traceparent != "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" {
			t.Errorf("fetch %d traceparent = %q, want the caller context", i, f.traceparent)
		}
		if f.tracestate != "rojo=00f067aa0ba902b7,congo=t61rcWkgMzE" {
			t.Errorf("fetch %d tracestate = %q, want the caller context", i, f.tracestate)
		}
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), `"local_login_offered":false`) {
		t.Errorf("body = %q, want the port-derived offered bit (policy enabled)", body)
	}
}

// TestProviders_V1AliasTerminates pins that the v1 alias rewrite
// runs ahead of the terminated providers GET.
func TestProviders_V1AliasTerminates(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":false}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("v1 status = %d, want terminated 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), `"providers":[]`) {
		t.Errorf("v1 body = %q, want the synthesized posture", body)
	}
}

// TestProviders_DeadCoreSingleAttempt pins the dead-core shape on the
// providers leg: the permanent 503 with a single dispatch attempt.
func TestProviders_DeadCoreSingleAttempt(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	var attempts int32
	srv.httpClient = &http.Client{Transport: countingTransport{base: http.DefaultTransport, attempts: &attempts}}
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("dispatch attempts = %d, want exactly 1 (no retry)", got)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_KeyedTwinRelaysNotSynthesizes pins the twin boundary
// from the other side: a keyed trailing-slash twin does NOT hit the
// synthesis — it crosses the generic relay with the core bytes (and
// the core headers) intact. The stub Cache-Control differs from both
// synthesized postures so a synthesis would fail this test.
func TestProviders_KeyedTwinRelaysNotSynthesizes(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"providers":[],"local_login_offered":true,"self_service_tokens_offered":true}`,
		header: http.Header{"Cache-Control": []string{"private"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodGet, "/api/v0/auth/providers/")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("keyed twin status = %d, want the relayed 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "private" {
		t.Errorf("keyed twin Cache-Control = %q, want the relayed core value (not synthesis)", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("keyed twin body = %q, want the core bytes %q", body, core.body)
	}
}

// TestProviders_ExplicitNullProvidersFailsClosed pins the null-list
// mapping: an explicit JSON null providers array violates the core
// contract (the field is always a slice, never null), so dest fails
// closed with the permanent 503 rather than synthesizing a posture
// on it.
func TestProviders_ExplicitNullProvidersFailsClosed(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/api/v0/auth/providers" {
			_, _ = io.WriteString(w, `{"providers":null,"local_login_offered":true,"self_service_tokens_offered":true}`)
			return
		}
		_, _ = io.WriteString(w, `{"require_sso":false}`)
	})
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_UnreadableBodyFailsClosed pins the unreadable-body
// mapping on the providers leg with the shared error-body transport.
func TestProviders_UnreadableBodyFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	srv.httpClient = &http.Client{Transport: errBodyRoundTripper{}}
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest(http.MethodGet, "/api/v0/auth/providers?tenant_id=tenant_a", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want fail-closed 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want absent (permanent 503)", got)
	}
}

// TestProviders_UnbuildableFetchFailsClosed pins the defensive
// mapping on the posture fetch: an unbuildable target fails the
// fetch with the sentinel.
func TestProviders_UnbuildableFetchFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	if _, err := srv.fetchProvidersPosture(t.Context(), signInPolicyFetchParams{
		tenant:      "tenant_a",
		query:       "tenant_id=\x7f",
		correlation: "corr-1",
	}); err != errSignInPolicyFetch {
		t.Errorf("fetch error = %v, want the sentinel", err)
	}
}
