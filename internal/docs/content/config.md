Configuration File Format (.envctl.yaml)
=========================================

envctl uses a YAML configuration file to define how secrets are loaded
from a secrets backend. Unknown fields cause a parse error, so every
field must match the structure documented here exactly.

Required Fields
---------------

version: 1                    # Config version (always 1)

Backend Selection
-----------------

The backend is determined by the presence of an 'aws' or '1pass' block —
there is no 'backend' field. When neither block is present, envctl defaults
to AWS Secrets Manager using the standard AWS SDK credential chain.

  # Global AWS config (applies to all environments)
  aws:
    region: us-east-1
    profile: mycompany-dev

  # Or per-environment override (inside source entries)
  environments:
    dev:
      - secret: myapp/dev
        aws:
          region: us-west-2       # Override region for this source

  # 1Password backend (use '1pass', NOT 'onepassword' or 'backend')
  1pass:
    vault: Development
    account: my-team            # Optional: for multi-account setups

COMMON MISTAKES (these cause parse errors):
  - backend: 1password     # NOT a valid field
  - onepassword:           # Wrong key, correct key is '1pass'
  - region: us-east-1      # On environment level — must be under aws:
  - profile: mycompany     # On environment level — must be under aws:

Mode Selection
--------------

Choose ONE of these modes based on your needs:

LEGACY MODE (single application, simpler)
Use when you have one application or want a flat structure.

  version: 1
  default_environment: dev

  aws:
    region: us-east-1

  environments:
    dev:
      secret: myapp/dev           # Single source (shorthand mapping format)
    staging:
      - secret: myapp/staging     # List format (even with one source)
    prod:
      - secret: myapp/prod
        aws:
          region: us-west-2       # Per-source region override
      - secret: shared/monitoring
        key: api_key
        as: MONITORING_KEY

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
        - secret: myorg/api/dev
          aws:
            profile: mycompany-dev
        - secret: shared/datadog
          key: api_key
          as: DD_API_KEY
      staging:
        secret: myorg/api/staging
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

Environment Source Lists
------------------------

Each environment is an ordered list of secret sources. Sources can be
specified in two YAML formats:

MAPPING FORMAT (single source shorthand):
  environments:
    local:
      secret: myapp/local

LIST FORMAT (multiple sources):
  environments:
    dev:
      - secret: myapp/dev           # Primary source
      - secret: shared/datadog      # Additional source
        key: api_key
        as: DD_API_KEY
      - secret: shared/database
        aws:
          region: us-east-1
        keys:
          - key: database_host
            as: DATABASE_HOST
          - key: database_password
            as: DATABASE_PASSWORD

The first source is the primary secret. When include_all is enabled,
all keys from the primary are loaded into the environment.

Later sources override earlier ones when keys conflict.

Source Entry Options
--------------------

Each source entry supports these fields:

  - secret: shared/stripe       # Required: secret reference
    key: api_key                # Optional: extract single key
    as: STRIPE_KEY              # Optional: rename the key

  - secret: shared/database
    keys:                       # Optional: extract multiple keys
      - key: db_host
        as: DATABASE_HOST
      - key: db_pass            # 'as' is optional, defaults to key name

  - secret: shared/datadog      # No key/keys: loads ALL keys
                                # (requires include_all: true, or errors)

  - secret: aws-secret/data     # Cross-backend source
    aws:                        # Override backend for this source
      region: us-east-1

NOTE: 'key' and 'keys' are mutually exclusive on the same source entry.
Use 'key' for a single key, 'keys' for multiple keys from the same secret.

Mappings-Only Mode (Default)
----------------------------

By default, envctl only injects explicitly mapped or keyed entries.
This is recommended because secrets often use snake_case keys
(e.g., database_url) while apps expect SCREAMING_SNAKE_CASE (e.g., DATABASE_URL).

To include all keys from sources, set include_all: true at
any level (global, application, or environment):

  # Global setting
  include_all: true

  # Or per-environment (mapping format only)
  environments:
    dev:
      secret: myapp/dev
      include_all: true   # Enable for dev only

You can also use the --include-all CLI flag to override at runtime.
Use --refresh to bypass the cache and fetch fresh secrets.

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

Field Reference
---------------

Every valid YAML field at each nesting level:

Config (root level):
  version: int                         # Required, must be 1
  default_application: string          # Optional
  default_environment: string          # Optional
  include_all: bool                    # Optional
  aws: AWSConfig                       # Optional (see below)
  1pass: OnePassConfig                 # Optional (see below)
  environments: map[string]Environment # Legacy mode
  applications: map[string]Application # Application mode
  mapping: map[string]string           # ENV_VAR: secret#key
  cache: CacheConfig                   # Optional

Application:
  <env_name>: Environment              # Inlined environment map
  mapping: map[string]string
  include_all: bool

Environment (mapping format — single source):
  secret: string                       # Required
  include_all: bool                    # Optional
  aws: AWSConfig                       # Optional
  1pass: OnePassConfig                 # Optional

Environment (list format — multiple sources):
  - IncludeEntry                       # Ordered list of sources

IncludeEntry (source entry):
  secret: string                       # Required
  key: string                          # Optional: specific key from secret
  as: string                           # Optional: rename the key
  keys: []KeyMapping                   # Optional: multiple keys (mutually exclusive with key)
  aws: AWSConfig                       # Optional: cross-backend override
  1pass: OnePassConfig                 # Optional: cross-backend override

KeyMapping:
  key: string                          # Required: key name in the secret
  as: string                           # Optional: rename (defaults to key name)

AWSConfig:
  region: string                       # AWS region
  profile: string                      # AWS CLI profile name

OnePassConfig:
  vault: string                        # 1Password vault name
  account: string                      # 1Password account (short domain, full URL, or ID)

CacheConfig:
  enabled: bool                        # Default: true
  ttl: string                          # Duration string (e.g. "15m", "1h")
  backend: string                      # "auto", "keyring", "file", "none"

Precedence Rules
----------------

When the same key appears in multiple sources, later sources win:

With include_all: true (all keys mode):
1. Primary source (first entry in source list) - lowest priority
2. Additional source entries (in order, later overrides earlier)
3. Global 'mapping' entries
4. App-level 'mapping' entries (if using applications)
5. Command-line --set overrides - highest priority

Default (mappings-only mode):
1. Source entries with explicit key/keys (in order)
2. Global 'mapping' entries
3. App-level 'mapping' entries
4. Command-line --set overrides - highest priority

Note: In mappings-only mode, source entries without a 'key' or 'keys'
field (other than the primary) will error. The primary source is
silently skipped when it has no explicit keys.

AWS Secret Format
-----------------

Secrets in AWS Secrets Manager can be:

JSON (multiple key-value pairs):
  {"DATABASE_URL": "postgres://...", "API_KEY": "sk-..."}

Plain text (single value):
  my-secret-password

Plain text secrets are exposed with the key "_value". Use 'as' to rename:
  environments:
    dev:
      - secret: myapp/password
        key: _value
        as: MY_PASSWORD
