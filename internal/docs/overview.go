package docs

const Overview = `envctl Documentation
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

  # Validate configuration and connectivity
  envctl validate

  # Run a command with secrets injected
  envctl run -- your-command

  # Generate .env file for Docker Compose
  envctl env > .env

For more information, see: https://github.com/sentiolabs/envctl
`
