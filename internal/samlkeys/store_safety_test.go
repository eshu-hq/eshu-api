// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package samlkeys

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStore_ReloadOnZeroStoreFailsClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "k", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bare := &Store{}
	if err := bare.Reload("actor", "acme", key); err == nil {
		t.Fatal("Reload on a zero-value Store must fail closed, never panic")
	}
}

func TestStore_ReloadRejectsStaleValidity(t *testing.T) {
	t.Parallel()

	now := time.Now()
	keyPEM, certPEM := fakeKeyPair(t, now.Add(-2*time.Hour), now.Add(-time.Hour))
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("parse fake pair: %v", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("parse fake leaf: %v", err)
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		t.Fatal("fake key must sign")
	}
	aged := Key{provider: "acme", version: "v0", fingerprint: "fp", notAfter: leaf.NotAfter, signer: signer, cert: leaf}
	store := NewStore(discardLog())
	if err := store.Reload("actor", "acme", aged); err == nil {
		t.Fatal("Reload must reject a key that expired since load")
	} else if !strings.Contains(err.Error(), "validity window") {
		t.Fatalf("error = %q, want the validity-window refusal", err)
	}
	if _, ok := store.Get("acme"); ok {
		t.Fatal("rejected stale key must leave no snapshot behind")
	}
}

func TestStore_EvictedOverlapIsAudited(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	var sb strings.Builder
	store := NewStore(slog.New(slog.NewJSONHandler(&sb, nil)))
	for i, name := range []string{"one", "two", "three"} {
		keyPath, certPath := writeFakePair(t, dir, name, now.Add(-time.Hour), now.Add(2*time.Hour))
		key, err := LoadKeyFromFiles("acme", keyPath, certPath, "v"+string(rune('1'+i)))
		if err != nil {
			t.Fatalf("load %s: %v", name, err)
		}
		if err := store.Reload("actor", "acme", key); err != nil {
			t.Fatalf("reload %s: %v", name, err)
		}
	}
	out := sb.String()
	if !strings.Contains(out, "saml-sp-key-overlap-evict") {
		t.Fatalf("second rotation must audit the evicted overlap key, got: %s", out)
	}
	if !strings.Contains(out, "v1") {
		t.Fatalf("eviction audit must name the evicted version, got: %s", out)
	}
	got, _ := store.Get("acme")
	if got.Version() != "v3" || got.PreviousVersion() != "v2" {
		t.Fatalf("after two rotations current=v3 previous=v2, got %q/%q", got.Version(), got.PreviousVersion())
	}
}

func TestStore_PaddedProviderIdHasNoGhost(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "k", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles(" acme ", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if key.Provider() != "acme" {
		t.Fatalf("provider = %q, want the trimmed canonical id", key.Provider())
	}
	store := NewStore(discardLog())
	if err := store.Reload("actor", " acme ", key); err != nil {
		t.Fatalf("padded reload: %v", err)
	}
	got, ok := store.Get("acme")
	if !ok || got.Version() != "v1" {
		t.Fatal("padded reload must address the trimmed entry Get finds")
	}
	secondPath, secondCert := writeFakePair(t, dir, "k2", now.Add(-time.Hour), now.Add(time.Hour))
	second, err := LoadKeyFromFiles("acme", secondPath, secondCert, "v2")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if err := store.Reload("actor", "acme", second); err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, _ = store.Get("acme")
	if got.PreviousVersion() != "v1" {
		t.Fatal("padded then plain reloads must rotate one entry, not fork ghosts")
	}
}
