Converting Kubernetes Secrets to envctl
========================================

This guide helps you convert Kubernetes secret configurations to envctl
format for local development.

secretKeyRef Pattern
--------------------

Kubernetes secretKeyRef in a deployment:

  env:
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef:
          name: myapp-secrets    # AWS secret name
          key: database_url      # Key within the secret

Converts to envctl mapping:

  mapping:
    DATABASE_URL: myapp-secrets#database_url

External Secrets Operator Pattern
---------------------------------

If using External Secrets Operator with AWS:

ExternalSecret:
  spec:
    secretStoreRef:
      name: aws-secrets-manager
    target:
      name: myapp-secrets
    data:
      - secretKey: database_url
        remoteRef:
          key: prod/myapp/secrets    # AWS secret path
          property: database_url

This means your AWS secret is at "prod/myapp/secrets". Use directly:

  environments:
    prod:
      secret: prod/myapp/secrets

Full Conversion Example
-----------------------

Given this Kubernetes values.yaml:

  env:
    # Hardcoded values (non-secrets)
    - name: APP_ENV
      value: "production"
    - name: LOG_LEVEL
      value: "info"

    # From primary app secret
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef:
          name: myapp-secrets
          key: database_url
    - name: API_KEY
      valueFrom:
        secretKeyRef:
          name: myapp-secrets
          key: api_key

    # From shared secret
    - name: DD_API_KEY
      valueFrom:
        secretKeyRef:
          name: datadog-secrets
          key: api_key

    # Plain text secret (like ElastiCache password)
    - name: REDIS_PASSWORD
      valueFrom:
        secretKeyRef:
          name: redis-auth
          key: password

Converts to .envctl.yaml (AWS is the default backend when no 'aws'
or '1pass' block is specified):

  version: 1
  default_environment: dev

  environments:
    dev:
      - secret: myapp-secrets       # Primary source (uses AWS by default)

      # Shared Datadog secret
      - secret: datadog-secrets
        key: api_key
        as: DD_API_KEY

      # Plain text Redis password (use _value for non-JSON secrets)
      - secret: redis-auth
        key: _value
        as: REDIS_PASSWORD

And run-local.sh for hardcoded values:

  #!/bin/bash
  exec envctl run \
    --set APP_ENV="development" \
    --set LOG_LEVEL="debug" \
    -- "$@"

Decision Guide: Applications vs Environments
--------------------------------------------

Use LEGACY MODE (environments only) when:
  - You have a single application
  - Each app has its own .envctl.yaml in its directory
  - You want the simplest possible config

Use APPLICATION MODE when:
  - Multiple apps share one config file
  - Apps share common secrets but have app-specific ones too
  - You're in a monorepo and want centralized config

Example decision:

Separate repos/directories (use legacy mode):
  api/.envctl.yaml         -> environments: dev, staging, prod
  worker/.envctl.yaml      -> environments: dev, staging, prod

Monorepo with shared config (use application mode):
  .envctl.yaml:
    applications:
      api:
        dev: ...
      worker:
        dev: ...

Helm Values Pattern
-------------------

If your Helm chart uses a values.yaml pattern like:

  secrets:
    - envName: DATABASE_URL
      secretName: myapp
      secretKey: db_url
    - envName: API_KEY
      secretName: myapp
      secretKey: api_key

Convert each entry to a mapping:

  mapping:
    DATABASE_URL: myapp#db_url
    API_KEY: myapp#api_key
