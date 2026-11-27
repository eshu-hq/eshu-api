// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zeroCoreSpellingIndex builds a synthetic package exercising one
// evasive transport spelling per handler, then indexes it. Parsing is
// syntax-only (go/parser, no type checking), so the stub client and
// server types never need to exist outside this source text.
func zeroCoreSpellingIndex(t *testing.T) map[string][]*ast.FuncDecl {
	t.Helper()
	const src = `package api

import "net/http"

type zeroCoreSpellingServer struct{ httpClient *http.Client }

func (s zeroCoreSpellingServer) zeroCoreAliasDo() {
	client := s.httpClient
	_, _ = client.Do(nil)
}

func (s zeroCoreSpellingServer) zeroCoreDirectGet() {
	_, _ = s.httpClient.Get("http://core.invalid")
}

func zeroCoreForeignDo() {
	_, _ = http.DefaultClient.Do(nil)
}

func zeroCoreBenignHeaderGet(h string) string {
	return http.Header{"X-Test": []string{h}}.Get("X-Test")
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "spellings.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write synthetic package: %v", err)
	}
	index, err := zeroCoreFuncIndex(dir, false)
	if err != nil {
		t.Fatalf("index synthetic package: %v", err)
	}
	return index
}

// TestZeroCore_CensusSpellingAliasedClientDo is the RED regression
// test for the aliased-client evasion: client.Do via a local assigned
// from s.httpClient must report a chain, never an empty pass.
func TestZeroCore_CensusSpellingAliasedClientDo(t *testing.T) {
	t.Parallel()

	index := zeroCoreSpellingIndex(t)
	chain := walkZeroCoreIndex(index, "zeroCoreSpellingServer.zeroCoreAliasDo", index["zeroCoreSpellingServer.zeroCoreAliasDo"])
	if len(chain) == 0 {
		t.Fatal("aliased client.Do must report a core chain, got empty (census evasion)")
	}
}

// TestZeroCore_CensusSpellingDirectClientGet is the RED regression
// test for the convenience-wrapper evasion: s.httpClient.Get must
// report a chain, never an empty pass.
func TestZeroCore_CensusSpellingDirectClientGet(t *testing.T) {
	t.Parallel()

	index := zeroCoreSpellingIndex(t)
	chain := walkZeroCoreIndex(index, "zeroCoreSpellingServer.zeroCoreDirectGet", index["zeroCoreSpellingServer.zeroCoreDirectGet"])
	if len(chain) == 0 {
		t.Fatal("s.httpClient.Get must report a core chain, got empty (census evasion)")
	}
}

// TestZeroCore_CensusSpellingForeignClientDo is the RED regression
// test for the foreign-client evasion: http.DefaultClient.Do must
// report a chain, never an empty pass. It also bypasses the counter
// leg (no Server transport to wrap), so the census is its only gate.
func TestZeroCore_CensusSpellingForeignClientDo(t *testing.T) {
	t.Parallel()

	index := zeroCoreSpellingIndex(t)
	chain := walkZeroCoreIndex(index, "zeroCoreForeignDo", index["zeroCoreForeignDo"])
	if len(chain) == 0 {
		t.Fatal("http.DefaultClient.Do must report a core chain, got empty (census evasion)")
	}
}

// TestZeroCore_CensusSpellingBenignHeaderGet pins the precision leg:
// a header lookup must stay silent so the sink normalization does
// not false-positive everyday handler code into a core path.
func TestZeroCore_CensusSpellingBenignHeaderGet(t *testing.T) {
	t.Parallel()

	index := zeroCoreSpellingIndex(t)
	chain := walkZeroCoreIndex(index, "zeroCoreBenignHeaderGet", index["zeroCoreBenignHeaderGet"])
	if len(chain) != 0 {
		t.Fatalf("header Get must stay silent, got chain %q", strings.Join(chain, " -> "))
	}
}
