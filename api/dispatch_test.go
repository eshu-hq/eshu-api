// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eshu-hq/eshu-api/internal/readclient"
)

// stubCore records what the dispatch forwarded and answers a fixed
// response. One stub covers every route family: dispatch is a
// byte-for-byte relay, so families differ only in method, path, and
// body, all exercised below.
type stubCore struct {
	t          *testing.T
	status     int
	body       string
	header     http.Header
	hits       int
	lastMethod string
	lastPath   string
	lastQuery  string
	lastBody   string
	lastAccept string
	lastAuth   string
}

func (s *stubCore) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits++
		s.lastMethod = r.Method
		s.lastPath = r.URL.Path
		s.lastQuery = r.URL.RawQuery
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			s.t.Errorf("stub read body: %v", err)
		}
		s.lastBody = string(raw)
		s.lastAccept = r.Header.Get("Accept")
		s.lastAuth = r.Header.Get("Authorization")
		for key, values := range s.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.body)
	})
}

// TestDispatch_ReadBatchRelaysByteForByte pins the read families: an
// authenticated GET arrives at the core with method, path, and query
// intact, and the core's status, headers, and body come back untouched.
func TestDispatch_ReadBatchRelaysByteForByte(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusOK,
		body:   `{"data":[{"id":"r1"}],"error":null,"truth":null}`,
		header: http.Header{"Content-Type": []string{"application/json; charset=utf-8"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest("GET", "/api/v0/repositories?limit=5")
	req.Header.Set("Accept", EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.hits != 1 {
		t.Fatalf("upstream hits = %d, want 1", core.hits)
	}
	if core.lastMethod != http.MethodGet {
		t.Errorf("upstream method = %q, want GET", core.lastMethod)
	}
	if core.lastPath != "/api/v0/repositories" {
		t.Errorf("upstream path = %q, want /api/v0/repositories", core.lastPath)
	}
	if core.lastQuery != "limit=5" {
		t.Errorf("upstream query = %q, want limit=5", core.lastQuery)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the caller's envelope MIME", core.lastAccept)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Errorf("Deprecation = %q, want 'true' on v0 dispatch", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("body = %q, want the core bytes %q", body, core.body)
	}
}

// TestDispatch_MutationBatchRelaysMethodAndBody pins the mutation
// families: POST method and body cross to the core, and the core's
// creation status and body come back untouched.
func TestDispatch_MutationBatchRelaysMethodAndBody(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusCreated,
		body:   `{"suppression_id":"s1","generation_id":7,"status":"created"}`,
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"suppression_id":"s1","justification":"accepted_risk"}`
	req := authedRequest("POST", "/api/v0/supply-chain/impact/suppressions")
	req.Body = io.NopCloser(strings.NewReader(payload))
	req.ContentLength = int64(len(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != "/api/v0/supply-chain/impact/suppressions" {
		t.Errorf("upstream path = %q, want the suppression route", core.lastPath)
	}
	if core.lastBody != payload {
		t.Errorf("upstream body = %q, want %q", core.lastBody, payload)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("body = %q, want the core bytes %q", body, core.body)
	}
}

// TestDispatch_V1RewritesBeforeProxy pins the alias ahead of dispatch:
// the core sees the rewritten v0 path and the response carries no
// deprecation headers.
func TestDispatch_V1RewritesBeforeProxy(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v1/repositories"))

	if core.lastPath != "/api/v0/repositories" {
		t.Errorf("upstream path = %q, want the rewritten /api/v0 path", core.lastPath)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("Deprecation = %q, want empty on v1", got)
	}
	if got := rec.Header().Get("Sunset"); got != "" {
		t.Errorf("Sunset = %q, want empty on v1", got)
	}
}

// TestDispatch_UnauthenticatedNeverReachesCore pins the gate ahead of
// dispatch: no credential means 401 and zero upstream contact.
func TestDispatch_UnauthenticatedNeverReachesCore(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := httptest.NewRequest("GET", "/api/v0/repositories", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if core.hits != 0 {
		t.Errorf("upstream hits = %d, want 0 (gate runs before dispatch)", core.hits)
	}
}

// TestDispatch_UpstreamErrorsRelayUntouched pins error passthrough: a
// core 503 marked retryable keeps its status and Retry-After; dispatch
// never rewrites the core's error contract.
func TestDispatch_UpstreamErrorsRelayUntouched(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusServiceUnavailable,
		body:   `{"error_code":"backend_unavailable","message":"reader fenced","correlation_id":"c1"}`,
		header: http.Header{"Retry-After": []string{"5"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/status/collectors"))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want the core's 503", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "5" {
		t.Errorf("Retry-After = %q, want the core's value relayed", got)
	}
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != core.body {
		t.Errorf("body = %q, want the core bytes %q", body, core.body)
	}
}

// TestDispatch_RedirectsRelayUnfollowed pins that dispatch never
// follows the core: a 307 comes back as a 307.
func TestDispatch_RedirectsRelayUnfollowed(t *testing.T) {
	t.Parallel()

	core := &stubCore{
		status: http.StatusTemporaryRedirect,
		header: http.Header{"Location": []string{"/api/v0ELSEWHERE"}},
	}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/auth/oidc/login"))

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want the core's 307 unfollowed", rec.Code)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no follow-up fetch)", core.hits)
	}
}

// TestDispatch_CoreDownAnswersLocal503 pins the only synthetic error
// dispatch still owns: an unreachable core answers the permanent 503
// backend_unavailable with no Retry-After, in both wire shapes.
func TestDispatch_CoreDownAnswersLocal503(t *testing.T) {
	t.Parallel()

	srv := NewServer("http://127.0.0.1:1", skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	for _, accept := range []string{"", EnvelopeMIME} {
		req := authedRequest("GET", "/api/v0/repositories")
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("Accept %q: status = %d, want 503", accept, rec.Code)
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Errorf("Accept %q: Retry-After is set on the permanent 503", accept)
		}
		body, err := io.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatalf("Accept %q: read body: %v", accept, err)
		}
		if accept == "" {
			if !strings.Contains(string(body), `"backend_unavailable"`) {
				t.Errorf("plain body = %s, want backend_unavailable", body)
			}
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatalf("Accept %q: decode body %q: %v", accept, body, err)
		}
		errSection, ok := decoded["error"].(map[string]any)
		if !ok || errSection["code"] != "backend_unavailable" {
			t.Errorf("Accept %q: envelope error = %v, want backend_unavailable", accept, decoded["error"])
		}
	}
}

// TestDispatch_HopHeadersStayOnThisHop pins transport hygiene: a
// Connection token list and a hop header never reach the core.
func TestDispatch_HopHeadersStayOnThisHop(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
	seen := http.Header{}
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		core.hits++
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, core.body)
	}))
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	srv := NewServer(upstream.URL, skeletonTestKey)
	handler := srv.Handler(DefaultV0Sunset)

	req := authedRequest("GET", "/api/v0/repositories")
	req.Header.Set("Connection", "X-Custom-Hop")
	req.Header.Set("X-Custom-Hop", "yes")
	req.Header.Set("Transfer-Encoding", "chunked")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := seen.Get("X-Custom-Hop"); got != "" {
		t.Errorf("upstream X-Custom-Hop = %q, want it stripped with its Connection token", got)
	}
	if got := seen.Get("Transfer-Encoding"); got != "" {
		t.Errorf("upstream Transfer-Encoding = %q, want it stripped", got)
	}
	if got := seen.Get("Authorization"); got == "" {
		t.Errorf("upstream Authorization is empty, want the caller credential forwarded")
	}
}

// TestDispatch_UnconfiguredOriginFailsClosed pins the fail-closed
// origin: without a valid core origin every authenticated /api/* call
// answers the local permanent 503 and dials nothing.
func TestDispatch_UnconfiguredOriginFailsClosed(t *testing.T) {
	t.Parallel()

	for _, origin := range []string{"", "://missing-scheme-host", "unix:///nope"} {
		srv := NewServer(origin, skeletonTestKey)
		handler := srv.Handler(DefaultV0Sunset)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/repositories"))

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("origin %q: status = %d, want 503", origin, rec.Code)
			continue
		}
		body, _ := io.ReadAll(rec.Result().Body)
		if !strings.Contains(string(body), `"backend_unavailable"`) {
			t.Errorf("origin %q: body = %s, want backend_unavailable", origin, body)
		}
		if rec.Result().Header.Get("Retry-After") != "" {
			t.Errorf("origin %q: Retry-After is set on the permanent 503", origin)
		}
	}
}

// TestDispatch_TimeoutMatchesReadContract pins the dispatch bound to
// the versioned read-contract client bound: a slower core answers on
// its own side rather than holding a worker past the census bound.
func TestDispatch_TimeoutMatchesReadContract(t *testing.T) {
	t.Parallel()

	if dispatchTimeout != readclient.DefaultTimeout {
		t.Fatalf("dispatchTimeout = %v, want the read-contract bound %v",
			dispatchTimeout, readclient.DefaultTimeout)
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	srv := NewServer(slow.URL, skeletonTestKey)
	srv.httpClient = &http.Client{Timeout: 50 * time.Millisecond}
	handler := srv.Handler(DefaultV0Sunset)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authedRequest("GET", "/api/v0/repositories"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 after the dispatch bound", rec.Code)
	}
}
