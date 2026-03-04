1Password Backend
==================

envctl supports 1Password as an alternative to AWS Secrets Manager for
local development. It uses the 1Password CLI (op) to access secrets with
biometric authentication support.

Prerequisites
-------------

1. Install 1Password desktop app
2. Install 1Password CLI:
   - macOS: brew install --cask 1password-cli
   - Linux: https://developer.1password.com/docs/cli/get-started/
   - Windows: winget install AgileBits.1Password.CLI

3. Enable CLI integration in 1Password:
   - Open 1Password > Settings > Developer
   - Enable "Integrate with 1Password CLI"

4. Verify setup:
   op account list
   op vault list

Configuration
-------------

To use 1Password, add a '1pass' block to your config. The backend is
determined by the presence of a '1pass' block (there is no 'backend' field).

  version: 1

  1pass:
    vault: Development    # Default vault name
    account: my-team      # Optional: for multi-account setups

  environments:
    dev:
      secret: My App Dev  # 1Password item name

You can also set 1Password per-environment while using AWS globally:

  version: 1

  aws:
    region: us-east-1

  environments:
    dev:
      secret: myapp/dev           # Uses AWS (from global aws: block)
    local:
      secret: My App Local
      1pass:
        vault: Development        # This env uses 1Password instead

COMMON MISTAKES (these will cause parse errors):
  - backend: 1password    # NOT a valid field - no such field exists
  - onepassword:          # Wrong key - the correct key is '1pass'

Quick Start
-----------

  # Initialize with 1Password backend
  envctl init --backend 1password --secret "My App Secrets"

  # Validate connectivity (will prompt for biometric if needed)
  envctl validate

  # Run with secrets
  envctl run -- npm start

1Password Item Structure
------------------------

envctl maps 1Password item fields to environment variables:

  Item: "My App Dev"
  +-- username --> USERNAME
  +-- password --> PASSWORD
  +-- api_key --> API_KEY
  +-- database_url --> DATABASE_URL

Field labels become environment variable names. Only non-empty fields
with labels are included.

Secret References
-----------------

The 'secret' field supports multiple reference formats:

  # Just item name (uses vault from 1pass: block)
  secret: My App Secrets

  # Vault and item
  secret: Development/My App Secrets

  # Full op:// format (op:// prefix is stripped, parsed the same as above)
  secret: op://Development/My App Secrets

  # With field (for include entries or specific field access)
  secret: op://Development/My App Secrets/api_key

For mappings, use the standard secret#key syntax:

  mapping:
    DATABASE_URL: Development/Database#connection_string
    API_KEY: API Keys#production

Here 'Development/Database' is the vault/item reference and
'connection_string' is the field label, separated by '#'.

1pass Block Reference
---------------------

The '1pass' block can appear at three levels:

  Global (applies to all environments):
    1pass:
      vault: Development
      account: my-team

  Per-environment (overrides global):
    environments:
      dev:
        secret: My App Dev
        1pass:
          vault: Dev Vault

  Per-include entry (cross-backend access):
    include:
      dev:
        - secret: op://SharedVault/Shared Secrets
          key: api_key
          as: SHARED_API_KEY
          1pass:
            vault: SharedVault

Fields:
  vault    - Default vault name for lookups
  account  - 1Password account identifier (for multi-account setups).
             Accepted formats: short domain (my-team), full URL
             (my-team.1password.com), or account ID. The short domain
             is recommended.

Comparison with AWS
-------------------

  Feature              | AWS                | 1Password
  ---------------------|--------------------|-----------------
  Authentication       | IAM credentials    | Biometrics/CLI
  Secret format        | JSON key-value     | Item fields
  Backend selection    | aws: block         | 1pass: block
  Mapping syntax       | secret_name#key    | vault/item#field
  Caching             | Supported          | Relies on CLI
  CI/CD support       | IAM roles          | Service accounts

When to Use 1Password
---------------------

1Password is ideal when:
- You use 1Password for team password management
- You prefer biometric authentication over AWS credentials
- Your team doesn't use AWS in local development
- You want a simpler setup for new developers

Use AWS when:
- You're already using AWS in production
- You need consistent secret format across environments
- You want envctl caching for offline access
- Your CI/CD uses IAM roles

Tips
----

1. Create a "Development" vault for local dev secrets
2. Use descriptive item names (e.g., "MyApp Dev Secrets")
3. Use consistent field labels across environments
4. The CLI caches authentication - unlock 1Password once per session

For more information:
- 1Password CLI: https://developer.1password.com/docs/cli/
- Biometric setup: https://developer.1password.com/docs/cli/get-started/
