// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package samlkeys

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// verifyFakeRSASignature verifies a PKCS#1 v1.5 RSA signature made by
// an in-test fake key: the test proves the captured snapshot still
// signs with the old key after rotation, not any SAML profile shape.
func verifyFakeRSASignature(pub any, digest, sig []byte) error {
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("fake test cert must carry an RSA public key")
	}
	return rsa.VerifyPKCS1v15(key, crypto.SHA256, digest, sig)
}

// mustSnapshot wraps one key as a single-key snapshot for redaction
// tests without a store.
func mustSnapshot(k Key) Snapshot {
	return Snapshot{current: k}
}

func discardLog() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestStore_ReloadKeepsPreviousForOverlap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	firstKeyPath, firstCertPath := writeFakePair(t, dir, "first", now.Add(-time.Hour), now.Add(2*time.Hour))
	secondKeyPath, secondCertPath := writeFakePair(t, dir, "second", now.Add(-time.Hour), now.Add(2*time.Hour))

	store := NewStore(discardLog())
	first, err := LoadKeyFromFiles("acme", firstKeyPath, firstCertPath, "v1")
	if err != nil {
		t.Fatalf("load first: %v", err)
	}
	if err := store.Reload("test-actor", "acme", first); err != nil {
		t.Fatalf("reload first: %v", err)
	}
	got, ok := store.Get("acme")
	if !ok {
		t.Fatal("Get must find the reloaded provider")
	}
	if got.HasPrevious() {
		t.Fatal("first publish must have no previous key")
	}
	if len(got.MetadataCertificates()) != 1 {
		t.Fatalf("metadata certs = %d, want 1 before overlap", len(got.MetadataCertificates()))
	}

	second, err := LoadKeyFromFiles("acme", secondKeyPath, secondCertPath, "v2")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if err := store.Reload("test-actor", "acme", second); err != nil {
		t.Fatalf("reload second: %v", err)
	}
	got, ok = store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider after rotation")
	}
	if got.Version() != "v2" {
		t.Fatalf("current version = %q, want v2", got.Version())
	}
	if !got.HasPrevious() || got.PreviousVersion() != "v1" {
		t.Fatalf("previous version = %q, want v1 after rotation", got.PreviousVersion())
	}
	if certs := got.MetadataCertificates(); len(certs) != 2 {
		t.Fatalf("metadata certs = %d, want 2 (current+previous) during overlap", len(certs))
	}

	store.DropPrevious("test-actor", "acme")
	got, ok = store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider after overlap ends")
	}
	if got.HasPrevious() {
		t.Fatal("DropPrevious must end the overlap")
	}
	if certs := got.MetadataCertificates(); len(certs) != 1 {
		t.Fatalf("metadata certs = %d, want 1 after overlap ends", len(certs))
	}
}

func TestStore_CapturedSnapshotStillSignsAfterRotation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	firstKeyPath, firstCertPath := writeFakePair(t, dir, "old", now.Add(-time.Hour), now.Add(2*time.Hour))
	secondKeyPath, secondCertPath := writeFakePair(t, dir, "new", now.Add(-time.Hour), now.Add(2*time.Hour))

	store := NewStore(discardLog())
	first, err := LoadKeyFromFiles("acme", firstKeyPath, firstCertPath, "v1")
	if err != nil {
		t.Fatalf("load first: %v", err)
	}
	if err := store.Reload("test-actor", "acme", first); err != nil {
		t.Fatalf("reload first: %v", err)
	}

	// G7 captures one snapshot per AuthnRequest and signs outside any
	// lock: capture before the rotation, sign after it.
	captured, ok := store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider")
	}
	second, err := LoadKeyFromFiles("acme", secondKeyPath, secondCertPath, "v2")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	if err := store.Reload("test-actor", "acme", second); err != nil {
		t.Fatalf("reload second: %v", err)
	}

	digest := sha256.Sum256([]byte("fake SAML AuthnRequest bytes for test only"))
	sig, err := captured.Signer().Sign(nil, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatalf("captured snapshot must still sign after rotation: %v", err)
	}
	// The captured signature verifies under the OLD certificate the
	// snapshot was captured with, not the rotated one.
	if err := verifyFakeRSASignature(captured.Certificate().PublicKey, digest[:], sig); err != nil {
		t.Fatalf("captured signature must verify with the old cert: %v", err)
	}

	live, ok := store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider after rotation")
	}
	if live.Fingerprint() == captured.Fingerprint() {
		t.Fatal("live snapshot must carry the new key after rotation")
	}
}

func TestStore_FailedReloadKeepsOldSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "stable", now.Add(-time.Hour), now.Add(2*time.Hour))

	var sb strings.Builder
	log := slog.New(slog.NewJSONHandler(&sb, nil))
	store := NewStore(log)
	first, err := LoadKeyFromFiles("acme", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := store.Reload("test-actor", "acme", first); err != nil {
		t.Fatalf("reload: %v", err)
	}

	if err := store.ReloadFromFiles("test-actor", "acme", keyPath, certPath+"-missing", "v2"); err == nil {
		t.Fatal("reload with a missing cert file must fail")
	}
	got, ok := store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider after a failed reload")
	}
	if got.Version() != "v1" || got.HasPrevious() {
		t.Fatalf("failed reload must retain the old snapshot untouched, got version %q previous=%v", got.Version(), got.HasPrevious())
	}
	if out := sb.String(); !strings.Contains(out, "rejected") {
		t.Fatalf("failed reload must audit the rejection, got: %s", out)
	}
}

func TestStore_ReloadSwapsWithoutDroppingReaders(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "race", now.Add(-time.Hour), now.Add(2*time.Hour))
	next, err := LoadKeyFromFiles("acme", keyPath, certPath, "v-next")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	first, err := LoadKeyFromFiles("acme", keyPath, certPath, "v-first")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	store := NewStore(discardLog())
	if err := store.Reload("test-actor", "acme", first); err != nil {
		t.Fatalf("reload: %v", err)
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				snap, ok := store.Get("acme")
				if !ok {
					t.Error("concurrent Get must always find the provider")
					return
				}
				_ = snap.Signer()
				_ = snap.MetadataCertificates()
			}
		}()
	}
	if err := store.Reload("rotation-actor", "acme", next); err != nil {
		t.Fatalf("concurrent reload: %v", err)
	}
	wg.Wait()
	if got, _ := store.Get("acme"); got.Version() != "v-next" {
		t.Fatalf("rotated version = %q, want v-next", got.Version())
	}
}

func TestStore_AuditNeverCarriesMaterial(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "audit", now.Add(-time.Hour), now.Add(2*time.Hour))
	rawKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	var sb strings.Builder
	store := NewStore(slog.New(slog.NewJSONHandler(&sb, nil)))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := store.Reload("audit-actor", "acme", key); err != nil {
		t.Fatalf("reload: %v", err)
	}
	store.DropPrevious("audit-actor", "acme")
	out := sb.String()
	if strings.Contains(out, "PRIVATE KEY") || strings.Contains(out, "CERTIFICATE") {
		t.Fatalf("audit record must never carry key material: %s", out)
	}
	for _, line := range strings.Split(string(rawKey), "\n") {
		if len(line) > 20 && strings.Contains(out, line) {
			t.Fatalf("audit record leaks a PEM body line: %.20q...", line)
		}
	}
	for _, want := range []string{"audit-actor", "acme", key.Version(), key.Fingerprint()} {
		if !strings.Contains(out, want) {
			t.Fatalf("audit record must carry %q, got: %s", want, out)
		}
	}
	for _, rendered := range []string{
		fmt.Sprintf("%v", key),
		fmt.Sprintf("%#v", mustSnapshot(key)),
	} {
		_ = rendered
	}
}

func TestStore_UnknownProvider(t *testing.T) {
	t.Parallel()

	store := NewStore(discardLog())
	if _, ok := store.Get("no-such-provider"); ok {
		t.Fatal("Get on an unknown provider must report absent")
	}
	if store.DropPrevious("actor", "no-such-provider") {
		t.Fatal("DropPrevious on an unknown provider must report absent")
	}
}

func TestStore_GetMissPaths(t *testing.T) {
	t.Parallel()

	store := NewStore(discardLog())
	if _, ok := store.Get(""); ok {
		t.Fatal("Get on an empty provider id must report absent without touching the table")
	}
	// A Store that never went through NewStore has a nil table:
	// reads must report absent, never panic.
	bare := &Store{}
	if _, ok := bare.Get("acme"); ok {
		t.Fatal("Get on a nil table must report absent")
	}
	if bare.DropPrevious("actor", "acme") {
		t.Fatal("DropPrevious on a nil table must report absent")
	}
}

