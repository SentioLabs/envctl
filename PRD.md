# envctl - AWS Secrets Manager CLI for Local Development

## Overview

`envctl` is a lightweight CLI tool that enables developers to use AWS Secrets Manager as the single source of truth for application secrets during local development. It eliminates the need to maintain secrets in multiple places (e.g., Doppler AND AWS) by providing a Doppler-like developer experience backed entirely by AWS Secrets Manager.

## Goals

1. **Single source of truth**: Use the same secrets in local development that run in production
2. **Minimal secrets on disk**: Prefer injection into process environment; generate `.env` files only when needed (e.g., Docker Compose)
3. **Simple developer experience**: `envctl run -- make dev` should be all a developer needs
4. **Self-documenting**: Config files declare what secrets an app needs without containing values
5. **AWS-native**: Leverage existing AWS credentials and IAM policies
6. **Docker Compose compatible**: Support workflows that require `.env` files

## Non-Goals

- Production deployment (apps in production read from AWS directly)
- Secret rotation management (AWS handles this)
- Complex RBAC beyond AWS IAM
- Cross-cloud support

## Developer Workflows

### Workflow 1: Direct Execution (Preferred)

```bash
# Secrets injected into process memory, never touch disk
envctl run -- go run ./cmd/server
envctl run -- uv run main.py
envctl run -- bun run dev
```

### Workflow 2: Docker Compose

```bash
# Generate .env file for Docker Compose
envctl env > .env
docker compose build
docker compose up -d

# Or as a one-liner
envctl env > .env && docker compose up -d
```

**Important**: `.env` should be in `.gitignore` - it's a generated artifact, not source of truth.

## Configuration

### Project Configuration File

Each project contains a `.envctl.yaml` file (committed to repo):

```yaml
# .envctl.yaml
version: 1

# Default environment when -e/--env not specified
default_environment: dev

# Environment definitions
environments:
  dev:
    # Primary secret - all keys from this JSON blob are loaded
    secret: myapp/dev
    # Optional: AWS region (defaults to AWS_REGION or us-east-1)
    region: us-west-2
    
  staging:
    secret: myapp/staging
    
  prod:
    secret: myapp/prod

# Optional: Additional secrets to include
# Supports full secret (all keys) or specific key extraction
include:
  # Pull all keys from a shared secret
  - secret: shared/datadog
  
  # Pull specific key and optionally rename it
  - secret: shared/third-party-apis
    key: stripe_key           # Extract just this key from the JSON
    as: STRIPE_SECRET_KEY     # Rename it in the environment
    
  # Another example: pull specific key, keep original name
  - secret: shared/monitoring
    key: DD_API_KEY           # Will be exposed as DD_API_KEY

# Optional: Explicit env var mappings using AWS-style syntax
# Format: ENV_VAR_NAME: secret_name#key_name
# These take precedence over automatic loading
mapping:
  DATABASE_URL: myapp/dev#database_url
  REDIS_URL: myapp/dev#redis_url
  ANALYTICS_KEY: shared/analytics#api_key
```

### Configuration Precedence

When resolving environment variables, `envctl` applies sources in this order (later wins):

1. Primary `secret` for the environment (all keys loaded)
2. `include` entries (in order specified)
3. Explicit `mapping` entries
4. Command-line overrides (`--set KEY=VALUE`)

### AWS Secret Format

Secrets in AWS Secrets Manager are stored as JSON objects:

```json
{
  "DATABASE_URL": "postgres://user:pass@host:5432/db",
  "REDIS_URL": "redis://localhost:6379",
  "API_KEY": "sk-...",
  "STRIPE_SECRET_KEY": "sk_test_..."
}
```

### Secret Reference Syntax

For `mapping` entries, use the AWS-style syntax:

```
secret_name#key_name
```

Examples:
- `myapp/dev#DATABASE_URL` - key `DATABASE_URL` from secret `myapp/dev`
- `shared/datadog#api_key` - key `api_key` from secret `shared/datadog`

## CLI Commands

### `envctl run`

Primary command - runs a subprocess with secrets injected as environment variables.

