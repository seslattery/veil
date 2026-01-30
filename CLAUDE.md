# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Veil is a security sandbox for AI agents. It provides filesystem isolation via macOS seatbelt and network policy enforcement via an allowlist proxy. The agent's network traffic is forced through a localhost proxy that enforces host-based allowlists.

**Primary language:** Go 1.25  
**CLI framework:** Cobra  
**Build system:** Justfile

## Common Commands

```bash
# Build
just build              # Build veil CLI → bin/veil

# Test
just test               # All tests with race detection

# Lint & Format
just lint               # golangci-lint
just fmt                # go fmt + gofmt

# Development workflow
just dev                # fmt + tidy + lint + test + build

# Run a specific test
go test -race -v -run TestName ./...

# Manual testing
bin/veil -- curl https://github.com      # Run command in sandbox
bin/veil --dry-run -- echo test          # Print seatbelt profile without executing
bin/veil init                            # Create default config at ~/.veilwarden/config.yaml
```

## Architecture

### Request Flow
```
User runs: veil -- npm install

1. CLI loads config from ~/.veilwarden/config.yaml
2. Generates seatbelt profile with allowed_write_paths
3. Starts Martian proxy on ephemeral localhost port
4. Executes command under sandbox-exec with:
   - Filesystem: writes blocked except allowed paths
   - Filesystem: reads blocked for ~/.<dotfiles>
   - Network: all traffic forced through localhost proxy
5. Proxy evaluates each request against allowlist policy
   - Allowed: request proceeds
   - Denied: connection rejected
6. Command exits, proxy shuts down
```

### Key Packages

- **cmd/veil/** - CLI entry point. Root command orchestrates config → policy → proxy → sandbox.
- **internal/config/** - YAML config loading from `~/.veilwarden/config.yaml`
- **internal/sandbox/** - macOS seatbelt integration with PTY support for interactive commands
- **internal/proxy/** - Martian-based forward proxy that enforces allowlist policy
- **internal/policy/** - Glob-based host allowlist engine
- **internal/logging/** - Structured JSON logging via slog to `~/.veilwarden/veil.log`

### Security Model

**Asymmetric filesystem access:**
- **Writes** denied everywhere except explicitly configured `allowed_write_paths`
- **Reads** allowed everywhere except `~/.*` dotfiles (blocked by default)
- **Dangerous files** always blocked even in allowed paths: `.env`, `.git/config`, `.aws/credentials`, etc.

**Network isolation:**
- Only localhost connections to proxy allowed
- All HTTP/HTTPS traffic forced through proxy via `HTTP_PROXY`/`HTTPS_PROXY` env vars
- Proxy enforces host-based allowlist (glob patterns like `*.npmjs.org`)

### Configuration

Config file: `~/.veilwarden/config.yaml`

```yaml
sandbox:
  allowed_write_paths:
    - ./              # Relative to config file directory
    - /tmp
policy:
  allowlist:
    - "*.npmjs.org"
    - "github.com"
    - "api.anthropic.com"
```

Create default config with `veil init`.

## Implementation Notes

- Seatbelt profile template is in `internal/sandbox/profile.sbpl.tmpl`
- Symlink resolution is critical on macOS (`/tmp` → `/private/tmp`)
- PTY handling enables interactive commands (terminal resize, raw mode)
- No MITM/credential injection in v1 — proxy operates as forward proxy using HTTP CONNECT
