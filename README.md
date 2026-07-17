# ctlkit

Shared Go substrate for the dotdevlabs `*ctl` agent CLIs.

Provides reusable packages for contexts, HTTP client, output rendering, AI reference, and goreleaser configuration across the dotdevlabs CLI toolchain.

## Packages

- `pkg/ctxutil` — context helpers and propagation
- `pkg/httpclient` — shared HTTP client with retry/timeout defaults
- `pkg/output` — machine-stable JSON and human-readable output rendering
- `pkg/airef` — AI model reference helpers

## Usage

```go
import "github.com/dotdevlabs/ctlkit/pkg/output"
```

## Development

```bash
bin/ci   # format · vet · lint · test · build
```

### Prerequisites

- Go 1.24+
- `golangci-lint` (`brew install golangci-lint` or see https://golangci-lint.run/usage/install/)

## License

MIT
