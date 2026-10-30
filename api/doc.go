// Package api owns the eshu-api HTTP surface: routing, the shared-key
// auth gate, the v1 alias rewrite, the v0 deprecation headers, and the
// byte-for-byte dispatch of the authenticated /api/* subtree to the
// configured core origin.
//
// Every behavior here is ported from the versioned core contracts pinned
// under contracts/ (CoreReadContractsVersion, currently v0) and the
// in-tree surface it replaces (go/cmd/api in eshu). This package takes
// no core implementation handles and no shared-table shortcuts: core
// truth arrives only over HTTP to the configured origin — relayed
// through dispatch for /api/*, called through internal/readclient for
// versioned read-contract use — and durable mutations persist only
// through versioned core admission contracts.
package api
