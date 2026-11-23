# D9 clone-latency evidence

D9 latency record for the OIDC-login and GitHub-login tier-C clone
lanes (Refs `eshu-hq/eshu#9669`, part of `eshu-hq/eshu#6707`). The
census assigns the satellite this gate: core keeps the
contract-registry and static-contract gates; this repo proves the
tier-C clones are not slower than the relay they replace.

## Method

Hermetic stub-core comparison, same binary, same caller shape:

- Before legs: the explicit relay (`BenchmarkD8RelayWrite` for
  writes, `BenchmarkD8RelayRead` for reads) through the full
  production chain. The write-relay leg is foreign-bearer POST
  `/api/v0/impact/change-surface` on its wave-mounted route
  (change-surface has since terminated natively in the impact lane's
  Wave-3, so the shared-key shape would run
  terminated-vs-terminated);
  the read-relay leg is foreign-bearer GET `/api/v0/dependencies`
  on its wave-mounted route (dependencies has since terminated
  natively in W8b-narrow for the shared key).
- After legs: the D9 clones — the keyless login-start GET bare and
  with the full provider-IDs-plus-return_to query
  (`BenchmarkD9PublicLogin`), and the keyless callback GET carrying
  state + code (`BenchmarkD9PublicCallback`) — through the full
  production chain with the explicit route match and telemetry.
- Both legs hit the same in-process stub core, so the numbers isolate
  dest-side cost, not core or network time.
- `bash scripts/measure-d9-latency.sh` runs each leg `-count=5`,
  takes the median ns/op, and fails when a clone leg exceeds relay by
  more than 50%. The threshold is generous on purpose: both legs do
  exactly one server-side fetch, so any real pathology (a double
  fetch, a retry, a body re-read) exceeds it loudly while scheduling
  noise does not.

## Relay-route disclosure

The write-relay leg is POST `/api/v0/impact/change-surface` via
foreign-bearer explicit relay (the route the W6 scan lane re-pointed
when it terminated the last code POST; change-surface has since
terminated natively in the impact lane's Wave-3); the read-relay leg
is GET `/api/v0/dependencies` via foreign-bearer explicit relay
(dependencies has since terminated natively in W8b-narrow). Both legs
do exactly one server-side fetch, so the 1.50 bound stays a real
pathology check.

## Numbers (local, 2026-11-07; relay leg re-pointed 2026-11-15)

Shared host, in-process stub core (`go test` benchmarks, 2000x5):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Login-start GET, bare | 66240 | 83797 | 0.79 | ≤ 1.50 |
| Login-start GET, full query | 65434 | 83797 | 0.78 | ≤ 1.50 |
| Callback GET, state + code | 71210 | 83797 | 0.85 | ≤ 1.50 |
| Login-start GET vs relay POST | 65434 | 84704 | 0.77 | ≤ 1.50 |
| Callback GET vs relay POST | 71210 | 84704 | 0.84 | ≤ 1.50 |

Re-measured 2026-11-15 after the W6 scan lane re-pointed the relay
leg from the terminated code bundles POST to the impact
change-surface POST (then the relay leg; change-surface has since
terminated natively in the impact lane's Wave-3, and the leg now
rides foreign-bearer explicit relay).

Within budget on all five legs: the lane adds the explicit route
match around the same single fetch, with no gate and no authorizer on
the public carve-out. Absolute deployment numbers against a pinned
core image ride flip prep (D11); this gate runs hermetically on every
PR and in CI.

## GitHub lane numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5 via
`bash scripts/measure-d9github-latency.sh` against
`BenchmarkD9GitHubPublicLogin` / `BenchmarkD9GitHubPublicCallback`):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Login-start GET, bare | 79306 | 87099 | 0.91 | ≤ 1.50 |
| Login-start GET, full query | 79339 | 87099 | 0.91 | ≤ 1.50 |
| Callback GET, state + code | 86436 | 87099 | 0.99 | ≤ 1.50 |
| Login-start GET vs relay POST | 79339 | 88167 | 0.90 | ≤ 1.50 |
| Callback GET vs relay POST | 86436 | 88167 | 0.98 | ≤ 1.50 |

Within budget on all five legs, same hermetic method as the OIDC
lane: the GitHub clones add the explicit route match around the same
single fetch, with no gate and no authorizer on the public
carve-out.

## SAML lane numbers (local, 2026-11-07)

Shared host, in-process stub core (`go test` benchmarks, 2000x5 via
`bash scripts/measure-d9saml-latency.sh` against
`BenchmarkD9SAMLPublicMetadata` / `BenchmarkD9SAMLPublicLogin` /
`BenchmarkD9SAMLPublicACS`):

| Leg | Clone (median ns/op) | Relay (median ns/op) | Ratio | Budget |
| --- | --- | --- | --- | --- |
| Metadata GET | 81474 | 83780 | 0.97 | ≤ 1.50 |
| Login-start GET, bare | 76695 | 83780 | 0.92 | ≤ 1.50 |
| Login-start GET, full query | 77837 | 83780 | 0.93 | ≤ 1.50 |
| ACS POST, large form body (~288KiB) | 456652 | 457065 | 1.00 | ≤ 1.50 |
| Login-start GET vs relay POST | 77837 | 84669 | 0.92 | ≤ 1.50 |

Within budget on all five legs, same hermetic method as the OIDC and
GitHub lanes: the SAML clones add the explicit route match around the
same single fetch, with no gate and no authorizer on the public
carve-out. The ACS leg compares against same-bytes relay leg
`BenchmarkD9SAMLRelayLargePost` (a keyed change-surface POST carrying the
identical ~288KiB form through the generic relay): the 1.00 ratio
proves the streaming forward adds no per-byte dest cost beyond the
relay itself. The D9 wave is complete with this lane.
