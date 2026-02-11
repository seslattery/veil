# Veil Design Document

A simplified security sandbox for AI agents. Veil provides filesystem isolation via macOS seatbelt and network policy enforcement via an allowlist proxy.

## Goals

1. **Simplicity** — Minimal, auditable codebase (~1,000 LOC)
2. **Correctness** — Sound security model, well-tested
3. **Conciseness** — No feature bloat; do one thing well

## Non-Goals (v1)

- Credential injection / MITM
- Multiple security tiers
- OPA policy engine
- Secret backends (Doppler, etc.)
- Linux support (Bubblewrap)
- Configurable Mach services / sysctls

## Architecture

```
veil CLI
    ├── Config (~/.veilwarden/config.yaml)
    ├── Seatbelt sandbox (macOS sandbox-exec)
    │   └── Fixed "standard" security profile
    └── Martian proxy (network policy enforcement)
        └── Allowlist policy (host glob patterns)
```

### Request Flow

```
User runs: veil -- npm install

1. CLI loads config from ~/.veilwarden/config.yaml
2. Collects dynamic env paths (TMPDIR, XDG_CACHE_HOME, XDG_CONFIG_HOME)
3. Generates strict seatbelt profile with:
   - Read-deny-by-default baseline
   - System path reads allowed (/usr, /bin, /System, etc.)
   - allowed_read_paths (read-only) and allowed_write_paths (read+write)
   - Env-derived paths allowed as read+write
   - CWD allowed as read+write
4. Starts Martian proxy on ephemeral localhost port
5. Executes command under sandbox-exec with:
   - Filesystem: reads denied except system paths and configured allows
   - Filesystem: writes denied except configured allowed_write_paths
   - Filesystem: ~/.<dotfiles> and /var/run always denied (last-match-wins)
   - Network: all traffic forced through localhost proxy
6. Proxy evaluates each request against allowlist policy
   - Allowed: request proceeds
   - Denied: connection rejected
7. Command exits, proxy shuts down
```

## Components

### 1. CLI (`cmd/veil/`)

Cobra-based CLI with implicit exec behavior.

```bash
veil -- npm install          # Run command in sandbox
veil --config ./alt.yaml -- make  # Explicit config
```

**Flags:**
- `--config` — Path to config file (default: walk-up discovery)
- `--log-level` — Log verbosity (default: info)
- `--dry-run` — Print seatbelt profile without executing

**Estimated LOC:** ~100

### 2. Config (`internal/config/`)

YAML configuration loaded from `~/.veilwarden/config.yaml`.

```yaml
# ~/.veilwarden/config.yaml

sandbox:
  allowed_read_paths:     # Read-only exceptions (optional)
    - ~/.claude
  allowed_write_paths:    # Read+write exceptions
    - ./
    - /tmp

policy:
  allowlist:
    - "*.npmjs.org"
    - "github.com"
    - "api.anthropic.com"
```

`TMPDIR`, `XDG_CACHE_HOME`, and `XDG_CONFIG_HOME` are automatically allowed (read+write) when explicitly set in the environment.

**Config struct:**

```go
type Config struct {
    Sandbox SandboxConfig `yaml:"sandbox"`
    Policy  PolicyConfig  `yaml:"policy"`
}

type SandboxConfig struct {
    AllowedReadPaths  []string `yaml:"allowed_read_paths"`
    AllowedWritePaths []string `yaml:"allowed_write_paths"`
}

type PolicyConfig struct {
    Allowlist []string `yaml:"allowlist"` // Host glob patterns
}
```

**Estimated LOC:** ~150

### 3. Sandbox (`internal/sandbox/`)

macOS seatbelt integration with a fixed "standard" security profile.

