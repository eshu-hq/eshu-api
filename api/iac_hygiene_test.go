// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestIaCLaneFileHygiene is the no-parse grep proof at the source
// level: the lane file carries no dest-side store, port, or Content
// symbol (the pin's four ContentStore reads and the out-of-pin graph,
// inventory, management, selector, and reachability legs all stay
// core-side), no query/form/path parser, no request-URI sighting, and
// no dest-side body-key sighting — the caller bytes cross opaquely.
// A store handle, parse, or Content symbol added to api/iac.go fails
// here until it is removed: the stores-stay-core and opaque-bytes
// gates are enforced by this test, not by review alone.
// scripts/check-parity.sh gates the same list.
func TestIaCLaneFileHygiene(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("iac.go")
	if err != nil {
		t.Fatalf("read iac.go: %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{
		"ContentStore",
		"ContentReader",
		"ListRepositories",
		"MatchRepositories",
		"ListRepoFiles",
		"GetFileContent",
		"RepositoryCatalogEntry",
		"FileContent",
		"querycontract",
		"postgres",
		"database/sql",
		"sql.DB",
		"sql.Open",
		".Query(",
		"ParseForm(",
		"FormValue(",
		"PathValue(",
		".RequestURI",
		`"repo_id"`,
		`"repo_ids"`,
		`"families"`,
		`"limit"`,
		`"offset"`,
		"slog.",
		"tracing.",
		"span.",
		"json.Marshal",
		"sync.",
		"Mutex",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("iac.go contains %q: lane must stay store-free, parse-free, log-free, and lock-free", forbidden)
		}
	}
}

// TestIaCDeadNative_BodyHygiene pins that dest never handles the dead
// pin's body values: the repo scope crosses byte-exact to the core
// while no body value reaches dest logs, metrics, or spans. The
// canaries below fail this test on any dest-side parse, log, or
// telemetry of them. Dest reads no body values on this lane — the
// forward carries the raw body untouched — so the only dest-side
// sighting would be a leak.
func TestIaCDeadNative_BodyHygiene(t *testing.T) {
	t.Parallel()

	const canaryRepoA = "repository:canary-dead-9f2c4e-a"
	const canaryRepoB = "repository:canary-dead-9f2c4e-b"

	core := &stubCore{status: http.StatusOK, body: `{"data":{"total":0},"error":null}`}
	mux := http.NewServeMux()
	mux.Handle("/", core.handler())
	upstream := httptest.NewServer(mux)
	defer upstream.Close()

	var logs bytes.Buffer
	tel := NewTelemetry(slog.New(slog.NewJSONHandler(&logs, nil)))
	srv := NewServer(upstream.URL, skeletonTestKey)
	srv.telemetry = tel
	handler := srv.Handler(DefaultV0Sunset)

	payload := `{"repo_ids":[` + `"` + canaryRepoA + `","` + canaryRepoB + `"` + `],"limit":100,"families":["terraform"]}`
	req := authedRequest(http.MethodPost, iacRoutePath(iacDeadRoute))
	req.Body = io.NopCloser(strings.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 from the stub core", rec.Code)
	}

	if got := core.hits; got != 1 {
		t.Fatalf("upstream hits = %d, want 1 (one fetch for the dead leg)", got)
	}
	for _, canary := range []string{canaryRepoA, canaryRepoB} {
		if !strings.Contains(core.lastBody, canary) {
			t.Errorf("upstream body = %q, want it to carry canary %q byte-exact", core.lastBody, canary)
		}
	}

	logged := logs.String()
	for _, leaked := range []string{canaryRepoA, canaryRepoB} {
		if strings.Contains(logged, leaked) {
			t.Errorf("dest log contains %q: body values must never reach logs\nfull logs:\n%s", leaked, logged)
		}
	}
	rendered := tel.render()
	for _, leaked := range []string{canaryRepoA, canaryRepoB} {
		if strings.Contains(rendered, leaked) {
			t.Errorf("dest metrics contain %q: body values must never reach metrics", leaked)
		}
	}
}
