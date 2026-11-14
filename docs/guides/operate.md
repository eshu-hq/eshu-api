# Operate eshu-api

Two ways to run this: the container image, or the binary on its own.
Releases publish `ghcr.io/eshu-hq/eshu-api`. The `Dockerfile` compiles
with Go 1.26.11 and drops the binary onto a distroless base with no
shell, exposes port `8080`, and starts `/eshu-api`.

## Step 1: run it

Image:

```bash
docker build -t eshu-api:dev .
docker run --rm -p 8080:8080 -e ESHU_API_KEY='change-me' \
  eshu-api:dev --core-base-url http://host.docker.internal:8443
```

Binary: build `./cmd/eshu-api`, set `ESHU_API_KEY`, pass
`--core-base-url` — the exact steps are in
[Getting started](../getting-started/README.md). If you want the full
stack (core plus every service), don't assemble it by hand. Use
[eshu-deploy](https://github.com/eshu-hq/eshu-deploy). This repo alone
doesn't deploy the core.

## Step 2: check it's healthy

All three probes are public, so you can curl them without a token:

- `GET /health` returns `200` with the service name, one version key
  per contract family, and the core origin you configured. Good for
  "is the process up, and what does it think it's talking to?"
- `GET /readyz` returns `200` with
  `{"service":"eshu-api","probe":"readyz","status":"ok"}` when the
  relay can reach its core origin, and `503` with `status
  "unavailable"` plus a closed `details.reason` when it cannot (no
  origin configured, or the core unreachable). This is the gate to
  point the Kubernetes readiness probe at — not `/health`, which
  stays `200` either way. Each verdict also files
  `eshu_api_readiness_total{result,reason}` on `/metrics`.
- `GET /metrics` returns Prometheus text with per-family request and
  dispatch counts. [Telemetry](../telemetry.md) explains each series
  and how to split "core is slow" from "we are slow".

## Step 3: read the logs

Logs are JSON lines on stdout. Each dispatch logs one `api request`
line with the method, family, status, and a correlation ID. At
startup, look for the `eshu-api serving` line with the contract
versions and listen address. If boot fails, the process exits with
code `1` and logs `eshu-api exited` with the error.

One rollout note: there's no graceful-shutdown handler wired in the
binary today, so an orchestrator's SIGTERM simply ends the process.
Drain at the load balancer, not here.

## Releases

Pushes to `main` and `v*` tags publish the image for
`linux/amd64,linux/arm64` with SLSA provenance and an SPDX SBOM, and
Trivy blocks the release on any `HIGH` or `CRITICAL` CVE. A tag also
cuts a GitHub Release carrying the SBOM. [Release](../release.md) has
the tag rules and the digest checks to run before you deploy anything.
Nothing in this repo deploys for you.

Something not starting, or answering `401`/`503`? Head to
[Troubleshoot](troubleshoot.md).
