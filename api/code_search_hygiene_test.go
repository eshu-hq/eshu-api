// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestCodeSearchTerm_LaneFileHygiene is the no-core-import grep proof
// at the source level: the lane files carry no dest-side store, port,
// or table symbol (the pin's graph reader, content reader, and
// out-of-pin legs all stay core-side), no query/form/path parser, no
// request-URI sighting, no logger/tracer reference, and no
// serialization primitive or lock. A store handle, parse, log line, or
// lock added to api/code_search_terminate.go or api/code_search_wave.go
// fails here until it is removed — the stores-stay-core and
// opaque-bytes gates are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestCodeSearchTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"code_search_terminate.go", "code_search_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"GraphQuery",
			"ContentReader",
			"ReadStore",
			"RunSingle",
			"SearchEntityNames",
			"SearchSymbols",
			"SearchCodeCandidates",
			"SearchEntitiesByName",
			"SearchEntityContent",
			"ListRepoEntitiesByIDs",
			"database/sql",
			"sql.DB",
			"sql.Open",
			".Query(",
			"ParseForm(",
			"FormValue(",
			"PathValue(",
			".RequestURI",
			`"Location"`,
			`"Set-Cookie"`,
			`"query"`,
			`"repo_id"`,
			`"language"`,
			`"limit"`,
			`"offset"`,
			`"exact"`,
			`"rank_cursor"`,
			`"symbol"`,
			`"match_mode"`,
			"slog.",
			"tracing.",
			"span.",
			"json.Marshal",
			"sync.",
			"Mutex",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q: lane must stay store-free, parse-free, log-free, and lock-free", file, forbidden)
			}
		}
	}
}

// TestCodeSearchTermNative_BodyHygiene pins that dest never handles
// the pin's body values: the query text, the repository selector, and
// the symbol selector cross byte-exact to the core while no body value
// reaches dest logs, metrics, or spans. The canaries below fail this
// test on any dest-side parse, log, or telemetry of them. Dest reads
// no body values on this lane — the forward carries the raw body
// untouched — so the only dest-side sighting would be a leak.
func TestCodeSearchTermNative_BodyHygiene(t *testing.T) {
	t.Parallel()

	const canaryQuery = "render-canary-9f2c4e"
	const canaryRepo = "repo-canary-71ab08"
	const canarySymbol = "RenderCanary_4d1a"

	core := &stubCore{status: http.StatusOK, body: `{"query":"render-canary-9f2c4e"}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	targets := []struct {
		path string
		body string
	}{
		{"/api/v0/code/search", `{"query":"` + canaryQuery + `"}`},
		{"/api/v0/code/search", `{"repo_id":"` + canaryRepo + `","query":"` + canaryQuery + `","language":"go","exact":true}`},
		{"/api/v0/code/symbols/search", `{"symbol":"` + canarySymbol + `"}`},
		{"/api/v0/code/symbols/search", `{"query":"` + canarySymbol + `","repo_id":"` + canaryRepo + `","match_mode":"fuzzy"}`},
	}
	var bodies []string
	for _, target := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, codeSearchAuthedPost(target.path, target.body, "application/json"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200 from the stub core", target.path, rec.Code)
		}
		bodies = append(bodies, core.lastBody)
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}
	for _, canary := range []string{canaryQuery, canaryRepo, canarySymbol} {
		if !strings.Contains(strings.Join(bodies, "&"), canary) {
			t.Errorf("upstream bodies = %q, want them to carry canary %q byte-exact", bodies, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryQuery, canaryRepo, canarySymbol} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: body values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryQuery, canaryRepo, canarySymbol} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: body values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