**Security model (strict, deny-by-default):**
- **Reads denied** everywhere except:
  - System paths (`/usr`, `/bin`, `/sbin`, `/opt`, `/etc`, `/System`, `/Applications`, `/Library`, `/dev`, `/tmp`, `/nix`, `/private/var/db`, `/private/var/folders`)
  - `allowed_read_paths` from config (read-only)
  - `allowed_write_paths` from config (read+write)
  - Canonical CWD (read+write)
  - `TMPDIR`, `XDG_CACHE_HOME`, `XDG_CONFIG_HOME` when explicitly set (read+write)
- **Writes denied** everywhere except:
  - `allowed_write_paths` from config
  - `/private/var/folders` (macOS temp)
  - `/dev/null`, `/dev/tty`, etc.
- **Always denied** (last-match-wins):
  - `~/.*` dotfiles (reads and writes, with configured exceptions)
  - `/var/run` and `/private/var/run` (socket escape paths)
- **Dangerous files blocked** even in allowed paths:
  - `.env`, `.env.*`
  - `.git/hooks/*`, `.git/config`
  - `.npmrc`, `.pypirc`
  - `.aws/credentials`, `.docker/config.json`
- **Network** restricted to localhost proxy only
- **Mach services** — curated list of 23 services (fonts, security, DNS, etc.)
- **Sysctls** — curated allowlist (53 names + 9 prefixes)

**Interface:**

```go
type Sandbox struct {
    config    Config
    proxyPort int
}

func New(cfg Config, proxyPort int) *Sandbox

// Start executes the command in the sandbox.
// If stdin is a terminal, allocates a PTY for interactive use.
func (s *Sandbox) Start(ctx context.Context, cmd string, args []string) error
```

**Files:**
- `sandbox.go` — Execution logic, PTY handling (~120 LOC)
- `profile.go` — Profile generation from config (~80 LOC)
- `profile.sbpl.tmpl` — Seatbelt profile template (~150 LOC)

**Estimated LOC:** ~250 (Go) + ~150 (template)

### 4. Proxy (`internal/proxy/`)

Martian-based HTTP/HTTPS proxy for policy enforcement. No MITM or credential injection — just allowlist evaluation.

**Behavior:**
- Listens on ephemeral localhost port
- Evaluates each request against allowlist policy
- Allowed requests: forwarded to destination
- Denied requests: returns 403 Forbidden

**Interface:**

```go
type Proxy struct {
    policy   Policy
    listener net.Listener
    server   *http.Server
}

func New(policy Policy) (*Proxy, error)

func (p *Proxy) Port() int
func (p *Proxy) Start(ctx context.Context) error
func (p *Proxy) Shutdown(ctx context.Context) error
```

**Note:** Since we're not doing MITM, we don't need ephemeral CA generation. The proxy operates as a forward proxy using HTTP CONNECT for HTTPS traffic. Policy is evaluated on the CONNECT request (host level) rather than inspecting encrypted traffic.

**Environment setup:** Veil automatically sets `HTTP_PROXY` and `HTTPS_PROXY` environment variables for the child process, pointing to the proxy's localhost address.

**Estimated LOC:** ~200

### 5. Policy (`internal/policy/`)

Simple allowlist engine with glob pattern matching.

**Evaluation logic:**
1. For each CONNECT request, extract the target host
2. Check if any allowlist pattern matches (case-insensitive glob)
3. If any pattern matches → Allow
4. No patterns match → Deny

**Interface:**

```go
type Policy struct {
    patterns []glob.Glob // Compiled host patterns
}

func NewPolicy(hosts []string) (*Policy, error)

// Evaluate returns true if the host is allowed.
func (p *Policy) Evaluate(host string) bool
```

**Estimated LOC:** ~100

## Directory Structure

```
veil/
├── cmd/
│   └── veil/
│       └── main.go           # CLI entry point
├── internal/
│   ├── config/
│   │   └── config.go         # Config loading + validation
│   ├── sandbox/
│   │   ├── sandbox.go        # Execution logic
│   │   ├── profile.go        # Profile generation
│   │   └── profile.sbpl.tmpl # Seatbelt template
│   ├── proxy/
│   │   └── proxy.go          # Martian wrapper
│   └── policy/
│       └── policy.go         # Allowlist engine
├── go.mod
├── go.sum
└── DESIGN.md
```

