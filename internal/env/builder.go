// Package env handles environment variable resolution and merging.
package env

import (
	"context"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/sentiolabs/envctl/internal/errors"
	"github.com/sentiolabs/envctl/internal/secrets"
)

// Entry represents a resolved environment variable with its source.
type Entry struct {
	Key    string
	Value  string
	Source string
}

// BuilderOptions configures optional builder behavior for cross-backend includes.
type BuilderOptions struct {
	NoCache bool
	Refresh bool
}

// ResolvedFile is a file sink with its content fetched.
type ResolvedFile struct {
	Sink    config.FileSink
	Secret  string
	Content []byte
}

// Result is the output of BuildWithFiles.
type Result struct {
	Entries []Entry
	Files   []ResolvedFile
}

// Builder builds environment variables from configuration.
type Builder struct {
	secrets    secrets.Client
	config     *config.Config
	appName    string
	envName    string
	app        *config.Application // Resolved application (nil in legacy mode)
	env        *config.Environment // Resolved environment
	includeAll *bool               // CLI override for include_all setting
	noCache    bool
	refresh    bool
	newClient  func(ctx context.Context, opts secrets.Options) (secrets.Client, error)
}

// NewBuilder creates a new environment builder.
// appName can be empty for legacy (non-application) configs.
func NewBuilder(client secrets.Client, cfg *config.Config, appName, envName string) *Builder {
	return &Builder{
		secrets:   client,
		config:    cfg,
		appName:   appName,
		envName:   envName,
		newClient: secrets.NewClient,
	}
}

// WithIncludeAll sets the CLI override for include_all setting.
// Returns the builder for method chaining.
func (b *Builder) WithIncludeAll(val *bool) *Builder {
	b.includeAll = val
	return b
}

// WithOptions sets cross-backend client options (noCache, refresh).
// Returns the builder for method chaining.
func (b *Builder) WithOptions(opts BuilderOptions) *Builder {
	b.noCache = opts.NoCache
	b.refresh = opts.Refresh
	return b
}

// shouldIncludeAll determines if all keys from primary secret should be included.
// Checks CLI override first, then delegates to config precedence.
func (b *Builder) shouldIncludeAll() bool {
	if b.includeAll != nil {
		return *b.includeAll
	}
	return b.config.ShouldIncludeAll(b.app, b.env)
}

// Build resolves all environment variables according to precedence rules.
// Sources that carry a file: block are skipped here and never fetched;
// use BuildWithFiles to resolve them.
func (b *Builder) Build(ctx context.Context, overrides map[string]string) ([]Entry, error) {
	return b.build(ctx, overrides)
}

// BuildWithFiles resolves environment entries and fetches every file sink's
// content. Sink content never appears in Entries.
func (b *Builder) BuildWithFiles(ctx context.Context, overrides map[string]string) (*Result, error) {
	entries, err := b.build(ctx, overrides)
	if err != nil {
		return nil, err
	}
	files, err := b.processFiles(ctx)
	if err != nil {
		return nil, err
	}
	return &Result{Entries: entries, Files: files}, nil
}

// build is the shared environment-variable resolution.
// Sources are processed in order (later sources override earlier ones),
// then mappings, then overrides.
//
//nolint:revive // confusing-naming: build/Build split is intentional; Build and BuildWithFiles both call build
func (b *Builder) build(ctx context.Context, overrides map[string]string) ([]Entry, error) {
	entries := make(map[string]Entry)

	// Resolve environment config based on mode
	if err := b.resolveConfig(); err != nil {
		return nil, err
	}

	includeAll := b.shouldIncludeAll()

	// 1. Process primary source (first entry) using the primary client.
	//    A primary that is a file sink contributes nothing here.
	if len(b.env.Sources) > 0 && b.env.Sources[0].File == nil {
		if err := b.processPrimary(ctx, entries, b.env.Sources[0], includeAll); err != nil {
			return nil, err
		}
	}

	// 2. Process additional sources (later sources override earlier)
	if len(b.env.Sources) > 1 {
		if err := b.processIncludes(ctx, entries, b.env.Sources[1:], includeAll); err != nil {
			return nil, err
		}
	}

	// 3. Apply global mapping entries
	if err := b.processMapping(ctx, entries, b.config.Mapping); err != nil {
		return nil, err
	}

	// 4. Apply app-level mapping entries (if in application mode)
	if b.app != nil && len(b.app.Mapping) > 0 {
		if err := b.processMapping(ctx, entries, b.app.Mapping); err != nil {
			return nil, err
		}
	}

	// 5. Apply overrides
	for key, value := range overrides {
		entries[key] = Entry{
			Key:    key,
			Value:  value,
			Source: "override",
		}
	}

	// Convert map to slice
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}

	return result, nil
}

// resolveConfig determines the environment config based on application mode.
func (b *Builder) resolveConfig() error {
	if b.config.HasApplications() {
		// Application mode
		env, app, err := b.config.GetApplicationEnvironment(b.appName, b.envName)
		if err != nil {
			return err
		}
		b.env = env
		b.app = app
	} else {
		// Legacy mode
		env, err := b.config.GetEnvironment(b.envName)
		if err != nil {
			return err
		}
		b.env = env
	}
	return nil
}

