# envctl

A lightweight CLI tool that enables developers to use a secrets manager as the single source of truth for application secrets during local development.

## Table of Contents

- [Supported Backends](#supported-backends)
- [Features](#features)
- [Installation](#installation)
  - [Install Script](#install-script)
  - [From Source](#from-source)
  - [Build Locally](#build-locally)
  - [Updating](#updating)
  - [Shell Completions](#shell-completions)
- [Quick Start](#quick-start)
- [Usage](#usage)
  - [Direct Execution](#direct-execution-preferred)
  - [Docker Compose Workflow](#docker-compose-workflow)
  - [Shell Integration](#shell-integration)
  - [Inspect Secrets](#inspect-secrets)
- [Configuration](#configuration)
  - [Basic Configuration](#basic-configuration)
  - [Advanced Configuration](#advanced-configuration)
  - [Multi-Application Configuration](#multi-application-configuration)
  - [Mappings-Only Mode](#mappings-only-mode-default)
  - [Configuration Precedence](#configuration-precedence)
  - [AWS Secret Format](#aws-secret-format)
  - [Secret Reference Syntax](#secret-reference-syntax)
  - [Cache Configuration](#cache-configuration)
- [Caching](#caching)
  - [How It Works](#how-it-works)
  - [Security](#security)
  - [Cache Control](#cache-control)
- [AWS Setup](#aws-setup)
  - [Authentication](#authentication)
  - [Required IAM Permissions](#required-iam-permissions)
  - [Creating Secrets in AWS](#creating-secrets-in-aws)
- [1Password Setup](#1password-setup)
  - [Prerequisites](#prerequisites)
  - [Configuration](#configuration-1)
  - [Field Mapping](#how-1password-items-map-to-environment-variables)
  - [Creating Items](#creating-items-for-envctl)
  - [Authentication](#authentication-1)
- [CLI Reference](#cli-reference)
  - [Global Flags](#global-flags)
  - [Commands](#commands)
- [Examples](#examples)
- [Security](#security-1)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Supported Backends

| Backend | Authentication | Best For |
|---------|---------------|----------|
| **AWS Secrets Manager** | IAM credentials, SSO, profiles | Teams using AWS infrastructure |
| **1Password** | Biometrics (Touch ID, Windows Hello), CLI | Teams using 1Password for secrets |

## Features

- **Single source of truth** - Use the same secrets in local development that run in production
- **Multiple backends** - Support for AWS Secrets Manager and 1Password
- **Minimal secrets on disk** - Inject secrets directly into process environment; generate `.env` files only when needed
- **Simple developer experience** - `envctl run -- make dev` is all you need
- **Self-documenting** - Config files declare what secrets an app needs without containing values
- **Biometric authentication** - 1Password backend supports Touch ID, Windows Hello, and other biometrics
- **Docker Compose compatible** - Full support for `.env` file workflows
- **Intelligent caching** - Secure local caching reduces API calls and improves performance (AWS backend)
- **Self-updating** - Update to the latest version with `envctl self update`

## Installation

### Install Script

```bash
curl -fsSL https://raw.githubusercontent.com/sentiolabs/envctl/main/scripts/install.sh | bash
```

This detects your platform (macOS/Linux, amd64/arm64) and installs the latest release to `/usr/local/bin` or `~/.local/bin`.

### From Source

```bash
go install github.com/sentiolabs/envctl/cmd/envctl@latest
```

### Build Locally

```bash
git clone https://github.com/sentiolabs/envctl.git
cd envctl
make build
# Binary is at ./bin/envctl
```

### Updating

```bash
envctl self update
```

This checks GitHub for the latest release and updates in place. Use `--check` to see if an update is available without installing.

### Shell Completions

Enable tab completion for commands, flags, and environment names:

**Bash:**
```bash
# Linux
envctl completion bash > /etc/bash_completion.d/envctl

# macOS with Homebrew
envctl completion bash > $(brew --prefix)/etc/bash_completion.d/envctl

# Load in current session only
source <(envctl completion bash)
```

**Zsh:**
```bash
# Add to fpath (persistent)
envctl completion zsh > "${fpath[1]}/_envctl"

# Or for Oh My Zsh
envctl completion zsh > ~/.oh-my-zsh/completions/_envctl

# Load in current session only
source <(envctl completion zsh)
```

After installing completions, restart your shell or source your profile.

## Quick Start

### 1. Initialize Configuration

**For AWS Secrets Manager:**
```bash
cd your-project
envctl init --secret myapp/dev
```

**For 1Password:**
```bash
cd your-project
envctl init --backend 1password --secret "My App Secrets"
```

This creates `.envctl.yaml`:

<details>
<summary>AWS Secrets Manager config</summary>

```yaml
version: 1
default_environment: dev

environments:
  dev:
    secret: myapp/dev
```
</details>

<details>
<summary>1Password config</summary>

```yaml
version: 1
default_environment: dev

1pass:
  vault: Development

environments:
  dev:
    secret: My App Secrets
```
</details>

### 2. Validate Setup

```bash
envctl validate
```

Output:
```
✓ Config file: .envctl.yaml
✓ Environment: dev
✓ Mode: mappings-only (explicit keys only)
✓ Backend: aws (authenticated)
✓ Secret 'myapp/dev': accessible (5 keys)

Total: 5 environment variables will be set
```

### 3. Run Your Application

```bash
# Secrets injected directly into process memory
envctl run -- go run ./cmd/server
envctl run -- npm start
envctl run -- python app.py
```

## Usage

### Direct Execution (Preferred)

Secrets are injected into the process environment and never touch disk:

```bash
# Use default environment from config
envctl run -- go run ./cmd/server

# Specify environment
envctl run -e staging -- npm start

# Override specific values
envctl run --set DEBUG=true --set LOG_LEVEL=debug -- make dev

# Verbose mode for debugging
envctl run -v -- ./app
```

### Docker Compose Workflow

**Preferred: Direct injection (no files on disk)**

Define environment variables without values in your compose file:

```yaml
services:
  api:
    build: .
    environment:
      - DATABASE_URL
      - API_KEY
      - REDIS_URL
```

Then run with envctl:

```bash
envctl run -- docker compose up
```

Docker inherits the variables from envctl's environment - secrets never touch disk.

**Alternative: Generate .env file**

When direct injection isn't possible (e.g., detached mode, CI pipelines):

```bash
# Generate .env and start containers
envctl env > .env
docker compose up -d

# Or as a one-liner
envctl env > .env && docker compose up -d

# Write directly to file
envctl env -o .env
```

> **Important**: Add `.env` to your `.gitignore` - it's a generated artifact, not source of truth.

### Shell Integration

For direnv or shell eval:

```bash
# Export for current shell
eval "$(envctl export)"

# Different formats
envctl export --format shell  # export KEY="VALUE"
envctl export --format env    # KEY=VALUE
envctl export --format json   # {"KEY": "VALUE"}
```

### Inspect Secrets

```bash
# List all keys (not values) and their sources
envctl list
# Output:
# DATABASE_URL    (from: myapp/dev)
# REDIS_URL       (from: myapp/dev)
# DD_API_KEY      (from: shared/datadog)

# Quiet mode - just key names
envctl list --quiet

# Get a single value (for scripts)
envctl get DATABASE_URL
psql "$(envctl get DATABASE_URL)"

# Get from specific secret (bypass config)
envctl get --secret myapp/prod#API_KEY
```

## Configuration

### Basic Configuration

Create `.envctl.yaml` in your project root:

```yaml
version: 1
default_environment: dev

environments:
  dev:
    secret: myapp/dev
  staging:
    secret: myapp/staging
  prod:
    secret: myapp/prod
```

### Advanced Configuration

```yaml
version: 1
default_environment: dev

aws:
  region: us-east-1           # Global AWS defaults

environments:
  dev:
    secret: myapp/dev
    aws:
      region: us-west-2       # Override AWS region
      profile: mycompany-dev  # Use specific AWS profile
  staging:
    secret: myapp/staging
    aws:
      profile: mycompany-staging
  prod:
    secret: myapp/prod

# Include additional secrets (merged into all environments)
include:
  # Pull specific key and rename it
  - secret: shared/stripe
    key: test_key
    as: STRIPE_SECRET_KEY

  # Pull specific key, keep original name
  - secret: shared/sendgrid
    key: API_KEY

  # Pull all keys from a shared secret (requires include_all: true)
  - secret: shared/datadog

# Explicit mappings (highest precedence)
mapping:
  # Override DATABASE_URL for local Docker network
  DATABASE_URL: myapp/dev#DATABASE_URL_DOCKER

  # Pull from a different secret
  LEGACY_API_KEY: legacy-system/credentials#api_key
```

### Multi-Application Configuration

For monorepos or projects with multiple applications, use the `applications` block:

```yaml
version: 1
default_application: core-api
default_environment: dev

# Application-centric hierarchy
applications:
  core-api:
    dev:
      secret: dev/myorg/core-api/app-secrets
      aws:
        region: us-east-1
        profile: mycompany-dev
    staging:
      secret: staging/myorg/core-api/app-secrets
      aws:
        profile: mycompany-staging
    prod:
      secret: prod/myorg/core-api/app-secrets
      aws:
        region: us-west-2

  worker:
    dev:
      secret: dev/myorg/worker/app-secrets
    staging:
      secret: staging/myorg/worker/app-secrets
    # App-level includes and mappings
    include:
      - secret: shared/worker-specific
    mapping:
      WORKER_QUEUE: shared/queues#worker_url

# Global includes (apply to all applications)
include:
  - secret: shared/datadog

# Global mappings (apply to all applications)
mapping:
  DD_API_KEY: shared/datadog#api_key
```

Run with the `--app` flag:

```bash
# Use default application from config
envctl run -- make dev

# Specify application
envctl -a core-api -e dev run -- go run ./cmd/server
envctl -a worker -e staging run -- python worker.py

# Validate specific application
envctl validate -a core-api
```

When using applications:
- Global `include` and `mapping` entries apply to all applications
- App-level `include` and `mapping` override globals
- Both `--app/-a` and `--env/-e` flags support shell completion

### Mappings-Only Mode (Default)

By default, envctl only injects explicitly mapped keys. This is recommended because AWS secrets often use `snake_case` keys (e.g., `database_url`) while applications expect `SCREAMING_SNAKE_CASE` environment variables (e.g., `DATABASE_URL`).

To include all keys from the primary secret, set `include_all: true`:

```yaml
version: 1
default_environment: dev
include_all: true  # Global setting

environments:
  dev:
    secret: myapp/dev
    include_all: true  # Or per-environment
```

You can also use the `--include-all` CLI flag to override at runtime:

```bash
envctl run --include-all -- make dev
```

### Configuration Precedence

When resolving environment variables, sources are applied in this order (later wins):

**Default (mappings-only mode):**
1. Global `include` entries with specific keys
2. App-level `include` entries with specific keys
3. Global `mapping` entries
4. App-level `mapping` entries
5. Command-line overrides (`--set KEY=VALUE`)

**With `include_all: true`:**
1. Primary `secret` for the environment (all keys)
2. Global `include` entries (in order specified)
3. App-level `include` entries (in order specified)
4. Global `mapping` entries
5. App-level `mapping` entries
6. Command-line overrides (`--set KEY=VALUE`)

### AWS Secret Format

Secrets in AWS Secrets Manager can be JSON objects or plain text:

**JSON secrets (multiple key-value pairs):**
```json
{
  "DATABASE_URL": "postgres://user:pass@host:5432/db",
  "REDIS_URL": "redis://localhost:6379",
  "API_KEY": "sk-..."
}
```

**Plain text secrets (single value):**
```
my-redis-password
```

Plain text secrets are exposed as a single key named `_value`. Use the `as` field to rename it:

```yaml
include:
  - secret: myapp/redis-password
    key: _value
    as: REDIS_PASSWORD
```

### Secret Reference Syntax

For `mapping` entries, use the syntax:

```
secret_name#key_name
```

Examples:
- `myapp/dev#DATABASE_URL` - key `DATABASE_URL` from secret `myapp/dev`
- `shared/datadog#api_key` - key `api_key` from secret `shared/datadog`

### Cache Configuration

Configure caching behavior in your `.envctl.yaml`:

```yaml
version: 1
default_environment: dev

environments:
  dev:
    secret: myapp/dev

# Cache settings (all optional)
cache:
  enabled: true       # Enable/disable caching (default: true)
  ttl: "15m"          # Cache duration (default: 15m)
  backend: "auto"     # Backend: auto, keyring, file, none
```

#### Cache Backends

| Backend | Description |
|---------|-------------|
| `auto` | Automatically selects the best available backend (default) |
| `keyring` | Uses OS keyring (macOS Keychain, Linux secret-service) |
| `file` | Uses AES-256 encrypted files in `~/.cache/envctl/` |
| `none` | Disables caching |

## Caching

envctl caches secrets locally to improve performance and reduce AWS API calls. Caching is enabled by default with a 15-minute TTL.

### How It Works

1. **First request**: Fetches secret from AWS, stores encrypted in local cache
2. **Subsequent requests**: Returns cached value if still valid (within TTL)
3. **Expiration**: After TTL expires, next request fetches fresh data from AWS

### Security

Cached secrets are stored securely:

- **Keyring backend**: Uses OS-level credential storage (macOS Keychain, Linux secret-service)
- **File backend**: AES-256-GCM encryption with machine-derived keys
- **No plaintext**: Secrets are never stored in plaintext on disk
- **Auto-disabled**: Caching is automatically disabled when running as root

### Cache Control

```bash
# Bypass cache for a single command
envctl run --no-cache -- make dev

# Force refresh (fetch from AWS and update cache)
envctl run --refresh -- make dev

# Check cache status
envctl cache status

# Clear all cached secrets
envctl cache clear
```

## AWS Setup

### Authentication

envctl uses the standard AWS SDK credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (if running on EC2/ECS)
4. SSO credentials (`aws sso login`)

```bash
# Using AWS SSO (recommended)
aws sso login
envctl run -- make dev

# Using environment variables
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
envctl run -- make dev

# Using named profile (via environment)
export AWS_PROFILE=my-profile
envctl run -- make dev

# Using named profile (via config - preferred)
# In .envctl.yaml:
#   environments:
#     dev:
#       secret: myapp/dev
#       aws:
#         profile: my-profile
envctl run -- make dev
```

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue"
      ],
      "Resource": [
        "arn:aws:secretsmanager:*:*:secret:myapp/*",
        "arn:aws:secretsmanager:*:*:secret:shared/*"
      ]
    }
  ]
}
```

### Creating Secrets in AWS

```bash
# Create a new secret
aws secretsmanager create-secret \
  --name myapp/dev \
  --secret-string '{"DATABASE_URL":"postgres://localhost/myapp","API_KEY":"dev-key"}'

# Update an existing secret
aws secretsmanager put-secret-value \
  --secret-id myapp/dev \
  --secret-string '{"DATABASE_URL":"postgres://localhost/myapp","API_KEY":"new-key"}'
```

## 1Password Setup

### Prerequisites

1. **Install 1Password desktop app**

2. **Install 1Password CLI:**
   ```bash
   # macOS
   brew install --cask 1password-cli

   # Linux - see https://developer.1password.com/docs/cli/get-started/
   ```

3. **Enable CLI integration:**
   - Open 1Password → Settings → Developer
   - Enable "Integrate with 1Password CLI"

4. **Verify setup:**
   ```bash
   op account list
   op vault list
   ```

### Configuration

```yaml
version: 1
default_environment: dev

1pass:
  vault: Development        # Default vault name
  # account: my-account    # Optional: for multi-account setups

environments:
  dev:
    secret: My App Dev Secrets  # 1Password item name
  staging:
    secret: My App Staging Secrets
```

You can also mix backends per environment (e.g., 1Password for local dev, AWS for staging):

```yaml
version: 1
default_environment: local

environments:
  local:
    secret: My App Local Secrets
    1pass:
      vault: Development
  staging:
    secret: myapp/staging
    aws:
      region: us-east-1
      profile: mycompany-staging
```

### How 1Password Items Map to Environment Variables

1Password item fields become environment variables:

```
1Password Item: "My App Secrets"
├── DATABASE_URL  →  DATABASE_URL=postgres://...
├── API_KEY       →  API_KEY=sk-...
└── REDIS_URL     →  REDIS_URL=redis://...
```

Field labels become variable names. Only non-empty fields with labels are included.

### Creating Items for envctl

You can create items via the 1Password app or CLI:

```bash
# Create a Secure Note with custom fields
op item create \
  --category="Secure Note" \
  --title="My App Dev Secrets" \
  --vault="Development" \
  'DATABASE_URL=postgres://localhost:5432/myapp' \
  'API_KEY=sk-dev-12345' \
  'REDIS_URL=redis://localhost:6379'
```

### Authentication

The 1Password backend uses biometric authentication via the desktop app:
- **macOS**: Touch ID
- **Windows**: Windows Hello
- **Linux**: System authentication via PolKit

No tokens or credentials to manage - just unlock 1Password once per session.

## CLI Reference

### Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | `-c` | Config file path (default: `.envctl.yaml`) |
| `--app` | `-a` | Application name (default: from config) |
| `--env` | `-e` | Environment name (default: from config) |
| `--verbose` | `-v` | Enable verbose output |
| `--no-cache` | | Bypass secret cache for this request |
| `--refresh` | | Force refresh secrets and update cache |
| `--include-all` | | Include all keys from primary secret (override config) |

### Commands

#### `envctl run`

Run a command with secrets injected.

```bash
envctl run [flags] -- command [args...]

Flags:
  --set KEY=VALUE   Override or add environment variable (repeatable)
```

#### `envctl env`

Output secrets in `.env` format.

```bash
envctl env [flags]

Flags:
  -o, --output FILE   Write to file instead of stdout
```

#### `envctl export`

Output secrets in various formats.

```bash
envctl export [flags]

Flags:
  --format FORMAT   Output format: env, shell, json (default: shell)
```

#### `envctl list`

List available secret keys.

```bash
envctl list [flags]

Flags:
  -q, --quiet   Show only key names (no sources)
```

#### `envctl get`

Get a single secret value.

```bash
envctl get KEY [flags]

Flags:
  --secret REF   Get from specific secret (format: secret_name#key)
```

#### `envctl validate`

Validate configuration and backend connectivity.

```bash
envctl validate
```

#### `envctl init`

Create a starter configuration file.

```bash
envctl init [flags]

Flags:
  --secret NAME   Primary secret name for dev environment
```

#### `envctl completion`

Generate shell completion scripts.

```bash
envctl completion [bash|zsh]

Examples:
  envctl completion bash > /etc/bash_completion.d/envctl
  envctl completion zsh > "${fpath[1]}/_envctl"
  source <(envctl completion bash)
```

#### `envctl cache`

Manage the local secret cache.

```bash
# Show cache status and statistics
envctl cache status

# Clear all cached secrets
envctl cache clear
```

#### `envctl self update`

Update envctl to the latest version.

```bash
envctl self update [flags]

Flags:
  --check        Check for updates without installing
  -f, --force    Force reinstall even if up-to-date
  -y, --yes      Skip confirmation prompt

Examples:
  envctl self update          Update to latest version
  envctl self update --check  Check if an update is available
  envctl self update --force  Force reinstall
```

## Examples

### Monorepo Setup

```yaml
# services/api/.envctl.yaml
version: 1
default_environment: dev
environments:
  dev:
    secret: monorepo/api/dev
include:
  - secret: monorepo/shared/dev
```

```yaml
# services/worker/.envctl.yaml
version: 1
default_environment: dev
environments:
  dev:
    secret: monorepo/worker/dev
include:
  - secret: monorepo/shared/dev
```

### direnv Integration

Create `.envrc` in your project:

```bash
# .envrc
eval "$(envctl export)"
```

Then:

```bash
direnv allow
# Secrets auto-load when entering directory
```

### CI/CD Note

envctl is for **local development only**. In CI/CD and production:
- Use IAM roles attached to your compute (ECS tasks, Lambda, EC2)
- Access secrets directly via AWS SDK in your application
- Use AWS Secrets Manager's native integrations

## Security

- **Never logs secret values** - Only key names appear in verbose output
- **No shell expansion** - Commands are executed directly, preventing injection attacks
- **Memory safety** - Secrets are cleared from memory after use
- **File permission warnings** - Alerts if `.env` files have insecure permissions
- **Gitignore checks** - Warns if `.env` is not in `.gitignore`
- **Encrypted cache** - Cached secrets use AES-256-GCM encryption or OS keyring

## Troubleshooting

### "config file not found"

```bash
# Initialize a config file
envctl init --secret your-app/dev
```

### "AWS credentials not found"

```bash
# Check your AWS setup
aws sts get-caller-identity

# If using SSO, login first
aws sso login
```

### "secret not found"

```bash
# Verify the secret exists
aws secretsmanager describe-secret --secret-id myapp/dev

# Check the exact name in your config
cat .envctl.yaml
```

### "access denied"

Check your IAM permissions allow `secretsmanager:GetSecretValue` on the secret ARN.

### "invalid JSON format"

Ensure your AWS secret is a valid JSON object:

```bash
aws secretsmanager get-secret-value --secret-id myapp/dev --query SecretString --output text | jq .
```

### Stale cached secrets

If you've updated a secret in AWS and envctl is returning old values:

```bash
# Force refresh the cache
envctl run --refresh -- make dev

# Or clear the entire cache
envctl cache clear

# Or bypass cache entirely
envctl run --no-cache -- make dev
```

### Cache not working

```bash
# Check cache status
envctl cache status

# Common reasons cache is disabled:
# - Running as root user
# - CI environment detected
# - cache.enabled: false in config
```

## License

MIT
