package docs

const Config = `Configuration File Format (.envctl.yaml)
=========================================

envctl uses a YAML configuration file to define how secrets are loaded
from AWS Secrets Manager.

Required Fields
---------------

version: 1                    # Config version (always 1)

Mode Selection
--------------

Choose ONE of these modes based on your needs:

LEGACY MODE (single application, simpler)
Use when you have one application or want a flat structure.

  version: 1
  default_environment: dev

  environments:
    dev:
      secret: myapp/dev           # AWS secret name (required)
      region: us-east-1           # Optional, defaults to AWS_REGION
    staging:
      secret: myapp/staging
    prod:
      secret: myapp/prod

APPLICATION MODE (multiple applications)
Use when you have multiple apps sharing a config or want app-level isolation.

  version: 1
  default_application: api
  default_environment: dev

  applications:
    api:
      dev:
        secret: myorg/api/dev
        region: us-east-1
      staging:
        secret: myorg/api/staging
      # App-level includes (only for this app)
      include:
        - secret: myorg/api/extra-secrets
      # App-level mappings (only for this app)
      mapping:
        API_SPECIAL_KEY: myorg/api/dev#special_key

    worker:
      dev:
        secret: myorg/worker/dev

Defaults
--------

default_environment: dev      # Used when --env not specified
default_application: api      # Used when --app not specified (app mode only)

Including Additional Secrets
----------------------------

The 'include' block pulls in keys from other AWS secrets:

include:
  # Include ALL keys from a secret
  - secret: shared/datadog

  # Include a SPECIFIC key (keeps original name)
  - secret: shared/stripe
    key: api_key

  # Include a specific key and RENAME it
  - secret: shared/stripe
    key: secret_key
    as: STRIPE_SECRET

  # Plain text secret (non-JSON) - use _value key
  - secret: myapp/redis-password
    key: _value
    as: REDIS_PASSWORD

Mapping Entries
---------------

The 'mapping' block creates explicit env var -> secret key mappings:

mapping:
  # Syntax: ENV_VAR_NAME: secret_name#key_name
  DATABASE_URL: myapp/dev#database_url
  API_KEY: shared/keys#production_api_key

Cache Configuration
-------------------

cache:
  enabled: true           # Enable/disable caching (default: true)
  ttl: "15m"              # Cache duration (default: 15m)
  backend: "auto"         # auto, keyring, file, or none

Precedence Rules
----------------

When the same key appears in multiple sources, later sources win:

1. Primary secret (environment's 'secret' field) - lowest priority
2. Global 'include' entries (in order)
3. App-level 'include' entries (in order, if using applications)
4. Global 'mapping' entries
5. App-level 'mapping' entries (if using applications)
6. Command-line --set overrides - highest priority

AWS Secret Format
-----------------

Secrets in AWS Secrets Manager can be:

JSON (multiple key-value pairs):
  {"DATABASE_URL": "postgres://...", "API_KEY": "sk-..."}

Plain text (single value):
  my-secret-password

Plain text secrets are exposed with the key "_value". Use 'as' to rename:
  include:
    - secret: myapp/password
      key: _value
      as: MY_PASSWORD
`
