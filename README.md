# envctl

A lightweight CLI tool that enables developers to use AWS Secrets Manager as the single source of truth for application secrets during local development.

## Features

- **Single source of truth** - Use the same secrets in local development that run in production
- **Minimal secrets on disk** - Inject secrets directly into process environment; generate `.env` files only when needed
- **Simple developer experience** - `envctl run -- make dev` is all you need
- **Self-documenting** - Config files declare what secrets an app needs without containing values
- **AWS-native** - Leverages existing AWS credentials and IAM policies
- **Docker Compose compatible** - Full support for `.env` file workflows

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

Generate `.env` files when Docker Compose requires them:

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
    region: us-west-2  # Override AWS region
  staging:
    secret: myapp/staging
  prod:
    secret: myapp/prod

# Include additional secrets (merged into all environments)
include:
  # Pull all keys from a shared secret
  - secret: shared/datadog

  # Pull specific key and rename it
  - secret: shared/stripe
    key: test_key
    as: STRIPE_SECRET_KEY

  # Pull specific key, keep original name
  - secret: shared/sendgrid
    key: API_KEY

# Explicit mappings (highest precedence)
mapping:
  # Override DATABASE_URL for local Docker network
  DATABASE_URL: myapp/dev#DATABASE_URL_DOCKER

  # Pull from a different secret
  LEGACY_API_KEY: legacy-system/credentials#api_key
```

### Configuration Precedence

When resolving environment variables, sources are applied in this order (later wins):

1. Primary `secret` for the environment (all keys)
2. `include` entries (in order specified)
3. Explicit `mapping` entries
4. Command-line overrides (`--set KEY=VALUE`)

### AWS Secret Format

Secrets in AWS Secrets Manager must be JSON objects:

```json
{
  "DATABASE_URL": "postgres://user:pass@host:5432/db",
  "REDIS_URL": "redis://localhost:6379",
  "API_KEY": "sk-..."
}
```

### Secret Reference Syntax

For `mapping` entries, use the syntax:

```
secret_name#key_name
```

Examples:
- `myapp/dev#DATABASE_URL` - key `DATABASE_URL` from secret `myapp/dev`
- `shared/datadog#api_key` - key `api_key` from secret `shared/datadog`

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

# Using named profile
export AWS_PROFILE=my-profile
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
| `--env` | `-e` | Environment name (default: from config) |
| `--verbose` | `-v` | Enable verbose output |

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

## License

MIT
