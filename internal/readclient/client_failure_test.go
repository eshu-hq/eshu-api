// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package readclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// truncatedBodyCore answers with the given status and a Content-Length
// larger than the bytes it writes, then drops the connection, so the
// client's body read fails with io.ErrUnexpectedEOF.
func truncatedBodyCore(t *testing.T, status int) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, "partial")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
	}))
}

// TestDo_BodyReadFailureOn2xxIsAnError pins that a success status whose
// body is cut short fails the call instead of returning a partial body.
func TestDo_BodyReadFailureOn2xxIsAnError(t *testing.T) {
	t.Parallel()

	core := truncatedBodyCore(t, http.StatusOK)
	defer core.Close()

	client := NewClient(DefaultConfig(core.URL, "core-key"))
	raw, err := client.Do(context.Background(), "repositories", nil)
	if err == nil {
		t.Fatalf("Do() = %q, nil; want a read error", raw)
	}
	if !strings.Contains(err.Error(), "readclient: read repositories") {
		t.Errorf("Do() error = %v, want a wrapped read error", err)
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("Do() error = %v, want it to wrap io.ErrUnexpectedEOF", err)
	}
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		t.Errorf("Do() error is a StatusError (%v): the status was 2xx", err)
	}
}

// TestDo_BodyReadFailureOnErrorStatusKeepsStatus pins that the status wins
// over a failed body read: the StatusError carries the refusal and drops
// the unreliable partial snippet.
func TestDo_BodyReadFailureOnErrorStatusKeepsStatus(t *testing.T) {
	t.Parallel()

	core := truncatedBodyCore(t, http.StatusBadGateway)
	defer core.Close()

	client := NewClient(DefaultConfig(core.URL, "core-key"))
	_, err := client.Do(context.Background(), "repositories", nil)
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("Do() error = %v, want a *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want 502", statusErr.StatusCode)
	}
	if statusErr.Snippet != "" {
		t.Errorf("Snippet = %q, want empty after a failed body read", statusErr.Snippet)
	}
}

// TestDo_BadBaseURLFailsBuildingRequest pins the request-build failure: an
// unparsable origin returns a wrapped build error and makes no network
// call.
func TestDo_BadBaseURLFailsBuildingRequest(t *testing.T) {
	t.Parallel()

	client := NewClient(DefaultConfig("http://[::1", "core-key"))
	_, err := client.Do(context.Background(), "repositories", nil)
	if err == nil || !strings.Contains(err.Error(), "readclient: build request for repositories") {
		t.Fatalf("Do() error = %v, want a wrapped build-request error", err)
	}
}

// TestDo_OmitsAuthorizationWithoutKey pins that an empty key sends no
// Authorization header at all, rather than a bare "Bearer".
func TestDo_OmitsAuthorizationWithoutKey(t *testing.T) {
	t.Parallel()

	var gotAuth []string
	var gotQuery string
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Values("Authorization")
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{}`)
	}))
	defer core.Close()

	client := NewClient(DefaultConfig(core.URL, ""))
	if _, err := client.Do(context.Background(), "repositories", nil); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if len(gotAuth) != 0 {
		t.Errorf("Authorization = %v, want no header", gotAuth)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want none for a nil query", gotQuery)
	}
}
