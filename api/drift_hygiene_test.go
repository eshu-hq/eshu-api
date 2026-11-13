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

// TestDriftTerm_LaneFileHygiene is the no-core-import grep proof at
// the source level: the lane files carry no dest-side store, port, or
// table symbol (the pin's multi-cloud drift store and the
// out-of-pin graph/content/snapshot/ledger legs all stay core-side),
// no query/form/path parser, no request-URI sighting, no
// logger/tracer reference, and no serialization primitive or lock. A
// store handle, parse, log line, or lock added to
// api/drift_terminate.go or api/drift_wave.go fails here until it is
// removed — the stores-stay-core and opaque-bytes gates are enforced
// by this test, not by review alone. scripts/check-parity.sh gates
// the same list.
func TestDriftTerm_LaneFileHygiene(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"drift_terminate.go", "drift_wave.go"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		body := string(raw)
		for _, forbidden := range []string{
			"postgres",
			"MultiCloudRuntimeDrift",
			"CloudRuntimeDriftReadAdmission",
			"ListActiveMultiCloudRuntimeDriftFindings",
			"CountActiveMultiCloudRuntimeDriftFindings",
			"ReadStore",
			"GraphQuery",
			"ContentReader",
			"RunSingle",
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
			`"provider"`,
			`"limit"`,
			`"offset"`,
			`"cloud_resource_uid"`,
			`"finding_kinds"`,
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

// TestDriftTermNative_BodyHygiene pins that dest never handles the
// pin's body values: the scope identity and the resource selector
// cross byte-exact to the core while no body value reaches dest logs,
// metrics, or spans. The canaries below fail this test on any
// dest-side parse, log, or telemetry of them. Dest reads no body
// values on this lane — the forward carries the raw body untouched —
// so the only dest-side sighting would be a leak.
func TestDriftTermNative_BodyHygiene(t *testing.T) {
	t.Parallel()

	const canaryScope = "cloud-scope-canary-9f2c4e:gcp"
	const canaryUID = "uid-canary-71ab08"

	core := &stubCore{status: http.StatusOK, body: `{"drift_findings":[]}`}
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
		{"POST", driftFindingsPath, `{"scope_id":"` + canaryScope + `"}`},
		{"POST", driftFindingsPath, `{"scope_id":"` + canaryScope + `","provider":"gcp","cloud_resource_uid":"` + canaryUID + `"}`},
		{"GET", driftPacketPath + "?scope_id=" + canaryScope, ""},
		{"GET", driftPacketPath + "?scope_id=" + canaryScope + "&provider=gcp&cloud_resource_uid=" + canaryUID, ""},
	}
	var bodies []string
	for _, target := range targets {
		var req *http.Request
		if target.method == http.MethodPost {
			req = driftAuthedPost(target.body, "application/json")
		} else {
			req = driftAuthedGet(target.target, "application/json")
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: status = %d, want 200 from the stub core", target.method, target.target, rec.Code)
		}
		bodies = append(bodies, core.lastBody+core.lastQuery)
	}

	if got, want := core.hits, len(targets); got != want {
		t.Fatalf("upstream hits = %d, want %d (one fetch per leg)", got, want)
	}
	for _, canary := range []string{canaryScope, canaryUID} {
		if !strings.Contains(strings.Join(bodies, "&"), canary) {
			t.Errorf("upstream bodies/queries = %q, want them to carry canary %q byte-exact", bodies, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryScope, canaryUID} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: body values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryScope, canaryUID} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: body values must never reach metrics\n%s", leaked, rendered)
		}
	}
}
