// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// retireUnclaimedPayload is the caller body the byte-parity tests
// forward: a trimmed kind/instance pair with an operator note, matching
// the core request shape the pin freezes.
const retireUnclaimedPayload = `{"collector_kind":"sbom_attestation","collector_instance_id":"sbom-gone","operator_note":"dark scope cleanup"}`

// TestRetireUnclaimedNative_UnauthenticatedIs401 pins that the native
// route keeps the dest 401 shape: envelope callers get the envelope
// error, everyone else the plain error_code body, both with the bare
// Bearer challenge and no WWW-Authenticate parameters.
func TestRetireUnclaimedNative_UnauthenticatedIs401(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed", strings.NewReader(`{}`))
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Accept %q: status = %d, want 401", accept, rec.Code)
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("Accept %q: WWW-Authenticate = %q, want bare Bearer", accept, got)
		}
	}
}

// TestRetireUnclaimedNative_AuthorizerRejectsNonAllScopes pins the
// family 403 the dest authorizer adds: a non-AllScopes identity never
// reaches the core. Dest has no scoped/session resolver, so the deny
// branch is unreachable in production (the shared key maps to AllScopes
// in core, which re-judges admin scope as backstop); the test pins the
// documented shape, not a live path.
func TestRetireUnclaimedNative_AuthorizerRejectsNonAllScopes(t *testing.T) {
	t.Parallel()

	if !authorizeRetireUnclaimedAdmin(true) {
		t.Errorf("authorizeRetireUnclaimedAdmin(true) = deny, want allow (shared key is AllScopes)")
	}
	if authorizeRetireUnclaimedAdmin(false) {
		t.Errorf("authorizeRetireUnclaimedAdmin(false) = allow, want deny (family 403)")
	}
}

// TestRetireUnclaimedNative_DenyBranchShapePinned pins the deny shape
// for the day dest gains a scoped/session resolver: a non-AllScopes
// identity is denied with the documented shape and the core is never
// reached, in both negotiated shapes. The branch is unreachable on the
// live route today (the route passes true; the live 403s are core's),
// so this pins the contract, not a live path.
func TestRetireUnclaimedNative_DenyBranchShapePinned(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://core.invalid", skeletonTestKey)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		srv.retireUnclaimedWriteThroughAs(rec, req, false)

		if rec.Code != http.StatusForbidden {
			t.Errorf("Accept %q: status = %d, want 403", accept, rec.Code)
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), "all-scopes operator authorization is required") {
			t.Errorf("Accept %q: body = %q, want the family-403 message", accept, body)
		}
	}
}

