// Package api owns the eshu-api HTTP surface: routing, the shared-key
// auth gate, the v1 alias rewrite, and the v0 deprecation headers.
//
// Every behavior here is ported from the versioned core contracts pinned
// under contracts/ (CoreReadContractsVersion, currently v0) and the
// in-tree middleware it replaces (go/cmd/api in eshu). This package takes
// no core implementation handles and no shared-table shortcuts: core truth
// arrives only over HTTP through internal/readclient, and durable
// mutations persist only through versioned core admission contracts.
package api
