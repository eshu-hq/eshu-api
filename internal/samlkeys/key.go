// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package samlkeys

import (
	"crypto"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// Key is one validated SP signing key for a single SAML provider:
// the parsed signer G7 adapts to its SAML library, the leaf
// certificate published in metadata, and the audit metadata
// (provider, version, fingerprint, expiry) that is safe to log.
//
// A Key is immutable after load: the signer and certificate are
// never mutated, so snapshots captured before a rotation stay valid
// for in-flight signing. The zero value is invalid — loaders fail
// closed instead of returning it — and there is deliberately no
// MarshalJSON: only the metadata accessors below may leave this
// struct, never the signing material.
type Key struct {
	provider    string
	version     string
	fingerprint string
	notAfter    time.Time
	signer      crypto.Signer
	cert        *x509.Certificate
}

// LoadKeyFromFiles reads one SP signing key from secret-mounted
// files: keyPath names the file holding the PEM private key and
// certPath names the file holding the PEM certificate. The version
// is an optional audit label (a secret version or rotation
// timestamp); when empty it defaults to the certificate fingerprint
// so every key is attributable without extra configuration.
//
// The load fails closed — empty provider or paths, unreadable files,
// unparseable material, a key that does not match the certificate,
// or a certificate outside its validity window — and the returned
// error carries paths only, never key or certificate bytes. Parsed
// input buffers are cleared before return.
func LoadKeyFromFiles(provider, keyPath, certPath, version string) (Key, error) {
	// Canonicalize once: Get and Reload look up the trimmed id, so
	// a padded id must not create a second unreachable entry.
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return Key{}, fmt.Errorf("samlkeys: refusing to load a key with an empty provider id")
	}
	if strings.TrimSpace(keyPath) == "" {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q with an empty key path", provider)
	}
	if strings.TrimSpace(certPath) == "" {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q with an empty certificate path", provider)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: cannot read key file %s", provider, safePath(keyPath))
	}
	defer clearBytes(keyPEM)
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: cannot read certificate file %s", provider, safePath(certPath))
	}
	defer clearBytes(certPEM)
	return parseKey(provider, keyPEM, certPEM, version)
}

// safePath renders a configured file path for errors. A multiline
// PEM blob passed where a path belongs (for example through an
// environment variable) must not echo back out through the refusal:
// values that look like material render as a fixed refusal instead.
func safePath(path string) string {
	if strings.Contains(path, "\n") || strings.Contains(path, "PRIVATE KEY") || strings.Contains(path, "CERTIFICATE") {
		return "[redacted: value looks like key material, not a file path]"
	}
	return fmt.Sprintf("%q", path)
}

// LoadKeyFromEnv reads one SP signing key through secret references:
// keyPathEnv and certPathEnv name environment variables carrying the
// FILE PATHS of the mounted key and certificate (a Kubernetes secret
// volume mount, never the PEM value itself — a multiline PEM as an
// environment value is rejected by design), and versionEnv
// optionally names a variable carrying the secret version for audit.
// Empty or unset path variables fail closed like
// LoadKeyFromFiles; an empty version falls back to the certificate
// fingerprint.
func LoadKeyFromEnv(provider, keyPathEnv, certPathEnv, versionEnv string) (Key, error) {
	keyPath := strings.TrimSpace(os.Getenv(keyPathEnv))
	certPath := strings.TrimSpace(os.Getenv(certPathEnv))
	version := strings.TrimSpace(os.Getenv(versionEnv))
	if versionEnv == "" {
		version = ""
	}
	return LoadKeyFromFiles(provider, keyPath, certPath, version)
}

// parseKey validates PEM material with the standard library only and
// publishes the immutable Key. No dependency beyond the standard
// library is allowed here: go.mod carries none and this custody
// layer must not add one.
func parseKey(provider string, keyPEM, certPEM []byte, version string) (Key, error) {
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: key and certificate do not parse as a pair", provider)
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok || signer == nil {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: private key cannot sign", provider)
	}
	// X509KeyPair populates Leaf on success (Go >= 1.23); a nil
	// leaf here would mean the toolchain changed under us, so fail
	// closed rather than signing with an unparsed certificate.
	leaf := pair.Leaf
	if leaf == nil {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: certificate did not parse", provider)
	}
	now := time.Now()
	if now.After(leaf.NotAfter) {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: certificate expired", provider)
	}
	if now.Before(leaf.NotBefore) {
		return Key{}, fmt.Errorf("samlkeys: refusing to load provider %q: certificate not yet valid", provider)
	}
	sum := sha256.Sum256(leaf.Raw)
	fingerprint := hex.EncodeToString(sum[:])
	if strings.TrimSpace(version) == "" {
		version = fingerprint
	}
	return Key{
		provider:    provider,
		version:     version,
		fingerprint: fingerprint,
		notAfter:    leaf.NotAfter,
		signer:      signer,
		cert:        leaf,
	}, nil
}

// Provider reports the SAML provider id this key signs for. Routes
// are per-{provider_id} (see api/d9saml.go), so custody stays
// per-provider too.
func (k Key) Provider() string { return k.provider }

// Version reports the audit version this key was loaded with: the
// explicit secret version, else the certificate fingerprint.
func (k Key) Version() string { return k.version }

// Fingerprint reports the hex SHA-256 digest of the certificate DER:
// the default version and the metadata KeyDescriptor identifier.
func (k Key) Fingerprint() string { return k.fingerprint }

// NotAfter reports the certificate expiry: the end of this key's
// signing eligibility.
func (k Key) NotAfter() time.Time { return k.notAfter }

// Signer reveals the signing key for the G7 SAML library adapter.
// Callers must never log, label, or persist the signer.
func (k Key) Signer() crypto.Signer { return k.signer }

// Certificate reveals the leaf certificate published in SP metadata
// and used to verify captured signatures.
func (k Key) Certificate() *x509.Certificate { return k.cert }

// String redacts the material: only provider, version,
// fingerprint, and expiry render, so a stray %v in logs cannot leak
// the key.
func (k Key) String() string {
	return fmt.Sprintf("Key{provider:%s version:%s fingerprint:%s notAfter:%s}",
		k.provider, k.version, k.fingerprint, k.notAfter.UTC().Format(time.RFC3339))
}

// GoString redacts the material like String: without it, %#v would
// dump the signer into any debug-format log line.
func (k Key) GoString() string { return k.String() }

// clearBytes zeroes a parsed input buffer best-effort: the parsed
// signer and certificate stay live, but the raw PEM no longer sits
// in the read buffer after load.
func clearBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
