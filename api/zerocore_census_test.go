// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package api

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// RouteClaim is one census unit: a route pattern served under a
// credential class by a handler closure rooted at a named FuncDecl or
// method. The census unit is the triple, not the route alone: wave
// gates relay foreign credentials explicitly while shared-key lanes
// terminate viaNative, so the same pattern can reach different sinks
// per credential class (ADR-0002 section 1; Refs eshu-hq/eshu-api#449).
type RouteClaim struct {
	// Pattern is the route pattern, e.g. "GET /api/v0/repositories".
	Pattern string
	// Handler is the FuncDecl or Type.Method name the handler
	// closure roots at, e.g. "Server.health".
	Handler string
	// Credential is the credential class proving this lane:
	// shared-key, foreign, none, or public.
	Credential string
}

// AssertNoCorePath fails t when any claim's handler closure can reach
// a core sink. The walk is AST-based (go/parser only, stdlib): comment
// and string scanning is forbidden because forwardToCore is mentioned
// pervasively in comments. An unresolvable handler name fails, never
// skips: a census that passed what it could not examine could certify
// a route it never walked.
//
// The default allowlist is EMPTY: any future carve-out must be a named
// RouteClaim exception with an issue reference, never an inferred one.
func AssertNoCorePath(t *testing.T, claims []RouteClaim) {
	t.Helper()
	for _, claim := range claims {
		chain, err := findZeroCorePath(claim, false)
		if err != nil {
			t.Fatalf("route %s (%s): census error: %v", claim.Pattern, claim.Credential, err)
		}
		if len(chain) != 0 {
			t.Fatalf("route %s (%s): reachable core path: %s",
				claim.Pattern, claim.Credential, strings.Join(chain, " -> "))
		}
	}
}

// zeroCoreSinkName reports whether a called function name is a core
// sink: forwardToCore, the Server.dispatch relay entry, any func
// matching the fetch*Core port-fetch shape, or a low-level outbound
// call the AST records under its selector name (see the call walk).
func zeroCoreSinkName(name string) bool {
	switch name {
	case "forwardToCore", "dispatch", "httpGet", "httpPost", "netDial", "readclient":
		return true
	}
	return strings.HasPrefix(name, "fetch") && strings.HasSuffix(name, "Core")
}

// zeroCoreCalleeFacts scans one function body for same-package callee
// edges and core-sink hits. Selector calls on the http, net, and
// readclient packages are normalized to pseudo-names (httpGet,
// httpPost, netDial, readclient) so the sink table stays flat.
func zeroCoreCalleeFacts(decl *ast.FuncDecl) (callees []string, sinks []string) {
	seenCall := map[string]bool{}
	seenSink := map[string]bool{}
	addCall := func(name string) {
		if name == "" || seenCall[name] {
			return
		}
		seenCall[name] = true
		callees = append(callees, name)
	}
	addSink := func(name string) {
		if seenSink[name] {
			return
		}
		seenSink[name] = true
		sinks = append(sinks, name)
	}
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		// Method and function values (e.g. http.HandlerFunc(s.dispatch)
		// in routes) reach a sink without a call site in this body, so
		// every selector naming a sink counts, call or not.
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if zeroCoreSinkName(sel.Sel.Name) {
				addSink(sel.Sel.Name)
			}
			return true
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if zeroCoreSinkName(fun.Name) {
				addSink(fun.Name)
			} else {
				addCall(fun.Name)
			}
		case *ast.SelectorExpr:
			sel := fun.Sel.Name
			if zeroCoreSinkName(sel) {
				addSink(sel)
				return true
			}
			switch qualifier, ok := fun.X.(*ast.Ident); {
			case ok && qualifier.Name == "http" && (sel == "Get" || sel == "Post" || sel == "PostForm" || sel == "Head"):
				addSink("httpGet")
			case ok && qualifier.Name == "http" && sel == "DefaultClient":
				addSink("httpGet")
			case ok && qualifier.Name == "net" && strings.HasPrefix(sel, "Dial"):
				addSink("netDial")
			case ok && qualifier.Name == "readclient":
				addSink("readclient")
			default:
				// A *.httpClient.Do selector call is the single
				// transport every api core fetch flows through
				// (see dispatch.go); any other selector call may
				// still be a same-package method hop.
				if sel == "Do" {
					if inner, ok := fun.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "httpClient" {
						addSink("httpClient.Do")
						return true
					}
				}
				addCall(sel)
			}
		case *ast.FuncLit:
			// Closures inline in the body are walked by Inspect
			// itself; no edge needed.
		}
		return true
	})
	return callees, sinks
}

// zeroCoreFuncIndex maps every FuncDecl in dir to its declaration,
// keyed by bare name (functions and methods alike) and additionally by
// Type.Method for methods. Test files are included only when
// includeTests is set: the production census must never certify (or
// condemn) a handler through test helpers, while the fixture proof
// documents its one test-file claim explicitly.
func zeroCoreFuncIndex(dir string, includeTests bool) (map[string][]*ast.FuncDecl, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read package dir: %w", err)
	}
	index := map[string][]*ast.FuncDecl{}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if !includeTests && strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			index[fn.Name.Name] = append(index[fn.Name.Name], fn)
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recv := fn.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}
				if ident, ok := recv.(*ast.Ident); ok {
					key := ident.Name + "." + fn.Name.Name
					index[key] = append(index[key], fn)
				}
			}
		}
	}
	return index, nil
}

