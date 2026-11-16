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

// TestG2Term_LaneFileHygiene is the no-core-import grep proof at the
// source level: the lane files carry no dest-side store, port, or
// table symbol (the pins' readers all stay core-side), no pin
// admission type (the 5 v0 pins are cited by name in comments and
// contracts/g2-wave.md only, never imported), no query/form/path
// parser, no request-URI sighting, no path-param key sighting, no
// logger/tracer reference, and no serialization primitive or lock.
// A store handle, parse, log line, lock, or pin import added to
// api/g2_terminate.go or api/g2_wave.go fails here until it is
// removed — the stores-stay-core and opaque-bytes gates are enforced
// by this test, not by review alone. scripts/check-parity.sh gates
// the same list per lane.
func TestG2Term_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"g2_terminate.go", "g2_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"ExtractionReadinessReadAdmission",
			"FactSchemaReadAdmission",
			"SurfaceInventoryReadAdmission",
			"VisualizationReadAdmission",
			"ManifestDependenciesReadAdmission",
			"postgres",
			"ReadStore",
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
			`"family"`,
			`"fact_kind"`,
			"slog.",
			"tracing.",
			"span.",
			"json.Marshal",
			"sync.",
			"Mutex",
		} {
			if strings.Contains(body, forbidden) {
				t.Errorf("%s contains %q: lane must stay store-free, pin-import-free, parse-free, log-free, and lock-free", file, forbidden)
			}
		}
	}
}

// TestG2TermNative_OpaqueHygiene pins that dest never handles the
// pin's selector values: the {family} and {fact_kind} path segments,
// the derive POST body, and the raw query cross byte-exact to the
// core while no selector value reaches dest logs, metrics, or
// spans. The canaries below fail this test on any dest-side parse,
// log, or telemetry of them. Dest reads no selector values on this
// lane — the forward carries the raw path, query, and body
// untouched — so the only dest-side sighting would be a leak.
func TestG2TermNative_OpaqueHygiene(t *testing.T) {
	t.Parallel()

	const canaryFamily = "fam-canary-9f2c4e"
	const canaryKind = "kind-canary-71ab08"
	const canaryQuery = "probe-canary-4d1a"
	const canaryBody = "body-canary-77e5"

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

	targets := []struct {
		method string
		target string
		body   string
	}{
		{http.MethodGet, "/api/v0/collector-extraction-readiness?" + canaryQuery + "=1", ""},
		{http.MethodGet, "/api/v0/collector-extraction-readiness/" + canaryFamily + "?" + canaryQuery + "=1", ""},
		{http.MethodGet, "/api/v0/fact-schema-versions?" + canaryQuery + "=1", ""},
		{http.MethodGet, "/api/v0/fact-schema-versions/" + canaryKind + "?" + canaryQuery + "=1", ""},
		{http.MethodGet, "/api/v0/surface-inventory?" + canaryQuery + "=1", ""},
		{http.MethodPost, "/api/v0/visualizations/derive?" + canaryQuery + "=1", `{"packet":"` + canaryBody + `"}`},
		{http.MethodGet, "/api/v0/repository/manifest-dependencies?" + canaryQuery + "=1", ""},
	}
	for _, leg := range targets {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, g2AuthedReq(leg.method, leg.target, leg.body))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200 from the stub core", leg.method, leg.target, rec.Code)
		}
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}

	logged := logs.String()
	for _, leaked := range []string{canaryFamily, canaryKind, canaryQuery, canaryBody} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: selector values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryFamily, canaryKind, canaryQuery, canaryBody} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: selector values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
