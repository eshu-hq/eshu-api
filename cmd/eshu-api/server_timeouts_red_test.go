// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"os"
	"strings"
	"testing"
)

// TDD-red proof for eshu-hq/eshu-api#130 item 3 (part of
// eshu-hq/eshu#9669): only ReadHeaderTimeout is set on the production
// http.Server (cmd/eshu-api/main.go), so a slow client can hold a
// connection indefinitely — no Read, Write, or Idle timeout bounds it.
//
// This pins the server-construction text because run() builds the
// http.Server inline with no seam to observe. The fix should introduce
// a constructor (and prefer a behavioral slow-client test); it may
// replace this pin with that test.
func TestR130_ServerSetsReadWriteIdleTimeouts(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	src := string(raw)
	for _, field := range []string{"ReadTimeout:", "WriteTimeout:", "IdleTimeout:"} {
		if !strings.Contains(src, field) {
			t.Errorf("main.go sets no %s on the production http.Server: only ReadHeaderTimeout bounds slow clients today", field)
		}
	}
}