func TestStore_NewStoreNilLogger(t *testing.T) {
	t.Parallel()

	store := NewStore(nil)
	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "k", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := store.Reload("actor", "acme", key); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if _, ok := store.Get("acme"); !ok {
		t.Fatal("nil-logger store must serve reloaded keys")
	}
}

func TestStore_ReloadRejects(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "k", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	store := NewStore(discardLog())
	if err := store.Reload("actor", "acme", key); err != nil {
		t.Fatalf("baseline reload: %v", err)
	}
	for name, tc := range map[string]struct {
		actor    string
		provider string
		next     Key
	}{
		"empty provider": {"actor", "", key},
		"mismatched key": {"actor", "other", key},
		"invalid key":    {"actor", "acme", Key{}},
	} {
		if err := store.Reload(tc.actor, tc.provider, tc.next); err == nil {
			t.Errorf("%s: reload must fail closed", name)
		}
	}
	// Rejections leave the live snapshot untouched.
	got, ok := store.Get("acme")
	if !ok || got.Version() != "v1" {
		t.Fatalf("rejected reloads must keep the old snapshot live, got %+v %v", got.Version(), ok)
	}
}

func TestStore_ReloadFromFilesRejects(t *testing.T) {
	t.Parallel()

	store := NewStore(discardLog())
	if err := store.ReloadFromFiles("actor", "acme", "/no-such/key.pem", "/no-such/cert.pem", "v9"); err == nil {
		t.Fatal("reload from missing files must fail")
	}
	if _, ok := store.Get("acme"); ok {
		t.Fatal("failed file reload must leave no snapshot behind")
	}
}

func TestStore_DropPreviousWithoutOverlap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "k", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "v1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	store := NewStore(discardLog())
	if err := store.Reload("actor", "acme", key); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if store.DropPrevious("actor", "acme") {
		t.Fatal("DropPrevious with no overlap open must report false")
	}
}

func TestSnapshot_PreviousAccessors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	firstPath, firstCert := writeFakePair(t, dir, "first", now.Add(-time.Hour), now.Add(2*time.Hour))
	secondPath, secondCert := writeFakePair(t, dir, "second", now.Add(-time.Hour), now.Add(2*time.Hour))
	first, err := LoadKeyFromFiles("acme", firstPath, firstCert, "v1")
	if err != nil {
		t.Fatalf("load first: %v", err)
	}
	second, err := LoadKeyFromFiles("acme", secondPath, secondCert, "v2")
	if err != nil {
		t.Fatalf("load second: %v", err)
	}
	store := NewStore(discardLog())
	if err := store.Reload("actor", "acme", first); err != nil {
		t.Fatalf("reload first: %v", err)
	}
	if err := store.Reload("actor", "acme", second); err != nil {
		t.Fatalf("reload second: %v", err)
	}
	got, ok := store.Get("acme")
	if !ok {
		t.Fatal("Get must find the provider during overlap")
	}
	prev, ok := got.PreviousSigner()
	if !ok || prev == nil {
		t.Fatal("PreviousSigner must report the overlap key")
	}
	if got.PreviousVersion() != "v1" {
		t.Fatalf("PreviousVersion = %q, want v1", got.PreviousVersion())
	}
	if got.PreviousFingerprint() != first.Fingerprint() {
		t.Fatal("PreviousFingerprint must identify the overlap key")
	}
	if s := got.String(); !strings.Contains(s, "previous:") {
		t.Fatalf("overlap String must name the previous key, got %q", s)
	}
	if fmt.Sprintf("%#v", got) != got.String() {
		t.Fatal("GoString must redact exactly like String")
	}
	store.DropPrevious("actor", "acme")
	got, _ = store.Get("acme")
	if _, ok := got.PreviousSigner(); ok {
		t.Fatal("PreviousSigner must report absent after overlap ends")
	}
	if got.PreviousVersion() != "" || got.PreviousFingerprint() != "" {
		t.Fatal("previous accessors must empty out after overlap ends")
	}
	if s := got.String(); strings.Contains(s, "previous:") {
		t.Fatalf("single-key String must not name a previous key, got %q", s)
	}
}
