# Changelog

All notable changes to envctl will be documented in this file.

## [0.3.1](https://github.com/SentioLabs/envctl/compare/v0.3.0...v0.3.1) (2026-03-04)


### Bug Fixes

* **install:** prevent silent exit when version lacks v prefix ([18a51bc](https://github.com/SentioLabs/envctl/commit/18a51bc53eeae4cb24671f2838171e49b904b452))

## [0.3.0](https://github.com/SentioLabs/envctl/compare/v0.2.0...v0.3.0) (2026-03-03)


### Features

* **cmd:** wire per-environment backend resolution into CLI commands ([8ada2fc](https://github.com/SentioLabs/envctl/commit/8ada2fcdb6c9a95486ac750f70afb60a9c936659))
* **config:** add backend resolution methods to Config ([40d928c](https://github.com/SentioLabs/envctl/commit/40d928c6884f4b01355525c06c2bea4802a22b90))
* **config:** restructure types for per-environment backend selection ([82fc851](https://github.com/SentioLabs/envctl/commit/82fc8516ca0d12eaf570cd22bb96faf187bbd723))
* **secrets:** update factory to use per-environment backend resolution ([77b2d0d](https://github.com/SentioLabs/envctl/commit/77b2d0daf73bc8b49bd7d948069832c763a6de80))

## [0.2.0](https://github.com/SentioLabs/envctl/compare/v0.1.1...v0.2.0) (2026-03-03)


### Features

* add 1Password as alternative secrets backend ([f523335](https://github.com/SentioLabs/envctl/commit/f5233352490c1c62aa070aca688aec84e3b0eb40))
* add unit tests and remove CI/CD detection code ([09186ae](https://github.com/SentioLabs/envctl/commit/09186aea854054deb64ae36190ffa8a264a9dba2))
* **cmd:** add self update command ([811a3d2](https://github.com/SentioLabs/envctl/commit/811a3d28b76604b500f9825859ab87d28f850bec))
* **onepassword:** update JSON struct tags to omitzero for slice and struct-pointer fields ([3e5dc11](https://github.com/SentioLabs/envctl/commit/3e5dc119c7450bc03981ad2330aa67ce1988497f))
* **release:** replace manual release workflow with release-please + GoReleaser ([9aa99f3](https://github.com/SentioLabs/envctl/commit/9aa99f3724007cab793bb4048a5a25c19b18238e))
* **scripts:** add install script for self-update ([dbd5a1b](https://github.com/SentioLabs/envctl/commit/dbd5a1b7ed430f1a46224d250d81e5be5542ddf7))


### Bug Fixes

* lint issues in new test files ([226cde3](https://github.com/SentioLabs/envctl/commit/226cde3230e859a790034f6fc737c2a3c80d3fd7))
* removing beads ([75dd453](https://github.com/SentioLabs/envctl/commit/75dd453268913c114425a748d399ba08dc2a680e))
* removing beads p2 ([bb81068](https://github.com/SentioLabs/envctl/commit/bb810685beca3ce2b584366f7100451ede78bfdb))
* resolve lint issues from Go 1.26 upgrade and golangci-lint v2 ([7678aeb](https://github.com/SentioLabs/envctl/commit/7678aebdbe41fa4eca329bd53315f8dd8de4985d))


### Refactoring

* **aws:** replace errors.As with errors.AsType for type-safe error handling ([d67e1a0](https://github.com/SentioLabs/envctl/commit/d67e1a03d85d02e5ec9ede1d2c6c548f52883972))
* **env:** use t.Context() instead of context.Background() in tests ([5b1c56b](https://github.com/SentioLabs/envctl/commit/5b1c56b7c76f8d1a18334832c527b75c73b6557c))
* replace custom contains() with slices.Contains and sort.Slice with slices.SortFunc ([bd58371](https://github.com/SentioLabs/envctl/commit/bd58371943047818ef294ed11b485565c7f1e883))
* **runner:** replace isExitError helper with errors.AsType ([cc8bf7f](https://github.com/SentioLabs/envctl/commit/cc8bf7fc1483d76122cf943992447ce7b857a603))
* **test:** replace boolPtr helpers with Go 1.26 new() builtin ([cd1334d](https://github.com/SentioLabs/envctl/commit/cd1334d3d9da885c39fb6125715c63365ac99815))

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
