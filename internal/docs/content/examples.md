Example Configurations
======================

1. Simple Single Environment
----------------------------

Minimal config for a single application with one environment.
When no 'aws' or '1pass' block is present, envctl defaults to AWS
Secrets Manager using the standard AWS SDK credential chain.

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev       # Uses AWS by default

Usage: envctl run -- npm start

2. Multiple Environments
------------------------

Support dev, staging, and production:

  version: 1
  default_environment: dev

  aws:
    region: us-east-1               # Global default region

  environments:
    dev:
      secret: myapp/dev
    staging:
      secret: myapp/staging
    prod:
      secret: myapp/prod
      aws:
        region: us-west-2           # Override region for prod

Usage:
  envctl run -- npm start              # uses dev (default)
  envctl -e staging run -- npm start   # uses staging
  envctl -e prod run -- npm start      # uses prod

3. With Shared Secrets (Source Lists)
-------------------------------------

Pull secrets from multiple sources per environment using the list format:

  version: 1
  default_environment: dev

  environments:
    dev:
      - secret: myapp/dev               # Primary source

      # All keys from shared Datadog config
      - secret: shared/datadog

      # Specific key from Stripe, renamed
      - secret: shared/stripe
        key: test_key
        as: STRIPE_API_KEY

      # Plain text Redis password
      - secret: myapp/redis-pass
        key: _value
        as: REDIS_PASSWORD

4. Multi-Application Setup
--------------------------

Multiple apps in a monorepo or shared config:

  version: 1
  default_application: api
  default_environment: dev

  # Global AWS config (applies to all apps/envs unless overridden)
  aws:
    region: us-east-1

  applications:
    api:
      dev:
        - secret: myorg/api/dev
          aws:
            profile: mycompany-dev     # Override profile for dev
        - secret: shared/datadog       # Shared source for this app
      staging:
        - secret: myorg/api/staging
          aws:
            profile: mycompany-staging
        - secret: shared/datadog

    worker:
      dev:
        - secret: myorg/worker/dev
        - secret: myorg/worker/queues  # Worker-specific source
      staging:
        - secret: myorg/worker/staging
        - secret: myorg/worker/queues

Usage:
  envctl -a api run -- go run ./cmd/api
  envctl -a worker run -- python worker.py

5. With Explicit Mappings
-------------------------

Override or rename keys from the primary secret:

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev

  mapping:
    # Use a different key for local Docker networking
    DATABASE_URL: myapp/dev#database_url_docker

    # Pull from a completely different secret
    LEGACY_API_KEY: legacy-system/creds#api_key

6. With Caching Configured
--------------------------

Customize cache behavior:

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev

  cache:
    enabled: true
    ttl: "30m"          # Cache for 30 minutes
    backend: "keyring"  # Use OS keyring

7. Cross-Backend Multi-Key Sources
-----------------------------------

Use 1Password as your primary backend while pulling specific keys
from an AWS secret in the same environment:

  version: 1
  default_environment: dev

  1pass:
    vault: Development

  aws:
    region: us-east-1

  default_backend: 1pass              # Required when both backends configured

  environments:
    dev:
      - secret: My App Dev Secrets    # Uses 1pass (default_backend)

      - secret: dev/bacstack/core-api/app-secrets
        backend: aws                  # Routes to AWS
        keys:
          - key: database_host
            as: DATABASE_HOST
          - key: database_password
            as: DATABASE_PASSWORD
          - key: database_user
            as: DATABASE_USER
          - key: database_name
            as: DATABASE_NAME

Usage: envctl -e dev run -- npm start

8. Docker Compose Integration
-----------------------------

For use with Docker Compose, define env vars without values:

docker-compose.yaml:
  services:
    api:
      build: .
      environment:
        - DATABASE_URL
        - REDIS_URL
        - API_KEY

.envctl.yaml:
  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev

Usage: envctl run -- docker compose up
