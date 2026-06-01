# pngr CLI

Command-line interface and [MCP](https://modelcontextprotocol.io) server for
**[pngr.dev](https://pngr.dev)** — a multi-tenant URL monitoring SaaS. Manage
monitors, notification channels, and alerting rules from your terminal, your CI
pipeline, or an AI agent.

The CLI is a thin client over the pngr.dev REST API — it stores and sends a
personal access token and talks to `https://api.pngr.dev`. It contains no
server code.

## Install

```bash
# Linux / macOS
curl -fsSL https://pngr.dev/install.sh | sh

# Go
go install pngr.dev/cli@latest
```

The install script downloads the right binary for your OS/arch from the
[GitHub Releases](https://github.com/marfx000/pngr-cli/releases), verifies its
checksum, and installs it onto your `PATH`. Override with `PNGR_VERSION` or
`PNGR_INSTALL_DIR` (see the script header).

> `go install` builds from source and produces a binary named `cli`; the
> release archives and install script name it `pngr`.

## Getting started

```bash
pngr auth login        # paste a personal access token from /settings/api-tokens
pngr monitor list
pngr monitor create --name "API" --url https://api.example.com --interval 60
```

Authentication is **token-only** — there is no password login on the CLI. Mint a
personal access token in the web UI under **Settings → API tokens** and either
run `pngr auth login` or set `PNGR_TOKEN` (preferred for CI). Config lives at
`~/.config/pngr/config.yaml` (XDG-compliant, `0600`).

Every command supports `--output table|json|yaml` (default `table`), set per
invocation, via `PNGR_OUTPUT`, or with `pngr config set output=json`.

## MCP server

`pngr mcp` runs as a stdio MCP server so AI agents (Claude Desktop, Cursor,
Windsurf, …) can drive pngr.dev through typed tools. Each CLI command maps 1:1
to an MCP tool (`noun_verb`, e.g. `monitor_list`, `channel_test`).

```json
{
  "mcpServers": {
    "pngr": {
      "command": "pngr",
      "args": ["mcp"],
      "env": { "PNGR_TOKEN": "pngr_pat_…" }
    }
  }
}
```

Run `pngr mcp --help` for transport options (stdio default, SSE for remote
agents).

## Layout

```
*.go              — cobra commands (root main package)
internal/
  cliclient/      — constructs the API client from config/env
  cliconfig/      — XDG config file (token, active org, output)
  mcp/            — MCP server mode + tool definitions
  output/         — table / json / yaml rendering
pkg/client/       — typed Go client for the pngr.dev REST API (importable)
```

`pkg/client` is the published Go client for the pngr.dev API and can be
imported on its own:

```go
import "pngr.dev/cli/pkg/client"
```

## Development

```bash
make build                 # ./bin/pngr
make test                  # unit tests
make test-smoke            # pkg/client against a live server (needs PNGR_BASE_URL)
make snapshot              # GoReleaser local dry-run → dist/
```

Releases are cut by pushing a tag:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

which triggers `.github/workflows/release.yml` → GoReleaser builds the
cross-platform archives + checksums and publishes the GitHub Release.

## License

[MIT](LICENSE) © Oleg Malaphey
