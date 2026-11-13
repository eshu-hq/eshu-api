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

// TestDivergenceTerm_LaneFileHygiene is the no-core-import grep proof
// at the source level: the lane files carry no dest-side store, port,
// or table symbol (the pin's graph reader and content reader plus the
// out-of-pin search/list/summary/ledger legs all stay core-side), no
// query/form/path parser, no request-URI sighting, no body-key
// sighting, no logger/tracer reference, and no serialization
// primitive or lock. A store handle, parse, log line, or lock added
// to api/divergence_terminate.go or api/divergence_wave.go fails here
// until it is removed — the stores-stay-core and opaque-bytes gates
// are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestDivergenceTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"divergence_terminate.go", "divergence_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"CodeDivergenceReadAdmission",
			"GraphQuery",
			"ContentStore",
			"ContentReader",
			"ReadStore",
			"DivergenceGroupStats",
			"DivergenceMembers",
			"DriftedFinding",
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
			`"kind"`,
			`"fingerprint"`,
			`"limit"`,
			`"offset"`,
			`"include_tests"`,
			`"top_per_kind"`,
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

// TestDivergenceTermNative_BodyHygiene pins that dest never handles
// the pin's body values: the repository selectors, the kind and
// fingerprint shapes, and the paging caps cross byte-exact to
// the core while no body value reaches dest logs, metrics, or spans.
// The canaries below fail this test on any dest-side parse, log, or
// telemetry of them. Dest reads no body values on this lane — the
// forward carries the raw body untouched — so the only dest-side
// sighting would be a leak.
func TestDivergenceTermNative_BodyHygiene(t *testing.T) {
	t.Parallel()

	const canaryRepo = "repo-canary-9f2c4e"

	core := &stubCore{status: http.StatusOK, body: `{"findings":[]}`}
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
		{divergenceFindingsPath, `{"kind":"exact","repo_id":"` + canaryRepo + `"}`},
		{divergenceInvestigatePath, `{"repo_id":"` + canaryRepo + `","kind":"exact","fingerprint":"fp-1"}`},
		{divergenceReportPath, `{"repo_id":"` + canaryRepo + `","top_per_kind":5}`},
	}
	var bodies []string
	for _, target := range targets {
		req := divergenceAuthedPost(target.path, target.body, "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST %s: status = %d, want 200 from the stub core", target.path, rec.Code)
		}
		bodies = append(bodies, core.lastBody)
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}
	for _, canary := range []string{canaryRepo} {
		if !strings.Contains(strings.Join(bodies, "&"), canary) {
			t.Errorf("upstream bodies = %q, want them to carry canary %q byte-exact", bodies, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryRepo} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: body values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryRepo} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: body values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