## Implementation Plan

### PR 1: Project Scaffolding (~150 LOC)

**Goal:** Establish project structure, dependencies, and CLI skeleton.

**Contents:**
- `go.mod` with dependencies (cobra, martian, yaml, glob)
- `cmd/veil/main.go` — CLI skeleton with `--help`, `--version`
- `internal/config/config.go` — Config types (no loading yet)
- Basic `justfile` targets (build, test, lint)

**Review focus:** Dependency choices, project structure, config schema design.

### PR 2: Seatbelt Sandbox (~400 LOC)

**Goal:** Working sandbox that can execute commands with filesystem isolation.

**Contents:**
- `internal/sandbox/sandbox.go` — Execution with PTY support
- `internal/sandbox/profile.go` — Profile generation
- `internal/sandbox/profile.sbpl.tmpl` — Seatbelt template
- Hardcoded Mach services and sysctl lists (embedded in template or constants)
- Unit tests for profile generation

**Review focus:** Security model correctness, PTY handling, template readability.

**Test manually:**
```bash
# Should fail (writes to disallowed path)  
veil -- touch /etc/test.txt

# Should fail (reads dotfile)
veil -- cat ~/.ssh/id_rsa

# Should succeed (writes to path allowed in config)
veil -- touch ./test.txt
```

### PR 3: Proxy + Policy (~350 LOC)

**Goal:** Working proxy with allowlist policy enforcement.

**Contents:**
- `internal/policy/policy.go` — Allowlist evaluation
- `internal/proxy/proxy.go` — Martian wrapper
- Unit tests for policy evaluation

**Review focus:** Policy evaluation correctness, proxy lifecycle management.

**Test manually:**
```bash
# With policy allowing github.com
veil -- curl https://github.com  # Should succeed

# With policy NOT allowing example.com
veil -- curl https://example.com  # Should fail (403)
```

### PR 4: Config + Integration (~200 LOC)

**Goal:** Wire everything together with config file support.

**Contents:**
- `internal/config/config.go` — Load from `~/.veilwarden/config.yaml`, validation
- `cmd/veil/main.go` — Full CLI implementation
- Integration tests (optional, can borrow from veilwarden)

**Review focus:** Config validation, error messages, edge cases.

**Test manually:**
```bash
# Create config
mkdir -p ~/.veilwarden
cat > ~/.veilwarden/config.yaml << 'EOF'
sandbox:
  allowed_write_paths:
    - ./
policy:
  allowlist:
    - "*.github.com"
    - "*.npmjs.org"
EOF

# Run with config
veil -- npm install
```

## LOC Summary

| Component | Go LOC | Template LOC | Total |
|-----------|--------|--------------|-------|
| CLI | 100 | — | 100 |
| Config | 150 | — | 150 |
| Sandbox | 200 | 150 | 350 |
| Proxy | 200 | — | 200 |
| Policy | 100 | — | 100 |
| **Total** | **750** | **150** | **~900** |

## Logging

- **File:** `~/.veilwarden/veil.log`
- **Format:** Structured JSON via `slog`
- **Sandbox violations:** Verbose output for debugging (includes denied path, operation type, etc.)
- **Proxy denials:** Log denied hosts with timestamp

## Future Considerations

These are explicitly out of scope for v1 but the architecture should not preclude them:

- **Credential injection** — Would require MITM with ephemeral CA
- **Multiple tiers** — Could add `tier: permissive` to config
- **OPA policies** — Could add `policy.opa` section alongside `policy.allowlist`
- **Secret backends** — Could add `secrets` section with Doppler/env config
- **Linux support** — Would add Bubblewrap backend alongside seatbelt
