# Changelog

All notable changes to envctl will be documented in this file.

## [0.1.1] - 2026-01-16

### Features

- Add fallback version detection for go install

## [0.1.0] - 2026-01-16

### Documentation

- Document mappings-only mode and include_all option

### Features

- Add git-cliff changelog and version command
- Change default to mappings-only mode (**BREAKING**)
- Add AWS profile support in environment config

Allow specifying AWS profile per-environment, following the same pattern
as region. This eliminates the need for `export AWS_PROFILE=...` before
running envctl commands.

Example config:
  environments:
    dev:
      secret: myapp/dev
      profile: mycompany-dev
- Add timeout to keyring availability check

Prevents hanging when secret-service daemon is unresponsive on Linux
- Add docs command with config, examples, k8s, and patterns topics

- envctl docs shows overview and available topics

- envctl docs config shows full configuration schema

- envctl docs examples shows annotated example configurations

- envctl docs k8s shows Kubernetes secret conversion guide

- envctl docs patterns shows common integration patterns

- Shell completion for topic names
- Add plain text secrets and application hierarchy support

- Support plain text (non-JSON) secrets with _value key

- Add applications block for multi-app configs with per-app includes/mappings

- Add --app/-a flag with shell completion

- Update env builder to merge global and app-level config

- Document new features in README
- Add root main.go for go run/install support
- Add secret caching with encrypted backends

Implement TTL-based caching to reduce AWS API calls:

- Add cache package with Backend interface and Entry types
- Implement encrypted file cache backend (AES-256-GCM)
- Implement OS keyring backend (macOS Keychain, Linux secret-service)
- Auto-select best backend with fallback chain
- Auto-disable in CI environments and when running as root
- Add --no-cache and --refresh global flags
- Add cache command with clear and status subcommands
- Support cache configuration in .envctl.yaml
- Update README with cache documentation
- Add shell completions for bash and zsh

- Add completion command using Cobra's built-in generation
- Custom completion for --env flag reads environment names from config
- Update README with installation instructions
- Add comprehensive README with usage examples

Includes:
- Installation instructions
- Quick start guide
- All CLI commands with examples
- Configuration reference
- AWS setup and IAM permissions
- Monorepo and direnv examples
- Security notes and troubleshooting
- Implement envctl CLI for AWS Secrets Manager local development

This implements the full envctl CLI as specified in PRD.md:

- Core packages: config, aws, env, output, runner, errors
- CLI commands: run, env, export, list, get, validate, init
- Unit tests for config parsing and output formatting
- Example config and Makefile

The tool enables developers to inject AWS Secrets Manager secrets
into local development environments, either by running commands
directly or generating .env files for Docker Compose.


