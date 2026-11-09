# Reference: eshu-api

Exact values, no narrative. If you want the why, start in
[Concepts](../concepts/README.md); come here for the what.

| Page | Covers |
| --- | --- |
| [Configuration](configuration.md) | Every flag and env var, the API key, and which routes need a token |
| [Telemetry](../telemetry.md) | Every `/metrics` series and every field in the `api request` log line |
| [Core read contracts](../../contracts/core-read-contracts.md) | The versioned read contracts dispatch relies on |
| [What this build serves](../skeleton.md) | Scope and limits of the current build |

Route classification source of truth: the registration comments in
`api/router.go`, summarised in [Configuration](configuration.md).
