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

// TestContentPointReadsTerm_LaneFileHygiene is the no-core-import grep
// proof at the source level: the lane files carry no dest-side store,
// port, or table symbol (the pin's store, readers, search/list legs,
// and scoped-entity leg all stay core-side), no query/form/path
// parser, no request-URI sighting, no logger/tracer reference, and no
// serialization primitive or lock. A store handle, parse, log line, or
// lock added to api/content_terminate.go or api/content_wave.go fails
// here until it is removed — the stores-stay-core and opaque-bytes
// gates are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestContentPointReadsTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"content_terminate.go", "content_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"ContentStore",
			"ContentReader",
			"ReadStore",
			"SearchFileContent",
			"SearchEntityContent",
			"ListRepoFiles",
			"ListRepoEntities",
			"GetEntityContentInRepositories",
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
			`"repo_id"`,
			`"relative_path"`,
			`"entity_id"`,
			`"start_line"`,
			`"end_line"`,
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

// TestContentPointReadsTermNative_BodyHygiene pins that dest never
// handles the pin's body values: the repository selectors, the entity
// selector, and the line-range edges cross byte-exact to the core
// while no body value reaches dest logs, metrics, or spans. The
// canaries below fail this test on any dest-side parse, log, or
// telemetry of them. Dest reads no body values on this lane — the
// forward carries the raw body untouched — so the only dest-side
// sighting would be a leak.
func TestContentPointReadsTermNative_BodyHygiene(t *testing.T) {
	t.Parallel()

	const canaryRepo = "repo-canary-9f2c4e"
	const canaryPath = "src/canary-71ab08.go"
	const canaryEntity = "entity-canary-4d1a"

	core := &stubCore{status: http.StatusOK, body: `{"repo_id":"repo-canary-9f2c4e"}`}
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
		{"/api/v0/content/files/read", `{"repo_id":"` + canaryRepo + `","relative_path":"` + canaryPath + `"}`},
		{"/api/v0/content/files/lines", `{"repo_id":"` + canaryRepo + `","relative_path":"` + canaryPath + `","start_line":2,"end_line":4}`},
		{"/api/v0/content/entities/read", `{"entity_id":"` + canaryEntity + `"}`},
	}
	var bodies []string
	for _, target := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, contentAuthedPost(target.path, target.body, "application/json"))
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200 from the stub core", target.path, rec.Code)
		}
		bodies = append(bodies, core.lastBody)
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}
	for _, canary := range []string{canaryRepo, canaryPath, canaryEntity} {
		if !strings.Contains(strings.Join(bodies, "&"), canary) {
			t.Errorf("upstream bodies = %q, want them to carry canary %q byte-exact", bodies, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryRepo, canaryPath, canaryEntity} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: body values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryRepo, canaryPath, canaryEntity} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: body values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
