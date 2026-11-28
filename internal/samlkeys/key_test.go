// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package samlkeys

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeKeyPair generates a clearly-fake RSA key plus a self-signed
// certificate in-test: no static PEM lives in the repository, and
// nothing here authenticates anywhere outside the test process.
func fakeKeyPair(t *testing.T, notBefore, notAfter time.Time) (keyPEM, certPEM []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate fake key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "FAKE SP SIGNING KEY FOR TEST ONLY"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create fake certificate: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal fake key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// writeFakePair writes one generated pair under dir and returns the
// file paths. Every caller generates its own pair, so no fixture PEM
// is shared or committed.
func writeFakePair(t *testing.T, dir, name string, notBefore, notAfter time.Time) (keyPath, certPath string) {
	t.Helper()

	keyPEM, certPEM := fakeKeyPair(t, notBefore, notAfter)
	keyPath = filepath.Join(dir, name+".key.pem")
	certPath = filepath.Join(dir, name+".cert.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write fake key: %v", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write fake cert: %v", err)
	}
	return keyPath, certPath
}

func TestLoadKeyFromFiles_FailClosed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	goodKey, goodCert := writeFakePair(t, dir, "good", now.Add(-time.Hour), now.Add(time.Hour))
	otherKey, _ := writeFakePair(t, dir, "other", now.Add(-time.Hour), now.Add(time.Hour))
	expiredKey, expiredCert := writeFakePair(t, dir, "expired", now.Add(-2*time.Hour), now.Add(-time.Hour))
	badKey := filepath.Join(dir, "bad.key.pem")
	badCert := filepath.Join(dir, "bad.cert.pem")
	if err := os.WriteFile(badKey, []byte("NOT A FAKE PEM AT ALL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badCert, []byte("ALSO NOT A FAKE PEM\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		provider string
		keyPath  string
		certPath string
	}{
		{"EmptyProvider", "", goodKey, goodCert},
		{"EmptyKeyPath", "acme", "", goodCert},
		{"EmptyCertPath", "acme", goodKey, ""},
		{"MissingKeyFile", "acme", filepath.Join(dir, "no-such.key.pem"), goodCert},
		{"MissingCertFile", "acme", goodKey, filepath.Join(dir, "no-such.cert.pem")},
		{"UnparseableKey", "acme", badKey, goodCert},
		{"UnparseableCert", "acme", goodKey, badCert},
		{"MismatchedKeyCert", "acme", otherKey, goodCert},
		{"ExpiredCert", "acme", expiredKey, expiredCert},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := LoadKeyFromFiles(tc.provider, tc.keyPath, tc.certPath, ""); err == nil {
				t.Fatalf("%s must fail closed", tc.name)
			}
		})
	}

	key, err := LoadKeyFromFiles("acme", goodKey, goodCert, "")
	if err != nil {
		t.Fatalf("valid fake pair must load: %v", err)
	}
	if key.Signer() == nil || key.Certificate() == nil {
		t.Fatal("valid load must publish a signer and a certificate")
	}
	if key.Version() == "" || key.Fingerprint() == "" {
		t.Fatal("valid load without an explicit version must default to the cert fingerprint")
	}
	if key.Version() != key.Fingerprint() {
		t.Fatalf("default version = %q, want fingerprint %q", key.Version(), key.Fingerprint())
	}
}

func TestLoadKeyFromFiles_ExplicitVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "v", now.Add(-time.Hour), now.Add(time.Hour))
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "2026-11-27T00:00:00Z")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if key.Version() != "2026-11-27T00:00:00Z" {
		t.Fatalf("version = %q, want explicit version", key.Version())
	}
	if key.Fingerprint() == "" || key.Fingerprint() == key.Version() && len(key.Fingerprint()) < 64 {
		t.Fatalf("fingerprint must stay the cert digest, got %q", key.Fingerprint())
	}
}

func TestLoadKeyFromEnv_PathsNotValues(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel use.
	dir := t.TempDir()
	now := time.Now()
	keyPath, certPath := writeFakePair(t, dir, "env", now.Add(-time.Hour), now.Add(time.Hour))
	t.Setenv("ESHU_TEST_451_SP_KEY_PATH", keyPath)
	t.Setenv("ESHU_TEST_451_SP_CERT_PATH", certPath)

	key, err := LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_PATH", "ESHU_TEST_451_SP_CERT_PATH", "")
	if err != nil {
		t.Fatalf("env path load: %v", err)
	}
	if key.Provider() != "acme" {
		t.Fatalf("provider = %q, want acme", key.Provider())
	}

	t.Setenv("ESHU_TEST_451_SP_KEY_PATH", "   ")
	if _, err := LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_PATH", "ESHU_TEST_451_SP_CERT_PATH", ""); err == nil {
		t.Fatal("blank key path env must fail closed")
	}
}

