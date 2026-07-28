# ctlkit

Shared Go substrate for the dotdevlabs `*ctl` agent CLIs (`loopctl`, `clusterctl`, future `*ctl`).

Import ctlkit to get: a Cobra root command with global flags, named contexts backed by Viper config, an HTTP client with automatic browser User-Agent and JSON envelope decoding, three output modes, structured `CLIError` types with stable exit codes, an `ai` command reference generator, and a `version` command with self-update check.

## Packages

| Package | Purpose |
|---------|---------|
| `pkg/clierror` | Structured `CLIError{code, message, hint}` type; per-class exit codes; directive stderr rendering |
| `pkg/config` | Named contexts in `~/.config/atmt/<product>.yaml`; Viper-backed load/save; env var overrides; `auth` and `context` Cobra commands |
| `pkg/httpclient` | Bearer auth; automatic browser `User-Agent`; exponential-backoff retry; flat `{data, pagination}` JSON envelope decoding; JSON:API (`application/vnd.api+json`) single/collection resource decoding with typed attributes |
| `pkg/output` | Three modes: human-readable table (default), `--json` envelope, `--format '{{…}}'` Go-template projection |
| `pkg/ctxutil` | Type-safe `context.Context` helpers for propagating config, HTTP client, renderer, and global flags through Cobra's `RunE` chain |
| `pkg/root` | Single entry point — `root.New(BuildConfig)` returns a fully-wired `*cobra.Command` with all global flags and built-in subcommands |
| `pkg/version` | Build info (set via ldflags); `version` Cobra command; `CheckUpdate` helper |
| `pkg/airef` | Walks the Cobra command tree; emits the full command/flag/output-shape reference as structured markdown and `--json` for agent ingestion |

## Quick Start

```go
package main

import (
    "os"

    "github.com/dotdevlabs/ctlkit/pkg/clierror"
    "github.com/dotdevlabs/ctlkit/pkg/root"
    "github.com/dotdevlabs/ctlkit/pkg/version"
)

func main() {
    cmd := root.New(root.BuildConfig{
        Product:  "loopctl",
        Short:    "Manage Loop resources",
        Version:  version.Current("loopctl"),
        Commands: nil, // register your resource-verb commands here
    })
    if err := cmd.Execute(); err != nil {
        os.Exit(clierror.HandleErr(err, os.Stderr))
    }
}
```

Downstream CLIs get for free:

- `--context`, `--json`, `--format`, `--dry-run`, `--verbose` global flags
- `auth login --name <ctx> --url <url> --token <token>` to register a named context
- `context list` / `context select <name>` to manage active context
- `version` command with optional `--json` output
- `ai` command that dumps the full CLI reference as markdown or `--json`

## Named Contexts

Config file: `~/.config/atmt/<product>.yaml`

```yaml
current_context: prod
contexts:
  prod:
    base_url: https://api.example.com
    token: tok_live_xxx
  sandbox:
    base_url: https://sandbox.example.com
    token: tok_sandbox_xxx
```

Environment overrides (e.g., for `loopctl`):

| Variable | Effect |
|----------|--------|
| `LOOPCTL_CONTEXT` | Override active context name |
| `LOOPCTL_TOKEN` | Override bearer token for active context |

Multiple processes can target different environments simultaneously without mutating shared config.

## HTTP Client

```go
import "github.com/dotdevlabs/ctlkit/pkg/httpclient"

client := httpclient.New(baseURL, token)

// Flat-envelope helper — decodes {data: T, pagination: ...}
env, err := httpclient.GetEnvelope[[]MyResource](ctx, client, "/v1/resources")
```

Every request automatically sets a real browser `User-Agent` (Chrome/Mac) to pass through Cloudflare bot management. Retries with exponential backoff on 429/5xx responses (up to 3 attempts).

### JSON:API Support

For backends that serve `application/vnd.api+json`, ctlkit provides a typed JSON:API decoder. Resource `id` (string) and `type` are promoted alongside the flattened `attributes` struct — no extra nesting in call sites.

