// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
)

// Code-language relay-leg termination conformance (Refs
// eshu-hq/eshu#9669, satellite eshu-hq/eshu-api#98, W6c). TDD record:
// the frozen wave fails RED before the implementation lands
// (codeLanguageWave undefined), and the ahead-of-relay assertions
// fail next (the W6c language-query route still crosses the generic
// relay); both are green with the wave gate registered ahead of the
// generic proxy. Until the lane lands, every language-query call
// crosses the generic byte-relay, so the ahead-of-relay assertions
// fail first.

// TestCodeLanguageWaveFrozen pins the exact surface the satellite
// terminates: the one W6c language-query POST behind
// CodeLanguageQueryReadAdmission v0 (the bounded traversal for the
// graph-backed, graph-first-content, and directory branches, plus
// the three content reads for the content-only branch and the two
// enrichments). A wave change moves the ownership note, the fragment
// reference, and this mirror together — the same frozen-set pattern
// as TestRelationshipWaveFrozen.
func TestCodeLanguageWaveFrozen(t *testing.T) {
	t.Parallel()

	want := []string{
		"POST /api/v0/code/language-query code_language gated go/internal/query/openapi/paths/code/routes.go",
	}
	var got []string
	for _, entry := range codeLanguageWave {
		got = append(got, entry.Method+" "+entry.Path+" "+entry.Family+" "+entry.Serve+" "+entry.Fragment)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("codeLanguageWave holds %d entries, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("codeLanguageWave differs at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// Wave W6c route path, shared by the behavior suites.
const (
	codeLanguageQueryPath = "/api/v0/code/language-query"
	codeLanguageFamily    = "code_language"
)

// codeLanguageBodies are the well-formed request bodies the behavior
// tests drive through the language-query leg: the graph-backed
// function lookup and the content-only type-alias lookup. The
// satellite forwards every shape opaquely; core validates them.
var codeLanguageBodies = []string{
	`{"language":"go","entity_type":"function","query":"handleWire","repo_id":"repo-1"}`,
	`{"language":"go","entity_type":"type_alias","repo_id":"repo-1"}`,
}

// codeLanguageAuthedPost builds a shared-key W6c POST with one of
// the parity bodies, the negotiated shape, and a pinned correlation
// ID.
func codeLanguageAuthedPost(path, body, accept string) *http.Request {
	req := authedRequest(http.MethodPost, path)
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "code-language-parity-1")
	return req
}

// TestCodeLanguageWaveAheadOfRelay pins that the wave route is an
// explicit registration ahead of the generic proxy: a shared-key call
// must file its dispatch on the native lane (viaNative), not the
// relay lane — the leg on the code_language family. Until the lane
// lands, the call crosses the generic relay and this test is RED.
func TestCodeLanguageWaveAheadOfRelay(t *testing.T) {
	t.Parallel()

	for _, body := range codeLanguageBodies {
		t.Run("POST "+codeLanguageQueryPath, func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, codeLanguageAuthedPost(codeLanguageQueryPath, body, "application/json"))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
			}
			if core.lastBody != body {
				t.Errorf("upstream body = %q, want the W6c request %q forwarded intact", core.lastBody, body)
			}
			if core.lastPath != codeLanguageQueryPath {
				t.Errorf("upstream path = %q, want the wave route %q", core.lastPath, codeLanguageQueryPath)
			}
			snap := tel.snapshot()
			lanes, ok := snap.dispatches[codeLanguageFamily]
			if !ok {
				t.Fatalf("family %q has no dispatch rows, want the explicit wave route ahead of relay", codeLanguageFamily)
			}
			if lanes[viaNative] == nil {
				t.Errorf("family %q dispatches = %v, want a native lane row for the shared-key wave call", codeLanguageFamily, lanes)
			}
			if got := lanes[viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1", codeLanguageFamily, got)
			}
		})
	}
}

// TestCodeLanguageSlashSpellingsTerminateNatively pins the never-split
// rule for the wave: every trailing-slash spelling terminates on the
// native lane exactly like its bare spelling.
func TestCodeLanguageSlashSpellingsTerminateNatively(t *testing.T) {
	t.Parallel()

	for _, entry := range codeLanguageWave {
		t.Run(entry.Method+" "+entry.Path+" slash", func(t *testing.T) {
			t.Parallel()

			core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
			mux := http.NewServeMux()
			mux.Handle("/", core.handler())
			upstream := httptest.NewServer(mux)
			defer upstream.Close()

			tel := NewTelemetry(nil)
			srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
			handler := srv.Handler(DefaultV0Sunset)

			req := codeLanguageAuthedPost(entry.Path, codeLanguageBodies[0], "application/json")
			req.URL.Path = entry.Path + "/"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("%s %s/: status = %d, want 200 from the stub core", entry.Method, entry.Path, rec.Code)
			}
			snap := tel.snapshot()
			if got := snap.dispatches[entry.Family][viaNative][outcomeCoreOK]; got != 1 {
				t.Errorf("family %q native core_ok = %d, want 1 for the slash spelling", entry.Family, got)
			}
		})
	}
}

// TestCodeLanguageTerminated_ForwardsCallerBytes pins that the
// terminated reader forwards the caller's method, path, query,
// negotiation headers, credential, and body to the same core route:
// exactly one upstream hit, one family authorizer allow, no dest
// retry, no credential widening — for the W6c leg.
func TestCodeLanguageTerminated_ForwardsCallerBytes(t *testing.T) {
	t.Parallel()

	core := &stubCore{status: http.StatusOK, body: `{"results":[]}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	tel := NewTelemetry(nil)
	srv := NewServer(upstream.URL, skeletonTestKey).WithTelemetry(tel)
	handler := srv.Handler(DefaultV0Sunset)

	req := codeLanguageAuthedPost(codeLanguageQueryPath, codeLanguageBodies[0], EnvelopeMIME)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}
	if core.hits != 1 {
		t.Errorf("upstream hits = %d, want exactly 1 (no dest retry)", core.hits)
	}
	if core.lastMethod != http.MethodPost {
		t.Errorf("upstream method = %q, want POST", core.lastMethod)
	}
	if core.lastPath != codeLanguageQueryPath {
		t.Errorf("upstream path = %q, want the wave path", core.lastPath)
	}
	if core.lastBody != codeLanguageBodies[0] {
		t.Errorf("upstream body = %q, want the caller body intact", core.lastBody)
	}
	if core.lastAccept != EnvelopeMIME {
		t.Errorf("upstream Accept = %q, want the negotiated shape intact", core.lastAccept)
	}
	if core.lastAuth != "Bearer "+skeletonTestKey {
		t.Errorf("upstream Authorization carries the caller credential forward, got %q", core.lastAuth)
	}
	if got := rec.Header().Get("X-Correlation-ID"); got != "code-language-parity-1" {
		t.Errorf("response correlation = %q, want the caller value echoed", got)
	}
}