// TestRetireUnclaimedNative_WriteThroughRelaysCoreBytes pins the native
// write-through: method, path, query, headers (including the original
// Authorization header), and body forward to the same core route, and
// the core's status, headers, and body come back untouched across the
// pinned outcomes — 200 with the count/items wire shape in work_item_id
// order, the 400 validation rejection, the 404 unknown-instance and
// 409 still-configured refusals naming the pair, the core 403, the 500
// fixed failure message, and the core 503 (whose Retry-After relays,
// proving dest never invents or strips the core's retry signal).
func TestRetireUnclaimedNative_WriteThroughRelaysCoreBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		header http.Header
		body   string
	}{
		{"retired", http.StatusOK, nil, `{"count":2,"items":[{"work_item_id":"w-1","scope_id":"s-1"},{"work_item_id":"w-2","scope_id":"s-2"}]}`},
		{"empty noop", http.StatusOK, nil, `{"count":0,"items":[]}`},
		{"blank kind", http.StatusBadRequest, nil, `{"error":"Bad Request","detail":"collector_kind and collector_instance_id are required and must not be empty"}`},
		{"unknown instance", http.StatusNotFound, nil, `{"error":"Not Found","detail":"retire unclaimed workflow work items: unknown collector instance: sbom_attestation/never-existed"}`},
		{"still configured", http.StatusConflict, nil, `{"error":"Conflict","detail":"retire unclaimed workflow work items: collector instance still configured: sbom_attestation/sbom-live"}`},
		{"core 403", http.StatusForbidden, nil, `{"error":"Forbidden","detail":"workflow retirement requires an admin (all-scopes) token"}`},
		{"store fault", http.StatusInternalServerError, nil, `{"error":"Internal Server Error","detail":"retire unclaimed workflow work items request failed"}`},
		{"core 503 retryable", http.StatusServiceUnavailable, http.Header{"Retry-After": []string{"5"}}, `{"error":"Service Unavailable","detail":"workflow retirement request failed"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: tc.status, header: tc.header, body: tc.body}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed?probe=1")
			req.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if core.hits != 1 {
				t.Fatalf("upstream hits = %d, want exactly 1 (no retry on write-through)", core.hits)
			}
			if core.lastMethod != http.MethodPost {
				t.Errorf("upstream method = %q, want POST", core.lastMethod)
			}
			if core.lastPath != "/api/v0/admin/workflow-retire-unclaimed" {
				t.Errorf("upstream path = %q, want the core route", core.lastPath)
			}
			if core.lastQuery != "probe=1" {
				t.Errorf("upstream query = %q, want the caller query relayed", core.lastQuery)
			}
			if core.lastBody != retireUnclaimedPayload {
				t.Errorf("upstream body = %q, want the caller bytes", core.lastBody)
			}
			if !strings.HasPrefix(core.lastAuth, "Bearer ") || core.lastAuth == "Bearer " {
				t.Errorf("upstream Authorization = %q, want the forwarded Bearer [REDACTED]", core.lastAuth)
			}
			if rec.Code != tc.status {
				t.Errorf("status = %d, want relayed %d", rec.Code, tc.status)
			}
			if tc.header != nil {
				if got := rec.Header().Get("Retry-After"); got != tc.header.Get("Retry-After") {
					t.Errorf("Retry-After = %q, want the core's value relayed", got)
				}
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if string(body) != tc.body {
				t.Errorf("body = %q, want the core bytes %q", body, tc.body)
			}
			wantOutcome := outcomeCoreOK
			if tc.status == http.StatusServiceUnavailable {
				wantOutcome = outcomeCoreRetryable
			}
			if got := tel.snapshot().dispatches["retire_unclaimed"][viaNative][wantOutcome]; got != 1 {
				t.Errorf("retire_unclaimed native %s = %d, want 1 (terminated lane, not relay)", wantOutcome, got)
			}
			if got := len(tel.snapshot().dispatches["retire_unclaimed"][viaRelay]); got != 0 {
				t.Errorf("retire_unclaimed relay outcomes = %d, want 0 (native lane only)", got)
			}
		})
	}
}

// TestRetireUnclaimedNative_SuccessOrderAndCount pins the 200 wire
// freeze the pin records: count equals the items served, in
// work_item_id order with exactly the work_item_id and scope_id keys.
func TestRetireUnclaimedNative_SuccessOrderAndCount(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"count":2,"items":[{"work_item_id":"w-1","scope_id":"s-1"},{"work_item_id":"w-2","scope_id":"s-2"}]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	req.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	var decoded struct {
		Count int `json:"count"`
		Items []struct {
			WorkItemID string `json:"work_item_id"`
			ScopeID    string `json:"scope_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode relayed 200: %v", err)
	}
	if decoded.Count != 2 || len(decoded.Items) != 2 {
		t.Fatalf("count/items = %d/%d, want 2/2", decoded.Count, len(decoded.Items))
	}
	for i, want := range []string{"w-1", "w-2"} {
		if decoded.Items[i].WorkItemID != want {
			t.Errorf("items[%d].work_item_id = %q, want %q (port order)", i, decoded.Items[i].WorkItemID, want)
		}
		if decoded.Items[i].ScopeID == "" {
			t.Errorf("items[%d] lacks scope_id", i)
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode relayed 200 as map: %v", err)
	}
	for i, item := range raw["items"].([]any) {
		if len(item.(map[string]any)) != 2 {
			t.Errorf("items[%d] = %v, want exactly work_item_id and scope_id", i, item)
		}
	}
}