```go
// Define your attributes struct — only the fields under "attributes" in the JSON:API document.
type ProjectAttrs struct {
    Name     string `json:"name"`
    Platform string `json:"platform"`
    Repo     string `json:"repo"`
}

// Single resource: GET /api/projects/42
res, err := httpclient.GetJSONAPISingle[ProjectAttrs](ctx, client, "/api/projects/42")
// res.ID, res.Type, res.Attributes.Name, res.Attributes.Platform, ...

// Collection: GET /api/projects
col, err := httpclient.GetJSONAPICollection[ProjectAttrs](ctx, client, "/api/projects")
// col.Data []Resource[ProjectAttrs], col.Links.Next, col.Meta["total"]

// Create: POST /api/projects
res, err := httpclient.PostJSONAPISingle[ProjectAttrs](ctx, client, "/api/projects", body)
```

Requests automatically carry `Accept: application/vnd.api+json`; POST bodies use `Content-Type: application/vnd.api+json`. Non-2xx responses parse the JSON:API `errors[]` array and surface the first `detail` (or `title`) in the returned `CLIError`.

The flat-envelope helpers (`GetEnvelope`, `PostEnvelope`) are unchanged and remain the right choice for backends on the older `{data: {...}}` envelope.

## Output Modes

```go
import "github.com/dotdevlabs/ctlkit/pkg/output"

r := output.New(jsonFlag, formatFlag, os.Stdout, os.Stderr)

// Table (default)
r.Render(
    []output.Column{{Header: "NAME"}, {Header: "STATUS"}},
    [][]string{{"task-001", "done"}, {"task-002", "pending"}},
    envelope,
)
```

| Flag | Mode | stdout |
|------|------|--------|
| _(none)_ | Table | `text/tabwriter` formatted rows |
| `--json` | JSON | `{"data":…,"pagination":…}` envelope |
| `--format '{{.Data}}'` | Template | Go `text/template` projection of envelope |

Diagnostics always go to stderr via `r.Diag(...)`.

## Error Handling

```go
import "github.com/dotdevlabs/ctlkit/pkg/clierror"

return clierror.New(clierror.CodeNotFound, "task 999 not found",
    "List tasks with 'loopctl tasks list --project 9'.")
```

Exit codes:

| Code | Value | Meaning |
|------|-------|---------|
| `CodeOK` | 0 | Success |
| `CodeNotFound` | 1 | Resource not found |
| `CodeUnauthorized` | 2 | Auth required |
| `CodeForbidden` | 3 | Insufficient permissions |
| `CodeBadRequest` | 4 | Invalid request |
| `CodeConflict` | 5 | Resource conflict |
| `CodeServerError` | 6 | Server / unexpected error |
| `CodeNotReady` | 7 | Retry candidate |
| `CodeUsage` | 8 | Bad CLI usage |

In `main()`:

```go
if err := cmd.Execute(); err != nil {
    os.Exit(clierror.HandleErr(err, os.Stderr))
}
```

## AI Reference Generator

```bash
loopctl ai          # markdown reference for all commands, flags, and workflows
loopctl ai --json   # machine-stable JSON for agent ingestion
```

Pass `Workflows []airef.Workflow` in `BuildConfig` to include hand-authored "common workflows" in the output.

## Version Command

```bash
loopctl version
loopctl version --json
```

Set build info via ldflags:

```
-ldflags "-X github.com/dotdevlabs/ctlkit/pkg/version.Version={{.Version}} \
          -X github.com/dotdevlabs/ctlkit/pkg/version.Commit={{.Commit}} \
          -X github.com/dotdevlabs/ctlkit/pkg/version.Date={{.Date}}"
```

## Development

```bash
bin/ci   # format · vet · lint · test (race + coverage gate ≥70%) · build
```

### Prerequisites

- Go 1.24+
- `golangci-lint` (`brew install golangci-lint` or see https://golangci-lint.run/usage/install/)

### CI

GitHub Actions runs `bin/ci` on every push and PR (`.github/workflows/ci.yml`). Releases on `v*` tags generate changelog notes via goreleaser (`.github/workflows/release.yml`). ctlkit is a library — no binary artifacts are released.

## License

MIT