```bash
# Use default environment from config
envctl run -- go run ./cmd/server
envctl run -- uv run main.py
envctl run -- bun run dev

# Specify environment
envctl run -e staging -- go run ./cmd/server

# Override config file location
envctl run -c ./config/.envctl.yaml -- npm start

# Add/override specific values
envctl run --set DEBUG=true --set LOG_LEVEL=debug -- make dev

# Verbose mode for debugging
envctl run -v -- make dev
```

**Behavior:**
- Reads `.envctl.yaml` from current directory (or walks up to find it)
- Fetches secret(s) from AWS Secrets Manager
- Merges with current environment (secrets take precedence)
- Executes command with merged environment
- Streams stdout/stderr from child process
- Exits with child process exit code

### `envctl env`

Outputs secrets in `.env` format. Primary use case is Docker Compose.

```bash
# Generate .env file for Docker Compose
envctl env > .env
docker compose up -d

# Specify environment
envctl env -e staging > .env

# Output to specific file
envctl env -o .env

# Append to existing file (useful for combining with non-secret config)
envctl env >> .env.local
```

**Behavior:**
- Outputs `KEY=VALUE` pairs, one per line
- Values with special characters are quoted appropriately
- Includes comment header with timestamp and environment name
- Writes to stdout by default (redirect as needed)

**Example output:**
```bash
# Generated by envctl from environment: dev
# Timestamp: 2024-01-15T10:30:00Z
# DO NOT COMMIT THIS FILE
DATABASE_URL="postgres://user:pass@host:5432/db"
REDIS_URL="redis://localhost:6379"
API_KEY="sk-..."
```

### `envctl export`

Outputs secrets in shell-eval format. For direnv or shell integration.

```bash
# For shell eval
eval "$(envctl export)"

# Specify environment
eval "$(envctl export -e staging)"

# Output formats
envctl export --format env      # KEY=VALUE (default, same as `envctl env`)
envctl export --format json     # {"KEY": "VALUE"}
envctl export --format shell    # export KEY="VALUE"
```

**Use cases:**
- Shell integration (direnv, bashrc)
- Debugging what values would be injected
- Piping to other tools

### `envctl list`

Lists available secret keys (not values) for documentation/debugging.

```bash
envctl list
# Output:
# DATABASE_URL        (from: myapp/dev)
# REDIS_URL           (from: myapp/dev)
# DD_API_KEY          (from: shared/datadog)
# STRIPE_SECRET_KEY   (from: mapping)

envctl list -e staging

# Show just key names, no source info
envctl list --quiet
```

### `envctl get`

Retrieves a single secret value. Useful for scripts.

```bash
envctl get DATABASE_URL
envctl get -e staging API_KEY

# Use in scripts
psql "$(envctl get DATABASE_URL)"

# Get from specific secret (bypass config)
envctl get --secret myapp/prod#DATABASE_URL
```

### `envctl validate`

Validates configuration and AWS connectivity without revealing secrets.

```bash
envctl validate
# Output:
# ✓ Config file: .envctl.yaml
# ✓ Environment: dev
# ✓ AWS credentials: valid (using profile: default)
# ✓ Secret 'myapp/dev': accessible (12 keys)
# ✓ Include 'shared/datadog': accessible (3 keys)
# ✓ Mapping: 3 entries resolved
# 
# Total: 18 environment variables will be set
```

### `envctl init`

Creates a starter `.envctl.yaml` in the current directory.

```bash
envctl init
# Creates .envctl.yaml with commented template

envctl init --secret myapp/dev
# Creates config pointing to existing secret

envctl init --discover
# Scans AWS for secrets matching common patterns, suggests config
```

## AWS Authentication

`envctl` uses the standard AWS SDK credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (if running on EC2/ECS)
4. SSO credentials (`aws sso login`)

No custom authentication is implemented - developers use their existing AWS access.

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

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Config file not found | Error with helpful message about `envctl init` |
| Invalid YAML | Error with line number and parsing details |
| AWS credentials missing | Error explaining credential chain, suggest `aws configure` or `aws sso login` |
| Secret not found | Error with secret name, suggest checking AWS console |
| Access denied | Error with IAM troubleshooting hints |
| Invalid secret format (not JSON) | Error explaining expected JSON format |
| Key not found in secret | Error listing available keys in that secret |
| Network timeout | Retry 3x with exponential backoff, then error |

All errors should go to stderr and result in non-zero exit code.

## Implementation Details

### Language & Dependencies