// processPrimary handles the first source entry using the primary client.
// Unlike additional sources, the primary is silently skipped when includeAll
// is false and no explicit key/keys are specified.
func (b *Builder) processPrimary(
	ctx context.Context,
	entries map[string]Entry,
	primary config.IncludeEntry,
	includeAll bool,
) error {
	switch {
	case primary.Key != "":
		value, err := b.secrets.GetSecretKey(ctx, primary.Secret, primary.Key)
		if err != nil {
			return err
		}
		name := primary.Key
		if primary.As != "" {
			name = primary.As
		}
		entries[name] = Entry{Key: name, Value: value, Source: primary.Secret}
	case len(primary.Keys) > 0:
		for _, km := range primary.Keys {
			value, err := b.secrets.GetSecretKey(ctx, primary.Secret, km.Key)
			if err != nil {
				return err
			}
			name := km.Key
			if km.As != "" {
				name = km.As
			}
			entries[name] = Entry{Key: name, Value: value, Source: primary.Secret}
		}
	case includeAll:
		allSecrets, err := b.secrets.GetSecret(ctx, primary.Secret)
		if err != nil {
			return err
		}
		for key, value := range allSecrets {
			entries[key] = Entry{Key: key, Value: value, Source: primary.Secret}
		}
	default:
		// includeAll=false, no explicit keys — skip primary.
		// Mappings may still reference this secret directly.
	}
	return nil
}

// clientForInclude returns the appropriate secrets client for an include entry.
// If the include has a backend qualifier (aws:, 1pass:, or backend:), a new client
// is created for that backend. Otherwise, the primary client is used.
func (b *Builder) clientForInclude(ctx context.Context, inc config.IncludeEntry) (secrets.Client, error) {
	// Promote backend field for non-first sources (first source is promoted at parse time)
	config.PromoteBackend(&inc)

	if inc.AWS == nil && inc.OnePass == nil {
		return b.secrets, nil
	}

	syntheticEnv := config.NewEnvironment(inc)

	return b.newClient(ctx, secrets.Options{
		Config:  b.config,
		Env:     &syntheticEnv,
		NoCache: b.noCache,
		Refresh: b.refresh,
	})
}

// processIncludes processes a list of include entries.
// When includeAll is false, entries without a specific key will error.
//
//nolint:gocognit,revive // Include logic requires nested checks for key presence and includeAll mode
func (b *Builder) processIncludes(
	ctx context.Context,
	entries map[string]Entry,
	includes []config.IncludeEntry,
	includeAll bool,
) error {
	for _, inc := range includes {
		if inc.File != nil {
			continue // resolved by processFiles
		}

		client, err := b.clientForInclude(ctx, inc)
		if err != nil {
			return err
		}

		switch {
		case inc.Key != "":
			// Extract specific key
			value, err := client.GetSecretKey(ctx, inc.Secret, inc.Key)
			if err != nil {
				return err
			}
			name := inc.Key
			if inc.As != "" {
				name = inc.As
			}
			entries[name] = Entry{
				Key:    name,
				Value:  value,
				Source: inc.Secret,
			}
		case len(inc.Keys) > 0:
			// Extract multiple specific keys from the same secret
			for _, km := range inc.Keys {
				value, err := client.GetSecretKey(ctx, inc.Secret, km.Key)
				if err != nil {
					return err
				}
				name := km.Key
				if km.As != "" {
					name = km.As
				}
				entries[name] = Entry{
					Key:    name,
					Value:  value,
					Source: inc.Secret,
				}
			}
		default:
			// Include all keys from secret - requires include_all to be enabled
			if !includeAll {
				return &errors.IncludeAllRequiredError{Secret: inc.Secret}
			}
			incSecrets, err := client.GetSecret(ctx, inc.Secret)
			if err != nil {
				return err
			}
			for key, value := range incSecrets {
				entries[key] = Entry{
					Key:    key,
					Value:  value,
					Source: inc.Secret,
				}
			}
		}
	}
	return nil
}

// processMapping processes mapping entries.
func (b *Builder) processMapping(ctx context.Context, entries map[string]Entry, mapping map[string]string) error {
	for envVar, ref := range mapping {
		secretRef, err := config.ParseSecretRef(ref)
		if err != nil {
			return err
		}
		value, err := b.secrets.GetSecretKey(ctx, secretRef.SecretName, secretRef.KeyName)
		if err != nil {
			return err
		}
		entries[envVar] = Entry{
			Key:    envVar,
			Value:  value,
			Source: "mapping",
		}
	}
	return nil
}

// clientForSource returns the primary client for the first source and the
// include-resolved client for every other source.
func (b *Builder) clientForSource(ctx context.Context, i int, src config.IncludeEntry) (secrets.Client, error) {
	if i == 0 {
		return b.secrets, nil
	}
	return b.clientForInclude(ctx, src)
}

// processFiles fetches content for every source that carries a file: block.
// A keyed sink reads one field. A keyless sink needs a backend that implements
// secrets.RawReader and receives the secret verbatim.
func (b *Builder) processFiles(ctx context.Context) ([]ResolvedFile, error) {
	var files []ResolvedFile
	for i, src := range b.env.Sources {
		if src.File == nil {
			continue
		}
		client, err := b.clientForSource(ctx, i, src)
		if err != nil {
			return nil, err
		}

		var content []byte
		if src.Key != "" {
			value, err := client.GetSecretKey(ctx, src.Secret, src.Key)
			if err != nil {
				return nil, err
			}
			content = []byte(value)
		} else {
			raw, ok := client.(secrets.RawReader)
			if !ok {
				return nil, &errors.FileSinkKeyRequiredError{Secret: src.Secret, Backend: client.Name()}
			}
			content, err = raw.GetSecretRaw(ctx, src.Secret)
			if err != nil {
				return nil, err
			}
		}

		files = append(files, ResolvedFile{Sink: *src.File, Secret: src.Secret, Content: content})
	}
	return files, nil
}

// ToMap converts entries to a simple key-value map.
func ToMap(entries []Entry) map[string]string {
	result := make(map[string]string, len(entries))
	for _, e := range entries {
		result[e.Key] = e.Value
	}
	return result
}
