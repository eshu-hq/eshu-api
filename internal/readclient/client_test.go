// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package readclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestDefaultConfig_PinsCensusBounds locks the client to the census
// bounds: a 30s timeout and a 256KiB body budget. Dispatch tuning changes
// these deliberately, never by drift.
func TestDefaultConfig_PinsCensusBounds(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("https://core:8443", "key")
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.ResponseByteBudget != 256*1024 {
		t.Errorf("ResponseByteBudget = %d, want 262144", cfg.ResponseByteBudget)
	}
}

// TestDo_SendsEnvelopeAndAuth spins a stub core and pins the request
// shape: GET under /api/v0/ with the envelope Accept header and the
// bearer key, and the raw body back on success.
func TestDo_SendsEnvelopeAndAuth(t *testing.T) {
	t.Parallel()

	var gotAccept, gotAuth, gotPath, gotQuery string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer core.Close()

	client := NewClient(DefaultConfig(core.URL+"/", "core-key"))
	query := url.Values{"limit": []string{"5"}}
	raw, err := client.Do(context.Background(), "repositories", query)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if string(raw) != `{"status":"ok"}` {
		t.Errorf("Do() body = %q, want the stub body", raw)
	}
	if gotPath != "/api/v0/repositories" {
		t.Errorf("path = %q, want /api/v0/repositories", gotPath)
	}
	if gotQuery != "limit=5" {
		t.Errorf("query = %q, want limit=5", gotQuery)
	}
	if gotAccept != EnvelopeMIME {
		t.Errorf("Accept = %q, want the envelope MIME", gotAccept)
	}
	if gotAuth != "Bearer core-key" {
		t.Errorf("Authorization = %q, want the bearer key", gotAuth)
	}
}

// TestDo_StatusErrorClipsBody pins the failure shape: a non-2xx answer
// returns a StatusError carrying the status, route, and a clipped body
// snippet — never the whole body.
func TestDo_StatusErrorClipsBody(t *testing.T) {
	t.Parallel()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer core.Close()

	client := NewClient(DefaultConfig(core.URL, "core-key"))
	_, err := client.Do(context.Background(), "repositories", nil)
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("Do() error = %v, want a *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want 400", statusErr.StatusCode)
	}
	if statusErr.Route != "repositories" {
		t.Errorf("Route = %q, want 'repositories'", statusErr.Route)
	}
	if len(statusErr.Snippet) != errorSnippetBytes {
		t.Errorf("Snippet length = %d, want the %d-byte clip", len(statusErr.Snippet), errorSnippetBytes)
	}
}

// TestDo_OverBudgetFailsClosed pins the budget: a body larger than the
// configured budget fails the call with ErrOverBudget instead of growing
// this server.
func TestDo_OverBudgetFailsClosed(t *testing.T) {
	t.Parallel()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("y", 100)))
	}))
	defer core.Close()

	cfg := DefaultConfig(core.URL, "core-key")
	cfg.ResponseByteBudget = 10
	client := NewClient(cfg)
	_, err := client.Do(context.Background(), "repositories", nil)
	if !errors.Is(err, ErrOverBudget) {
		t.Fatalf("Do() error = %v, want ErrOverBudget", err)
	}
}

// TestStatusError_DescribesRoute pins the operator-facing string: route
// plus status, so a failing call is diagnosable from the message alone.
func TestStatusError_DescribesRoute(t *testing.T) {
	t.Parallel()

	err := &StatusError{StatusCode: http.StatusBadGateway, Route: "repositories", Snippet: "upstream down"}
	if got := err.Error(); got != "readclient: route repositories answered HTTP 502" {
		t.Errorf("Error() = %q, want route plus status", got)
	}
}

// TestNewClient_NormalizesZeroValue pins the promised bounds on the
// zero-value path: a bare Config still gets the census timeout, and a
// negative budget falls back instead of failing every call.
func TestNewClient_NormalizesZeroValue(t *testing.T) {
	t.Parallel()

	bare := NewClient(Config{BaseURL: "https://core:8443"})
	if bare.cfg.Timeout != DefaultTimeout {
		t.Errorf("zero-value Timeout = %v, want DefaultTimeout %v", bare.cfg.Timeout, DefaultTimeout)
	}
	negative := NewClient(Config{BaseURL: "https://core:8443", ResponseByteBudget: -1})
	if negative.cfg.ResponseByteBudget != DefaultResponseByteBudget {
		t.Errorf("negative budget = %d, want DefaultResponseByteBudget %d", negative.cfg.ResponseByteBudget, DefaultResponseByteBudget)
	}
}

// TestDo_RejectsBadRoutes pins route validation: separators, traversal,
// scheme/host smuggling, and the empty route all fail before any network
// call, so a bad route can never redirect the client or escape the
// read-API prefix.
func TestDo_RejectsBadRoutes(t *testing.T) {
	t.Parallel()

	client := NewClient(DefaultConfig("http://127.0.0.1:1", "core-key"))
	for _, route := range []string{"", "a/b", "/repositories", `a\b`, "..", "../x", "http://evil", "a:b", "a@b", "a?b", "a#b"} {
		if _, err := client.Do(context.Background(), route, nil); err == nil || !strings.Contains(err.Error(), "invalid route") {
			t.Errorf("Do(%q) error = %v, want an invalid-route error", route, err)
		}
	}
}

// TestDo_UnreachableCoreWraps pins the transport failure shape: a dead
// origin returns a wrapped call error, never a StatusError and never a
// panic.
func TestDo_UnreachableCoreWraps(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig("http://127.0.0.1:1", "core-key")
	cfg.Timeout = 2 * time.Second
	client := NewClient(cfg)
	_, err := client.Do(context.Background(), "repositories", nil)
	if err == nil || !strings.Contains(err.Error(), "readclient: call repositories") {
		t.Fatalf("Do() error = %v, want a wrapped call error", err)
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		t.Fatalf("Do() error is a StatusError (%v): a dead origin has no status", err)
	}
}