- **Language**: Go 1.21+
- **AWS SDK**: `github.com/aws/aws-sdk-go-v2`
- **YAML parsing**: `gopkg.in/yaml.v3`
- **CLI framework**: `github.com/spf13/cobra`

### Project Structure

```
envctl/
├── cmd/
│   └── envctl/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go      # YAML config parsing
│   │   └── resolver.go    # Secret reference resolution
│   ├── aws/
│   │   └── secrets.go     # AWS Secrets Manager client
│   ├── runner/
│   │   └── runner.go      # Subprocess execution
│   ├── env/
│   │   └── env.go         # Environment merging logic
│   └── output/
│       └── format.go      # Output formatting (env, json, shell)
├── .envctl.yaml           # Example config
├── go.mod
├── go.sum
└── README.md
```

### Security Considerations

1. **Never log secret values** - only key names in verbose mode
2. **Memory safety** - avoid keeping secrets in memory longer than necessary
3. **No shell expansion** - pass args directly to exec, not through shell
4. **File permissions** - if writing `.env`, warn if permissions are too open
5. **Gitignore reminder** - warn if `.env` is not in `.gitignore`

## Developer Workflow

### Initial Setup (one-time)

```bash
# Install the tool
go install github.com/yourorg/envctl@latest

# In project directory
envctl init --secret myapp/dev
# Edit .envctl.yaml as needed
git add .envctl.yaml

# Add .env to gitignore (important!)
echo ".env" >> .gitignore
```

### Daily Usage - Direct Execution

```bash
# Start development server (secrets in memory only)
envctl run -- go run ./cmd/server
envctl run -- uv run main.py
envctl run -- bun run dev

# Debug what's being injected
envctl list
envctl validate
```

### Daily Usage - Docker Compose

```bash
# Generate .env and start stack
envctl env > .env
docker compose up -d

# Or rebuild and restart
envctl env > .env && docker compose up -d --build

# Shorthand alias (add to .bashrc/.zshrc)
alias dcup='envctl env > .env && docker compose up -d'
```

### direnv Integration (Optional)

`.envrc` file (committed to repo):
```bash
# Use envctl for secrets
eval "$(envctl export)"
```

Then `direnv allow` once, and secrets auto-load when entering directory.

### CI/CD

Not needed - CI/CD environments should read from AWS directly using IAM roles. `envctl` is purely for local development.

## Future Enhancements (Out of Scope for MVP)

- `envctl set KEY=VALUE` - write secrets back to AWS
- `envctl diff` - compare environments
- `envctl edit` - open secrets in $EDITOR (decrypted temporarily)
- Shell completions (bash, zsh, fish)
- Secret caching with TTL (for offline/slow network)
- Integration with AWS Secrets Manager rotation
- Support for AWS Parameter Store as alternative backend
- `envctl compose` - wrapper that does `env > .env && docker compose $@`

## Success Criteria

1. Developer can run `envctl run -- make dev` and have all secrets injected
2. Developer can run `envctl env > .env && docker compose up` for Docker workflows
3. Single source of truth - change secret in AWS, next `run` or `env` picks it up
4. Flexible secret composition via `include` and `mapping` config
5. Clear error messages guide developers to solutions
6. Works with existing AWS SSO/credentials workflows
7. Config file clearly documents what secrets an app needs

## Appendix: Example Configurations

### Simple Single-Secret Setup

```yaml
# .envctl.yaml
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

### Complex Multi-Source Setup

```yaml
# .envctl.yaml
version: 1
default_environment: dev

environments:
  dev:
    secret: myapp/dev
    region: us-west-2
  staging:
    secret: myapp/staging
  prod:
    secret: myapp/prod

# Shared secrets merged into all environments
include:
  # All keys from datadog secret
  - secret: shared/datadog
  
  # Specific key with rename
  - secret: shared/stripe
    key: test_key
    as: STRIPE_SECRET_KEY
    
  # Specific key, keep name
  - secret: shared/sendgrid
    key: API_KEY

# Explicit mappings (override anything above)
mapping:
  # Override DATABASE_URL for local docker network
  DATABASE_URL: myapp/dev#DATABASE_URL_DOCKER
  
  # Pull from completely different secret
  LEGACY_API_KEY: legacy-system/credentials#api_key
```

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
