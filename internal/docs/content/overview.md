envctl Documentation
====================

envctl enables developers to use a secrets manager (AWS Secrets Manager or
1Password) as the single source of truth for application secrets during
local development.

Available Topics
----------------

  config      Configuration file format (.envctl.yaml)
  examples    Example configurations for common patterns
  k8s         Converting Kubernetes secrets to envctl
  patterns    Common integration patterns (Docker, direnv, etc.)
  1password   Using 1Password as a secrets backend

Run 'envctl docs <topic>' for detailed information on a topic.

Quick Start
-----------

  # Create a starter configuration
  envctl init --secret myapp/dev

  # Validate backend connectivity
  envctl validate

  # Run a command with secrets injected
  envctl run -- your-command

  # Generate .env file for Docker Compose
  envctl env > .env

Other Commands
--------------

  envctl export          Shell export statements (for eval)
  envctl get SECRET#KEY  Fetch a single secret value
  envctl list            List configured environments/applications
  envctl cache status    Show cache status
  envctl cache clear     Clear cached secrets

Global Flags
------------

  --config, -c FILE   Path to config file (default: auto-detect)
  --app, -a NAME      Select application (application mode)
  --env, -e NAME      Select environment
  --verbose, -v       Enable verbose output
  --no-cache          Disable secret caching
  --refresh           Bypass cache, fetch fresh secrets
  --include-all       Include all keys from primary secret

The 'run' command also supports:
  --set KEY=VALUE     Override or add environment variables

For more information, see: https://github.com/sentiolabs/envctl
