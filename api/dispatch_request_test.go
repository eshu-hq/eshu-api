// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestDispatch_UnbuildableUpstreamRequestAnswers503 pins the request-build
// failure branch: a method the HTTP stack cannot encode never reaches the
// core and answers the permanent local 503 backend_unavailable shape.
func TestDispatch_UnbuildableUpstreamRequestAnswers503(t *testing.T) {
	t.Parallel()

	core := &stubCore{t: t, status: http.StatusOK, body: `{}`}
	origin := httptest.NewServer(core.handler())
	defer origin.Close()

	srv := NewServer(origin.URL, "key")
	req := &http.Request{
		Method: "BAD METHOD",
		URL:    &url.URL{Path: "/api/v0/repositories"},
		Header: http.Header{},
	}
	rec := httptest.NewRecorder()
	srv.dispatch(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("dispatch status = %d, want 503", rec.Code)
	}
	decoded := decodeBody(t, rec)
	if decoded["error_code"] != backendUnavailableCode {
		t.Errorf("error_code = %v, want %q", decoded["error_code"], backendUnavailableCode)
	}
	if core.hits != 0 {
		t.Errorf("core hits = %d, want 0: an unbuildable request must not reach the core", core.hits)
	}
}
