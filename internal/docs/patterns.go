package docs

const Patterns = `Common Integration Patterns
===========================

Docker Compose
--------------

PREFERRED: Direct Injection (secrets never touch disk)

1. Define env vars WITHOUT values in docker-compose.yaml:

  services:
    api:
      build: .
      environment:
        - DATABASE_URL
        - REDIS_URL
        - API_KEY

2. Run with envctl:

  envctl run -- docker compose up

Docker inherits the variables from envctl's environment.

ALTERNATIVE: Generate .env file (for detached mode)

  envctl env > .env
  docker compose up -d

Remember to add .env to .gitignore!

direnv Integration
------------------

Create .envrc in your project:

  # .envrc
  eval "$(envctl export)"

Then:

  direnv allow

Secrets auto-load when you cd into the directory.

NOTE: This exports secrets to your shell environment. Use only in
directories where you trust all tools and scripts.

Shell Wrapper Scripts
---------------------

Create run-local.sh for hardcoded development values:

  #!/bin/bash
  set -euo pipefail

  exec envctl run \
    --set APP_ENV="development" \
    --set LOG_LEVEL="debug" \
    --set DD_AGENT_HOST="localhost" \
    --set DD_AGENT_PORT="8126" \
    -- "$@"

Usage:

  ./run-local.sh go run ./cmd/server
  ./run-local.sh npm start

This separates:
- Secrets (from AWS via envctl)
- Local dev overrides (hardcoded in script)

Monorepo Setup
--------------

Option 1: Per-app configs (simpler)

  monorepo/
    services/
      api/
        .envctl.yaml      # api-specific config
        run-local.sh
      worker/
        .envctl.yaml      # worker-specific config
        run-local.sh

Each .envctl.yaml uses legacy mode:

  version: 1
  environments:
    dev:
      secret: myorg/api/dev

Option 2: Shared config (centralized)

  monorepo/
    .envctl.yaml          # shared config with applications
    services/
      api/
        run-local.sh
      worker/
        run-local.sh

Root .envctl.yaml uses application mode:

  version: 1
  default_environment: dev

  applications:
    api:
      dev:
        secret: myorg/api/dev
    worker:
      dev:
        secret: myorg/worker/dev

  # Shared across all apps
  include:
    - secret: shared/datadog

Run from anywhere:

  envctl -a api run -- make dev
  envctl -a worker run -- python worker.py

Makefile Integration
--------------------

Add targets that use envctl:

  .PHONY: dev test

  dev:
      envctl run -- go run ./cmd/server

  test:
      envctl run -- go test ./...

  docker-up:
      envctl run -- docker compose up

CI/CD Considerations
--------------------

envctl is for LOCAL DEVELOPMENT only. In CI/CD and production:

1. Use IAM roles attached to compute (ECS tasks, Lambda, EC2)
2. Access secrets directly via AWS SDK in your application
3. Use AWS Secrets Manager native integrations

If you need envctl in CI for testing:

  # CI pipeline
  - name: Run tests
    env:
      AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
      AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
    run: |
      envctl run --no-cache -- go test ./...

Note: --no-cache is recommended in CI to avoid stale secrets.

Multiple AWS Accounts
---------------------

If dev/staging/prod are in different AWS accounts:

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev
      aws:
        region: us-east-1       # Dev account
        profile: dev
    staging:
      secret: myapp/staging
      aws:
        region: us-east-1       # Staging account
        profile: staging
    prod:
      secret: myapp/prod
      aws:
        region: us-west-2       # Prod account
        profile: prod

Each environment's aws.profile is used automatically:

  envctl -e dev run -- make dev         # uses 'dev' profile
  envctl -e staging run -- make dev     # uses 'staging' profile
`
