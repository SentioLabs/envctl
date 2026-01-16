# envctl

A lightweight CLI tool that enables developers to use AWS Secrets Manager as the single source of truth for application secrets during local development.

## Features

- **Single source of truth** - Use the same secrets in local development that run in production
- **Minimal secrets on disk** - Inject secrets directly into process environment; generate `.env` files only when needed
- **Simple developer experience** - `envctl run -- make dev` is all you need
- **Self-documenting** - Config files declare what secrets an app needs without containing values
- **AWS-native** - Leverages existing AWS credentials and IAM policies
- **Docker Compose compatible** - Full support for `.env` file workflows
- **Intelligent caching** - Secure local caching reduces AWS API calls and improves performance

## Installation

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

```bash
cd your-project
envctl init --secret myapp/dev
```

This creates `.envctl.yaml`:

```yaml
version: 1
default_environment: dev

environments:
  dev:
    secret: myapp/dev
```

### 2. Validate Setup

```bash
envctl validate
```

Output:
```
✓ Config file: .envctl.yaml
✓ Environment: dev
✓ Mode: mappings-only (explicit keys only)
✓ AWS credentials: valid
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

environments:
  dev:
    secret: myapp/dev
    region: us-west-2       # Override AWS region
    profile: mycompany-dev  # Use specific AWS profile
  staging:
    secret: myapp/staging
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
      region: us-east-1
      profile: mycompany-dev
    staging:
      secret: staging/myorg/core-api/app-secrets
      profile: mycompany-staging
    prod:
      secret: prod/myorg/core-api/app-secrets
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
- **Auto-disabled**: Caching is automatically disabled in CI environments and when running as root

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

### CI/CD Environments

Caching is automatically disabled when these environment variables are detected:
- `CI`
- `GITHUB_ACTIONS`
- `GITLAB_CI`
- `JENKINS_URL`
- `CIRCLECI`
- `TRAVIS`
- `BUILDKITE`

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
#       profile: my-profile
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

Validate configuration and AWS connectivity.

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
- **CI-aware** - Caching automatically disabled in CI environments

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