// TestRetireUnclaimedNative_CallerCredentialAndCorrelationIntact pins
// that the write-through carries the caller's credential and
// correlation to the core: the upstream Authorization equals the
// caller bearer, the caller X-Correlation-ID arrives unchanged, and a
// caller without one still reaches the core with a minted value.
func TestRetireUnclaimedNative_CallerCredentialAndCorrelationIntact(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var lastAuth, lastCorrelation string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		lastAuth, lastCorrelation = r.Header.Get("Authorization"), r.Header.Get(correlationHeader)
		mu.Unlock()
		if string(raw) != retireUnclaimedPayload {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"count":0,"items":[]}`)
	}))
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	withCallerCorrelation := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	withCallerCorrelation.Header.Set(correlationHeader, "caller-corr-1")
	withCallerCorrelation.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, withCallerCorrelation)
	if rec.Code != http.StatusOK {
		t.Fatalf("caller-correlation call: status = %d, want 200", rec.Code)
	}
	mu.Lock()
	auth, corr := lastAuth, lastCorrelation
	mu.Unlock()
	if auth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization = %q, want the caller [REDACTED]", auth)
	}
	if corr != "caller-corr-1" {
		t.Errorf("upstream correlation = %q, want the caller value intact", corr)
	}

	minted := authedRequest(http.MethodPost, "/api/v0/admin/workflow-retire-unclaimed")
	minted.Body = io.NopCloser(strings.NewReader(retireUnclaimedPayload))
	mintedRec := httptest.NewRecorder()
	handler.ServeHTTP(mintedRec, minted)
	if mintedRec.Code != http.StatusOK {
		t.Fatalf("minted-correlation call: status = %d, want 200", mintedRec.Code)
	}
	mu.Lock()
	mintedCorr := lastCorrelation
	mu.Unlock()
	if mintedCorr == "" {
		t.Errorf("upstream correlation is empty, want a minted value when the caller sends none")
	}
}

// TestRetireUnclaimedNative_UnconfiguredOriginFailsClosed pins the
// fail-closed origin: with no core origin configured the native route
// answers the permanent 503 backend_unavailable instead of panicking,
// in both negotiated shapes and for both spellings of the POST path.
func TestRetireUnclaimedNative_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	srv := NewServer("", skeletonTestKey)
	if srv.core != nil {
		t.Fatalf("NewServer(\"\", key).core = non-nil, want nil (unconfigured origin)")
	}
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{EnvelopeMIME, "application/json"} {
		for _, path := range []string{
			"/api/v0/admin/workflow-retire-unclaimed",
			"/api/v0/admin/workflow-retire-unclaimed/",
		} {
			req := authedRequest(http.MethodPost, path)
			if accept != "" {
				req.Header.Set("Accept", accept)
			}
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Errorf("Accept %q path %q: panicked with %v, want 503", accept, path, recovered)
					}
				}()
				handler.ServeHTTP(rec, req)
			}()

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("Accept %q path %q: status = %d, want 503", accept, path, rec.Code)
			}
			if got := rec.Header().Get("Retry-After"); got != "" {
				t.Errorf("Accept %q path %q: Retry-After = %q, want absent (permanent 503)", accept, path, got)
			}
			body, _ := io.ReadAll(rec.Result().Body)
			if !strings.Contains(string(body), backendUnavailableCode) {
				t.Errorf("Accept %q path %q: body = %q, want backend_unavailable", accept, path, body)
			}
		}
	}
}

// TestRetireUnclaimedNative_BothSpellingsWriteThrough pins that both
// spellings of the POST path stay on the native route: method, path
// (as spelled), and body forward to the same core route.
func TestRetireUnclaimedNative_BothSpellingsWriteThrough(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"/api/v0/admin/workflow-retire-unclaimed",
		"/api/v0/admin/workflow-retire-unclaimed/",
	} {
		core := &stubCore{status: http.StatusOK, body: `{"count":0,"items":[]}`}
		mux := http.NewServeMux()
		mux.Handle("/", core.handler())
		upstream := httptest.NewServer(mux)
		defer upstream.Close()

		srv := NewServer(upstream.URL, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		req := authedRequest(http.MethodPost, path)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if core.hits != 1 || core.lastPath != path {
			t.Errorf("path %q: hits = %d lastPath = %q, want the native route to forward as spelled", path, core.hits, core.lastPath)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("path %q: status = %d, want relayed 200", path, rec.Code)
		}
	}
}

// TestRetireUnclaimedNative_V1AliasOrderingUnchanged pins that the v1
// alias rewrite still lands the retire-unclaimed POST on its native
// handler with the v0 deprecation headers absent.
func TestRetireUnclaimedNative_V1AliasOrderingUnchanged(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"count":0,"items":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest(http.MethodPost, "/api/v1/admin/workflow-retire-unclaimed")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 || core.lastPath != "/api/v0/admin/workflow-retire-unclaimed" {
		t.Errorf("hits = %d path = %q, want alias rewrite to the native v0 route", core.hits, core.lastPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1 (outer layer sees the original path)", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want relayed 200", rec.Code)
	}
}
