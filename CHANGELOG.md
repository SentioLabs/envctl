# Changelog

All notable changes to envctl will be documented in this file.

## [0.6.0](https://github.com/SentioLabs/envctl/compare/v0.5.0...v0.6.0) (2026-03-19)


### Features

* **tui:** add v to reveal/conceal secret values in field editor ([49e7709](https://github.com/SentioLabs/envctl/commit/49e7709bbab588f23e303ed2e1f8d38a952fc2e8))


### Bug Fixes

* **onepassword:** batch field changes into single op item edit call ([5ef5c6e](https://github.com/SentioLabs/envctl/commit/5ef5c6e4dd357fe3e9fa8afe62a78fbe82f1de0d))
* **onepassword:** use field ID to disambiguate duplicate labels ([a898818](https://github.com/SentioLabs/envctl/commit/a8988184351b82455249067eb66d7935bd957ccf))
* **tui:** dynamically align field editor columns based on content width ([4a41c3a](https://github.com/SentioLabs/envctl/commit/4a41c3a35fe82b14c5a6f4cf33f9c16ec9f4f68c))

## [0.5.0](https://github.com/SentioLabs/envctl/compare/v0.4.0...v0.5.0) (2026-03-19)


### Features

* **aws:** implement Editor for TUI secret editing ([11ee95d](https://github.com/SentioLabs/envctl/commit/11ee95d3407fba03cf0d86ca5396e7f3c0e46a48))
* **cmd:** add config-driven mode and --browse flag to envctl edit ([28e69b2](https://github.com/SentioLabs/envctl/commit/28e69b2d72911b18c430957295b243b5b18461e7))
* **cmd:** add envctl edit command for interactive secret editing ([55b1e41](https://github.com/SentioLabs/envctl/commit/55b1e411b7cd9fd39a8e988fab2210563d27c3c8))
* **onepassword:** implement Editor and FieldTypeEditor for TUI editor ([24be4b0](https://github.com/SentioLabs/envctl/commit/24be4b09b7f3c27aa1beb7a9fdf7554876afd09d))
* **secrets:** add Editor interface, types, and factory for TUI editor ([4e0f07b](https://github.com/SentioLabs/envctl/commit/4e0f07b29d2bf3266aad327db6b968789dab9211))
* **tui:** add /filter to field editor for searching fields ([ac53ce0](https://github.com/SentioLabs/envctl/commit/ac53ce0be94342a744db25f8a0f5ab74b5d283fe))
* **tui:** add app picker and env picker screens for config mode ([16b8699](https://github.com/SentioLabs/envctl/commit/16b869967adc5537a8f62dee653e2fe59fccaff8))
* **tui:** add app shell with screen routing and editor integration ([30ae1f1](https://github.com/SentioLabs/envctl/commit/30ae1f145c67bcadc82ed223b39d4a5cd2a40ac8))
* **tui:** add config-driven screen routing with EditorFactory ([3803874](https://github.com/SentioLabs/envctl/commit/3803874b458889c359c2caedbe8b9d13d4f0b643))
* **tui:** add ConfigContext type for config-driven editor mode ([05bf416](https://github.com/SentioLabs/envctl/commit/05bf416af8c3c917fc86cfb3179920fe4e0242d2))
* **tui:** add enter-to-edit and s-to-save keybindings in field editor ([789986f](https://github.com/SentioLabs/envctl/commit/789986f2cf879aeaf5159020055f5ea51062b69b))
* **tui:** add field editor screen with CRUD operations ([1d78c17](https://github.com/SentioLabs/envctl/commit/1d78c17056b562e5a32614da8961307a23dfc33e))
* **tui:** add item list screen with create-item support ([bab9285](https://github.com/SentioLabs/envctl/commit/bab9285d0011bcf3015c57382b26ee20412754f5))
* **tui:** add Lip Gloss styles and key binding definitions ([9ca6f67](https://github.com/SentioLabs/envctl/commit/9ca6f67b77452b5626f4bc7f694bacf3dd910a16))
* **tui:** add secret list screen with backend badges ([a98673b](https://github.com/SentioLabs/envctl/commit/a98673b77cac08a2552af9d11f5c2f45e9140343))
* **tui:** add vault picker screen with filtering ([3e0fbc3](https://github.com/SentioLabs/envctl/commit/3e0fbc308fd0ffba25327113e0e0e11cbf3050d5))
* **tui:** add viewport scrolling and Ctrl+U/D page navigation ([3991ad8](https://github.com/SentioLabs/envctl/commit/3991ad84678b51cb5e6fa8588c0f4357c2564c6b))
* **tui:** add visual golden file tests for field editor ([20da82f](https://github.com/SentioLabs/envctl/commit/20da82ffb5d947d0bea48ae1039d3512b7c595b4))
* **tui:** explicit save model — esc/q discard with confirmation ([9aa398b](https://github.com/SentioLabs/envctl/commit/9aa398bb8be049a795477e7f1692f743ed959120))


### Bug Fixes

* **onepassword:** filter out structural fields (notesPlain) from editor ([a85f75b](https://github.com/SentioLabs/envctl/commit/a85f75b1ba313834147470e2387955de455547e0))
* **onepassword:** use section-qualified field refs for duplicate labels ([ae49484](https://github.com/SentioLabs/envctl/commit/ae49484df4d6fe9793a52507055dd6ec44d547cc))
* remove duplicate vault.go from parallel merge ([dfdce47](https://github.com/SentioLabs/envctl/commit/dfdce4731709d701b29a3b78e3518a942a104fd6))
* **secrets:** suppress nilnil lint in test mock ([a6402e8](https://github.com/SentioLabs/envctl/commit/a6402e8723a924f452f9f888477cf18973a2ace6))
* **tui:** handle key input in error state to prevent hung process ([774940a](https://github.com/SentioLabs/envctl/commit/774940abeaa42a76c73cdf7cf6c3266c4f8629f1))
* **tui:** prevent nil pointer panic in config-driven field editor ([6a17270](https://github.com/SentioLabs/envctl/commit/6a1727081e8ed82bd869361f3075d1d5bab39574))

## [0.4.0](https://github.com/SentioLabs/envctl/compare/v0.3.1...v0.4.0) (2026-03-06)


### Features

* **config,env:** env-filtered includes, cross-backend resolution, and fixture migration ([ddab1f9](https://github.com/SentioLabs/envctl/commit/ddab1f95854b297fe0df5d4bce9f8bfdd5cc5120))
* **config,env:** env-keyed includes with backend qualifiers and ClientFactory ([930001e](https://github.com/SentioLabs/envctl/commit/930001e13ccdcc170258cf84804b0feb674483fa))
* **config,env:** replace include maps with ordered source lists per environment ([dcb2e6a](https://github.com/SentioLabs/envctl/commit/dcb2e6ad062ed4100c6f8864bf06e398e80bd007))

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
