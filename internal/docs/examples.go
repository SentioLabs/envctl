package docs

const Examples = `Example Configurations
======================

1. Simple Single Environment
----------------------------

Minimal config for a single application with one environment:

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev

Usage: envctl run -- npm start

2. Multiple Environments
------------------------

Support dev, staging, and production:

  version: 1
  default_environment: dev

  aws:
    region: us-east-1

  environments:
    dev:
      secret: myapp/dev
    staging:
      secret: myapp/staging
    prod:
      secret: myapp/prod
      aws:
        region: us-west-2     # Override region for prod

Usage:
  envctl run -- npm start              # uses dev (default)
  envctl -e staging run -- npm start   # uses staging
  envctl -e prod run -- npm start      # uses prod

3. Mixed Backends (Per-Environment)
------------------------------------

Use 1Password for local development and AWS for deployed environments:

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
    prod:
      secret: myapp/prod
      aws:
        region: us-west-2

Usage:
  envctl run -- npm start              # uses local with 1Password
  envctl -e staging run -- npm start   # uses staging with AWS

5. With Shared Secrets
----------------------

Include secrets shared across applications:

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev

  include:
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

6. Multi-Application Setup
--------------------------

Multiple apps in a monorepo or shared config:

  version: 1
  default_application: api
  default_environment: dev

  applications:
    api:
      dev:
        secret: myorg/api/dev
      staging:
        secret: myorg/api/staging

    worker:
      dev:
        secret: myorg/worker/dev
      staging:
        secret: myorg/worker/staging
      # Worker-specific secrets
      include:
        - secret: myorg/worker/queues

  # Global includes (apply to all apps)
  include:
    - secret: shared/datadog

Usage:
  envctl -a api run -- go run ./cmd/api
  envctl -a worker run -- python worker.py

7. With Explicit Mappings
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

8. With Caching Configured
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

9. Docker Compose Integration
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
`
