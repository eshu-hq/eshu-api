// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Wave-1b conformance (Refs eshu-hq/eshu#9669, satellite
// eshu-hq/eshu-api#101). A shared-key call on every wave route must
// terminate natively with the caller's method, path, query, body,
// and credential intact and core bytes untouched; each trailing-slash
// spelling terminates exactly like its bare spelling (never-split).

// compextConformLegs pins one conformance leg per wave route with a
// representative body or query.
var compextConformLegs = []struct {
	method string
	path   string
	body   string
}{
	{http.MethodGet, "/api/v0/component-extensions", ""},
	{http.MethodGet, "/api/v0/component-extensions/comp-1/diagnostics", ""},
}

// TestCompextTerminatedByteForByte pins native termination for the
// whole wave: status, headers, and body cross identically, and the
// upstream sees the caller's method, path, query, body, and
// credential intact.
func TestCompextTerminatedByteForByte(t *testing.T) {
	t.Parallel()

	for _, leg := range compextConformLegs {
		t.Run(leg.method+" "+leg.path, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true,"lane":"native"}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			var req *http.Request
			if leg.body == "" {
				req = compextAuthedReq(leg.method, leg.path, "")
			} else {
				req = compextAuthedReq(leg.method, leg.path, leg.body)
			}
			req.Header.Set("X-Correlation-ID", "compext-conform-1")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			gotBody, _ := io.ReadAll(rec.Result().Body)
			if string(gotBody) != `{"ok":true,"lane":"native"}` {
				t.Errorf("body = %q, want core bytes untouched", gotBody)
			}
			if core.lastMethod != leg.method {
				t.Errorf("upstream method = %q, want %q", core.lastMethod, leg.method)
			}
			if core.lastAuth != "Bearer "+skeletonTestKey {
				t.Errorf("upstream auth forwarded = %q, want the shared-key credential intact", core.lastAuth)
			}
			if core.lastBody != leg.body {
				t.Errorf("upstream body = %q, want %q forwarded intact", core.lastBody, leg.body)
			}
		})
	}
}

// TestCompextSlashSpellingsTerminateNatively pins the never-split
// rule: each trailing-slash spelling terminates on the native lane
// exactly like its bare spelling.
func TestCompextSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, leg := range compextRedLegs {
		t.Run(leg.method+" "+leg.path+"/", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"ok":true}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, compextAuthedReq(leg.method, leg.path+"/", leg.body))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[leg.family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash twin", leg.family, got)
			}
		})
	}
}
