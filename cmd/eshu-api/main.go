// SPDX-License-Identifier: MIT
// Copyright (c) 2025-2026 eshu-hq

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/eshu-hq/eshu-api/api"
)

// apiKeyEnvVar is the only credential input: the shared-key bearer value
// the auth gate compares against. It travels by environment so the key
// never appears in process flags; an unset variable fails closed and
// every /api/* request 401s until the operator sets it.
const apiKeyEnvVar = "ESHU_API_KEY"

// server holds the dependencies of the dispatch surface. It answers
// health and readiness itself, gates /api/* behind the shared key, and
// dispatches authenticated calls to the core origin. It carries no
// credential at rest and takes no core handles.

type server struct {
	coreBaseURL string
	apiKey      string
}

func newServer(coreBaseURL, apiKey string) *server {
	return &server{coreBaseURL: coreBaseURL, apiKey: apiKey}
}

// handler returns the production middleware chain over the dispatch
// routes, with the sunset date read from the environment.
func (s *server) handler() http.Handler {
	return api.NewServer(s.coreBaseURL, s.apiKey).Handler(api.V0SunsetFromEnv(os.Getenv))
}

func run(args []string, logOut *slog.Logger) error {
	flags := flag.NewFlagSet("eshu-api", flag.ContinueOnError)
	addr := flags.String("addr", ":8080", "listen address for the dispatch surface")
	coreBaseURL := flags.String("core-base-url", "", "core read-API origin, for example https://core:8443")
	if err := flags.Parse(args); err != nil {
		return err
	}
	// The key travels by environment only, never by flag, so it stays out
	// of the process table. Empty fails closed at the auth gate.
	key := os.Getenv(apiKeyEnvVar)
	if *coreBaseURL == "" {
		return fmt.Errorf("eshu-api: --core-base-url is required")
	}
	srv := newServer(*coreBaseURL, key)
	logOut.Info("eshu-api serving",
		"core_read_contracts", api.CoreReadContractsVersion,
		"admission_contracts", api.SuppressionAdmissionContractVersion,
		"addr", *addr,
	)
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv.handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return httpServer.ListenAndServe()
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("eshu-api exited", "error", err)
		os.Exit(1)
	}
}
