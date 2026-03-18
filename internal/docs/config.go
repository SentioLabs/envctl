package docs

const Config = `Configuration File Format (.envctl.yaml)
=========================================

envctl uses a YAML configuration file to define how secrets are loaded
from your secrets backend (AWS Secrets Manager or 1Password).

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

  aws:                              # Global backend defaults
    region: us-east-1

  environments:
    dev:
      secret: myapp/dev             # Secret name (required)
      aws:                          # Per-environment backend override
        profile: mycompany-dev
      include_all: true             # Optional, include all keys from primary secret
    staging:
      secret: myapp/staging
    local:
      secret: My App Local          # 1Password item name
      1pass:                        # Use 1Password for this environment
        vault: Development

APPLICATION MODE (multiple applications)
Use when you have multiple apps sharing a config or want app-level isolation.

  version: 1
  default_application: api
  default_environment: dev

  aws:
    region: us-east-1

  applications:
    api:
      dev:
        secret: myorg/api/dev
        aws:
          profile: mycompany-dev
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

Mappings-Only Mode (Default)
----------------------------

By default, envctl only injects explicitly mapped keys. This is the
recommended approach because AWS secrets often use snake_case keys
(e.g., database_url) while apps expect SCREAMING_SNAKE_CASE (e.g., DATABASE_URL).

To include all keys from the primary secret, set include_all: true at
any level (global, application, or environment):

  # Global setting
  include_all: true

  # Or per-environment
  environments:
    dev:
      secret: myapp/dev
      include_all: true   # Enable for dev only

You can also use the --include-all CLI flag to override at runtime.

Including Additional Secrets
----------------------------

The 'include' block pulls in keys from other secrets:

include:
  # Include a SPECIFIC key (keeps original name) - always works
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

  # Include ALL keys from a secret - requires include_all: true
  - secret: shared/datadog

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

With include_all: true (all keys mode):
1. Primary secret (environment's 'secret' field) - lowest priority
2. Global 'include' entries (in order)
3. App-level 'include' entries (in order, if using applications)
4. Global 'mapping' entries
5. App-level 'mapping' entries (if using applications)
6. Command-line --set overrides - highest priority

Default (mappings-only mode):
1. Global 'include' entries with specific keys
2. App-level 'include' entries with specific keys
3. Global 'mapping' entries
4. App-level 'mapping' entries
5. Command-line --set overrides - highest priority

Note: In mappings-only mode, include entries without a 'key' field will error.

Secret Formats
--------------

AWS Secrets Manager secrets can be JSON objects or plain text:

  JSON (multiple key-value pairs):
    {"DATABASE_URL": "postgres://...", "API_KEY": "sk-..."}

  Plain text (single value):
    my-secret-password

  Plain text secrets are exposed with the key "_value". Use 'as' to rename:
    include:
      - secret: myapp/password
        key: _value
        as: MY_PASSWORD

1Password items map field labels to environment variable names.
See 'envctl docs 1password' for details on 1Password item structure.
`
