# Release (Refs `eshu-hq/eshu#9520`, part of `eshu-hq/eshu#6707`)

Pushes to `main` and tags `v*` publish. Either event builds the
`Dockerfile` for `linux/amd64,linux/arm64`, pushes
`ghcr.io/eshu-hq/eshu-api`, and attaches SLSA provenance plus an SPDX
SBOM to the image. Trivy blocks the release on any `HIGH` or `CRITICAL`
CVE in the pushed digest. The `build-binary` job cross-compiles the
`eshu-api` server for `linux/amd64` and `linux/arm64` (matching
`IMAGE_PLATFORMS`, `CGO_ENABLED=0`, `-trimpath`). A tag push
additionally cuts a GitHub Release carrying the SBOM and both compiled
binaries; a push to `main` publishes the moving `main`,
`latest`, and `sha-<full-sha>` tags with the same hardening and no
Release. Nothing here deploys: QA wiring lives in
`iac-eks-eshu` and stays non-authoritative until core dispatch lands.

## Cut a release

```bash
git tag v0.1.0
git push origin v0.1.0
```

The tag must be `vMAJOR.MINOR.PATCH[-prerelease]` and must point at the
current head of `main`. The first steps of the run refuse any other `v*` tag
before anything is built, logged in, pushed, or released; cut the tag from
an up-to-date `main` and push it right away.

Watch the `release` run on the tag: `build-and-push` and
`build-binary` in parallel, then `trivy`, then `github-release`. The
release body records the pushed digest; verify it before deploying
anywhere:

```bash
gh release view v0.1.0 --repo eshu-hq/eshu-api
gh attestation verify \
  oci://ghcr.io/eshu-hq/eshu-api:<tag> --repo eshu-hq/eshu-api
```

Prove the attached binary boots and serves health (use the asset
matching your machine; the example needs no core behind it for
`/health`, while the missing-flag check needs nothing at all):

```bash
gh release download v0.1.0 --repo eshu-hq/eshu-api \
  --pattern 'eshu-api-linux-*' --dir /tmp/eshu-api-rel
chmod +x /tmp/eshu-api-rel/eshu-api-linux-amd64
/tmp/eshu-api-rel/eshu-api-linux-amd64; test $? -ne 0
/tmp/eshu-api-rel/eshu-api-linux-amd64 --core-base-url http://127.0.0.1:9 &
srv=$!; sleep 1
curl -fsS http://127.0.0.1:8080/health
kill $srv
```

## What the workflow proves

- `build-and-push`: reproducible metadata (pinned labels, source epoch),
  push on push events only — main branch or version tag (a manual
  dispatch on any ref, tag or branch, builds without pushing), SLSA
  provenance and SBOM attestations pushed to GHCR and verified back
  with `gh attestation verify`.
- `trivy`: image scan fails on `HIGH`/`CRITICAL`; the SARIF uploads as
  an artifact for triage when it fails.
- `build-binary`: cross-compiles `linux/amd64` and `linux/arm64`
  with `CGO_ENABLED=0 -trimpath` and uploads both binaries as the
  `eshu-api-binaries` run artifact.
- `github-release`: runs only after the scan passes, on version tags
  only; the tag carries a Release with the SBOM, both binaries, and
  the digest in the notes, so a deploy pin never comes from a
  hand-typed tag.

## Failed-scan residue

The scan runs after the push, so a `HIGH`/`CRITICAL` failure leaves the
pushed digest in GHCR with no GitHub Release and a red run. That digest
must not be deployed: either re-tag after the fix, or delete the
version from the package. Scanning before the push would need a local
multi-arch image round-trip; that restructure stays a follow-up until
the first tag cut shows whether the residue matters in practice.

## Prove the workflow without pushing

Run it by hand from the branch: Actions, `release`, Run workflow. The
dispatch builds the image for `linux/amd64` only, still cross-compiles
both release binaries, and skips login, push, attestations, SBOM, scan,
and the Release, so it proves the Dockerfile, the binary build, and
workflow syntax without publishing anything.

## Release history

Tags `v0.1.0` through `v0.5.0` exist and prove the image path
(provenance verification and the Trivy gate have both been observed
green), but every one of them is SBOM-only: no tag carries a compiled
binary asset, so no release yet satisfies the cutover bar
(`eshu-hq/eshu#9669` step 1: binary asset plus image digest plus
SBOM/provenance attestations). The first tag cut after the
`build-binary` lane lands is the first binary-carrying release; verify
its binary boot and `/health` checks above before deploying it
anywhere. Core dispatch (`eshu-hq/eshu#8872`) still gates any
QA deploy and any cutover.
