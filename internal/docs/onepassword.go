package docs

const OnePassword = `1Password Backend
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

To use 1Password, set 'backend: 1password' in your config:

  version: 1
  backend: 1password

  onepassword:
    vault: Development    # Default vault name

  environments:
    dev:
      secret: My App Dev  # 1Password item name

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
  ├── username → USERNAME
  ├── password → PASSWORD
  ├── api_key → API_KEY
  └── database_url → DATABASE_URL

Field labels become environment variable names. Only non-empty fields
with labels are included.

Secret References
-----------------

You can reference secrets using multiple formats:

  # Just item name (uses configured default vault)
  secret: My App Secrets

  # Vault and item
  secret: Development/My App Secrets

  # Full op:// format
  secret: op://Development/My App Secrets

For mappings, use the same format with field name:

  mapping:
    DATABASE_URL: Development/Database#connection_string
    API_KEY: API Keys#production

Comparison with AWS
-------------------

  Feature              | AWS                | 1Password
  ---------------------|--------------------|-----------------
  Authentication       | IAM credentials    | Biometrics/CLI
  Secret format        | JSON key-value     | Item fields
  Reference syntax     | secret_name#key    | vault/item#field
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
`
