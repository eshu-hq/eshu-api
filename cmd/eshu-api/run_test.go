// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net"
	"strings"
	"testing"
)

// TestRunListenFailureReturnsError pins the serve path end to end: with a
// valid core origin, run logs the startup line with the contract versions
// and the listen address, then returns the listener error when the address
// is already taken. The log must never carry the API key.
func TestRunListenFailureReturnsError(t *testing.T) {
	// Not parallel: t.Setenv pins the credential environment.
	t.Setenv(apiKeyEnvVar, "test-shared-key")

	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer taken.Close()
	addr := taken.Addr().String()

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	runErr := run([]string{"--addr", addr, "--core-base-url", "https://core:8443"}, logger)
	if runErr == nil {
		t.Fatalf("run() error = nil, want a listen error for the taken address %s", addr)
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &record); err != nil {
		t.Fatalf("decode startup log %q: %v", logs.String(), err)
	}
	if record["msg"] != "eshu-api serving" || record["addr"] != addr {
		t.Errorf("startup log = %v, want the serving line for %s", record, addr)
	}
	if record["core_read_contracts"] != "v0" || record["admission_contracts"] != "v0" {
		t.Errorf("startup log contracts = %v, want the pinned v0 versions", record)
	}
	if strings.Contains(logs.String(), "test-shared-key") {
		t.Errorf("startup log leaks the API key: %s", logs.String())
	}
}