func TestKey_NeverRendersMaterial(t *testing.T) {
	t.Parallel()

	// Deliberately fake: generated in-test, never authenticates.
	keyPEM, certPEM := fakeKeyPair(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "redact.key.pem")
	certPath := filepath.Join(dir, "redact.cert.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	key, err := LoadKeyFromFiles("acme", keyPath, certPath, "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	for _, rendered := range []string{
		fmt.Sprintf("%v", key),
		fmt.Sprintf("%#v", key),
		fmt.Sprintf("%v", mustSnapshot(key)),
		fmt.Sprintf("%#v", mustSnapshot(key)),
	} {
		if strings.Contains(rendered, "PRIVATE KEY") || strings.Contains(rendered, "CERTIFICATE") {
			t.Fatalf("rendered key must never carry PEM material: %q", rendered)
		}
		for _, blob := range [][]byte{keyPEM, certPEM} {
			for _, line := range strings.Split(string(blob), "\n") {
				if len(line) > 20 && strings.Contains(rendered, line) {
					t.Fatalf("rendered key leaks a PEM body line: %.20q...", line)
				}
			}
		}
	}
	for _, want := range []string{"acme", key.Version(), key.Fingerprint()} {
		if got := fmt.Sprintf("%v", key); !strings.Contains(got, want) {
			t.Fatalf("rendered key must carry %q for audit, got: %s", want, got)
		}
	}
}

func TestLoadKeyFromFiles_ErrorCarriesNoMaterial(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Garbage with a long canary line: a leaking error would echo it.
	canary := "FAKE-CANARY-" + strings.Repeat("X", 64)
	badKey := filepath.Join(dir, "canary.key.pem")
	if err := os.WriteFile(badKey, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, goodCert := writeFakePair(t, dir, "canary-good", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	_, err := LoadKeyFromFiles("acme", badKey, goodCert, "")
	if err == nil {
		t.Fatal("garbage key must fail closed")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("load error must never echo file contents: %q", err)
	}
}

func TestParseKey_NotYetValid(t *testing.T) {
	t.Parallel()

	now := time.Now()
	keyPEM, certPEM := fakeKeyPair(t, now.Add(time.Hour), now.Add(2*time.Hour))
	if _, err := parseKey("acme", keyPEM, certPEM, "v1"); err == nil {
		t.Fatal("not-yet-valid certificate must fail closed")
	} else if !strings.Contains(err.Error(), "not yet valid") {
		t.Fatalf("error = %q, want the not-yet-valid refusal", err)
	}
}

func TestLoadKeyFromEnv_PEMValueRefusedWithoutEcho(t *testing.T) {
	// No t.Parallel: t.Setenv forbids parallel use.
	const canary = "FAKE-CANARY-MATERIAL-MUST-NEVER-ECHO"
	blob := "-----BEGIN PRIVATE KEY-----\n" + canary + "\n-----END PRIVATE KEY-----\n"
	t.Setenv("ESHU_TEST_451_SP_KEY_BLOB", blob)
	t.Setenv("ESHU_TEST_451_SP_CERT_BLOB", blob)
	_, err := LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_BLOB", "ESHU_TEST_451_SP_CERT_BLOB", "")
	if err == nil {
		t.Fatal("PEM blob as path value must fail closed")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("refusal must not echo key material: %q", err)
	}
	// Single-line headerless blobs carry no newline or PEM header,
	// so they exercise the not-a-plausible-path redaction leg.
	const flatCanary = "RkFLRS1IRUFERVJMRVNTLUJMT0ItQ0FOQVJZLVNJR0hUTEVTUw"
	t.Setenv("ESHU_TEST_451_SP_KEY_BLOB", flatCanary)
	_, err = LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_BLOB", "ESHU_TEST_451_SP_CERT_BLOB", "")
	if err == nil {
		t.Fatal("headerless blob as path value must fail closed")
	}
	if strings.Contains(err.Error(), flatCanary) {
		t.Fatalf("refusal must not echo headerless material: %q", err)
	}
	// Genuine paths keep %q rendering for diagnosability.
	const missing = "/no-such-dir/sp-key.pem"
	t.Setenv("ESHU_TEST_451_SP_KEY_BLOB", missing)
	_, err = LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_BLOB", "ESHU_TEST_451_SP_CERT_BLOB", "")
	if err == nil {
		t.Fatal("missing file must fail closed")
	}
	if !strings.Contains(err.Error(), `"`+missing+`"`) {
		t.Fatalf("refusal must quote the genuine path, got: %q", err)
	}
	// Slash-containing single-line blobs (base64 routinely holds
	// "/") must redact exactly like headerless ones.
	const slashCanary = "RkFL/QQQvQ0FOQVJZLVNMQVNILUJPQg"
	t.Setenv("ESHU_TEST_451_SP_KEY_BLOB", slashCanary)
	_, err = LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_BLOB", "ESHU_TEST_451_SP_CERT_BLOB", "")
	if err == nil {
		t.Fatal("slash blob as path value must fail closed")
	}
	if strings.Contains(err.Error(), slashCanary) {
		t.Fatalf("refusal must not echo slash-containing material: %q", err)
	}
	// Leading-slash blobs are not mount paths either: quote only
	// absolute paths with depth or an extension.
	const leadCanary = "/RkFLQ0FOQVJZLUxFQURJTkctU0xBU0g"
	t.Setenv("ESHU_TEST_451_SP_KEY_BLOB", leadCanary)
	_, err = LoadKeyFromEnv("acme", "ESHU_TEST_451_SP_KEY_BLOB", "ESHU_TEST_451_SP_CERT_BLOB", "")
	if err == nil {
		t.Fatal("leading-slash blob as path value must fail closed")
	}
	if strings.Contains(err.Error(), leadCanary) {
		t.Fatalf("refusal must not echo leading-slash material: %q", err)
	}
}