// findZeroCorePath walks the call graph from the claimed handler and
// returns the first handler-to-sink chain found, or nil when the
// closure is core-free. An unresolvable or ambiguous handler name is
// an error: fail closed, never skip. The walk explores every
// declaration sharing a bare callee name, so same-named methods on
// different receivers cannot hide a sink behind each other.
func findZeroCorePath(claim RouteClaim, includeTests bool) ([]string, error) {
	_, caller, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("locate package dir: runtime.Caller failed")
	}
	index, err := zeroCoreFuncIndex(filepath.Dir(caller), includeTests)
	if err != nil {
		return nil, err
	}
	roots, ok := index[claim.Handler]
	if !ok || len(roots) == 0 {
		return nil, fmt.Errorf("handler %q unresolvable: no FuncDecl or method found", claim.Handler)
	}
	if !strings.Contains(claim.Handler, ".") && len(roots) > 1 {
		return nil, fmt.Errorf("handler %q ambiguous: %d declarations share the name, qualify as Type.Method", claim.Handler, len(roots))
	}
	// Depth-first walk carrying the chain. visited is per-path (not
	// global) so diamond call shapes still report through every leg.
	var visit func(fn *ast.FuncDecl, chain []string, onPath map[*ast.FuncDecl]bool) []string
	visit = func(fn *ast.FuncDecl, chain []string, onPath map[*ast.FuncDecl]bool) []string {
		if onPath[fn] {
			return nil
		}
		onPath[fn] = true
		defer delete(onPath, fn)
		callees, sinks := zeroCoreCalleeFacts(fn)
		if len(sinks) > 0 {
			return append(chain, sinks[0])
		}
		for _, callee := range callees {
			for _, next := range index[callee] {
				if next.Body == nil {
					continue
				}
				if hit := visit(next, append(chain, callee), onPath); hit != nil {
					return hit
				}
			}
		}
		return nil
	}
	for _, root := range roots {
		if hit := visit(root, []string{claim.Handler}, map[*ast.FuncDecl]bool{}); hit != nil {
			return hit, nil
		}
	}
	return nil, nil
}

// TestZeroCore_CensusRelayPathFailsWithChain is the census positive
// control: Server.dispatch reaches forwardToCore today, so the
// call-graph walk must report the chain naming the route, the
// credential class, and every hop. Without this RED today, every
// census pass below would be vacuous.
func TestZeroCore_CensusRelayPathFailsWithChain(t *testing.T) {
	t.Parallel()

	chain, err := findZeroCorePath(RouteClaim{
		Pattern:    "GET /api/v0/repositories",
		Handler:    "Server.dispatch",
		Credential: "shared-key",
	}, false)
	if err != nil {
		t.Fatalf("findZeroCorePath error = %v, want a core-path chain", err)
	}
	joined := strings.Join(chain, " -> ")
	if !strings.Contains(joined, "forwardToCore") {
		t.Fatalf("chain = %q, want it to name forwardToCore", joined)
	}
	if !strings.Contains(joined, "Server.dispatch") {
		t.Fatalf("chain = %q, want it to start at the claimed handler", joined)
	}
}

// TestZeroCore_CensusFetchCorePathFails pins the fetch*Core sink class:
// Server.fetchGitHubCore performs the single port fetch over
// s.httpClient.Do, so the walk must fail it even though its name never
// mentions forwardToCore.
func TestZeroCore_CensusFetchCorePathFails(t *testing.T) {
	t.Parallel()

	chain, err := findZeroCorePath(RouteClaim{
		Pattern:    "POST /api/v0/auth/github/terminal-failure",
		Handler:    "Server.fetchGitHubCore",
		Credential: "foreign",
	}, false)
	if err != nil {
		t.Fatalf("findZeroCorePath error = %v, want a core-path chain", err)
	}
	joined := strings.Join(chain, " -> ")
	if !strings.Contains(joined, "fetchGitHubCore") {
		t.Fatalf("chain = %q, want it to name the fetch*Core hop", joined)
	}
}

// TestZeroCore_CensusLocalHandlerPasses pins the pass leg:
// Server.health writes compiled constants and never touches a core
// sink, so the walk must report no path.
func TestZeroCore_CensusLocalHandlerPasses(t *testing.T) {
	t.Parallel()

	chain, err := findZeroCorePath(RouteClaim{
		Pattern:    "GET /health",
		Handler:    "Server.health",
		Credential: "none",
	}, false)
	if err != nil {
		t.Fatalf("findZeroCorePath error = %v, want nil", err)
	}
	if len(chain) != 0 {
		t.Fatalf("chain = %q, want empty for a pure-local handler", strings.Join(chain, " -> "))
	}
}

// TestZeroCore_CensusUnresolvableHandlerFailsClosed pins the
// fail-closed contract: an unknown handler name is an error, never a
// silent pass. A census that skipped what it could not resolve could
// certify a route it never examined.
func TestZeroCore_CensusUnresolvableHandlerFailsClosed(t *testing.T) {
	t.Parallel()

	chain, err := findZeroCorePath(RouteClaim{
		Pattern:    "GET /api/v0/does-not-exist",
		Handler:    "Server.noSuchHandlerZeroCore",
		Credential: "shared-key",
	}, false)
	if err == nil {
		t.Fatalf("findZeroCorePath chain = %q, want an unresolvable-handler error", strings.Join(chain, " -> "))
	}
}

// TestZeroCore_AssertNoCorePathPassThrough pins the exported wrapper's
// pass path on a real pure-local handler.
func TestZeroCore_AssertNoCorePathPassThrough(t *testing.T) {
	t.Parallel()

	AssertNoCorePath(t, []RouteClaim{
		{Pattern: "GET /health", Handler: "Server.health", Credential: "none"},
	})
}
