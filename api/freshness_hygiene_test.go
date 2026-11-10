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

// TestFreshnessTerm_LaneFileHygiene is the no-core-import grep proof at
// the source level: the lane files carry no dest-side store, port, or
// table symbol (the pin's store, readers, ledger, summary, and
// snapshot surface all stay core-side), no query/form/path parser, no
// request-URI sighting, no logger/tracer reference, and no
// serialization primitive or lock. A store handle, parse, log line, or
// lock added to api/freshness_terminate.go or api/freshness_wave.go
// fails here until it is removed — the stores-stay-core and
// opaque-bytes gates are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestFreshnessTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"freshness_terminate.go", "freshness_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"StatusStore",
			"ReadStore",
			"LedgerReadEnabled",
			"WithSummaryReader",
			"SummaryReader",
			"Snapshot",
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
			`"scope_id"`,
			`"since_generation_id"`,
			`"service_id"`,
			`"sample_limit"`,
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

// TestFreshnessTermNative_FilterHygiene pins that dest never handles
// the pin's filter values: the scope/repository selectors, the since
// references, and the sample_limit cap cross byte-exact to the core
// while no filter value reaches dest logs, metrics, or spans. The
// canaries below fail this test on any dest-side parse, log, or
// telemetry of them. Dest reads no filter values on this lane — the
// forward carries the raw query untouched — so the only dest-side
// sighting would be a leak.
func TestFreshnessTermNative_FilterHygiene(t *testing.T) {
	t.Parallel()

	const canaryScope = "scope-canary-9f2c4e"
	const canaryGen = "gen-canary-71ab08"
	const canaryService = "svc-canary-4d1a"

	core := &stubCore{status: http.StatusOK, body: `{"data":[],"error":null,"truth":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	targets := []string{
		"/api/v0/freshness/changed-since?scope_id=" + canaryScope + "&since_generation_id=" + canaryGen,
		"/api/v0/freshness/generations?scope_id=" + canaryScope + "&generation_id=" + canaryGen,
		"/api/v0/freshness/services/changed-since?service_id=" + canaryService + "&since_generation_id=" + canaryGen,
	}
	var queries []string
	for _, target := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, authedRequest(http.MethodGet, target))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200 from the stub core", target, rec.Code)
		}
		queries = append(queries, core.lastQuery)
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}
	for _, canary := range []string{canaryScope, canaryGen, canaryService} {
		if !strings.Contains(strings.Join(queries, "&"), canary) {
			t.Errorf("upstream queries = %q, want them to carry canary %q byte-exact", queries, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryScope, canaryGen, canaryService} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: filter values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryScope, canaryGen, canaryService} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: filter values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
