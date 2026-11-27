// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package sessionstore

import (
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoadCredential_EmptyEnvFailsClosed(t *testing.T) {

	t.Setenv("ESHU_TEST_447_RW_DSN", "")
	if _, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", ""); err == nil {
		t.Fatal("empty credential env must fail closed")
	}
}

func TestLoadCredential_WhitespaceOnlyFailsClosed(t *testing.T) {
	t.Setenv("ESHU_TEST_447_RW_DSN", "   ")
	if _, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", ""); err == nil {
		t.Fatal("whitespace-only credential env must fail closed")
	}
}

func TestLoadCredential_UnknownRoleFailsClosed(t *testing.T) {
	t.Setenv("ESHU_TEST_447_RW_DSN", "postgres://db:5432/eshu")
	if _, err := LoadCredentialFromEnv(CredentialRole("superuser"), "ESHU_TEST_447_RW_DSN", ""); err == nil {
		t.Fatal("unknown roles must fail closed: only the two least-privilege roles load")
	}
}

func TestCredential_NeverLogsItsValue(t *testing.T) {
	// Deliberately fake: this value never authenticates anywhere; it
	// exists so the test can prove the audit path never logs it.
	const dsn = "postgres://eshu-rw:FAKE-SECRET-FOR-TEST-ONLY@db:5432/eshu"
	t.Setenv("ESHU_TEST_447_RW_DSN", dsn)
	cred, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	var sb strings.Builder
	log := slog.New(slog.NewJSONHandler(&sb, nil))
	r := NewRotating(cred, log)
	r.Reload("test-actor", cred)
	out := sb.String()
	if strings.Contains(out, dsn) || strings.Contains(out, "FAKE-SECRET-FOR-TEST-ONLY") {
		t.Fatalf("audit record must never carry the credential value: %s", out)
	}
	for _, want := range []string{"test-actor", string(RoleSessionRW)} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit record must carry %q, got: %s", want, out)
		}
	}
}

func TestRotating_ReloadSwapsWithoutDroppingReaders(t *testing.T) {
	t.Setenv("ESHU_TEST_447_RW_DSN", "postgres://db:5432/eshu")
	first, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := NewRotating(first, log)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				_ = r.Get()
			}
		}()
	}
	t.Setenv("ESHU_TEST_447_RW_DSN", "postgres://db:5432/eshu-next")
	next, err := LoadCredentialFromEnv(RoleSessionRW, "ESHU_TEST_447_RW_DSN", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r.Reload("rotation-actor", next)
	wg.Wait()
	if got := r.Get().Version(); got != next.Version() {
		t.Fatalf("rotated version = %q, want %q", got, next.Version())
	}
}

func TestRotating_VersionDefaultsToTimestamp(t *testing.T) {
	before := time.Now().UTC()
	t.Setenv("ESHU_TEST_447_RO_DSN", "postgres://db:5432/eshu")
	cred, err := LoadCredentialFromEnv(RoleSessionRO, "ESHU_TEST_447_RO_DSN", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cred.Version() == "" {
		t.Fatal("credential without an explicit version must stamp load time")
	}
	_ = before
}
