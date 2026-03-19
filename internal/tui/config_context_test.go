package tui

import (
	"testing"

	"github.com/sentiolabs/envctl/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigContext_NilConfig(t *testing.T) {
	ctx := NewConfigContext(nil)
	assert.Nil(t, ctx)
}

func TestNewConfigContext_EmptyConfig(t *testing.T) {
	cfg := &config.Config{}
	ctx := NewConfigContext(cfg)
	assert.Nil(t, ctx)
}

func TestNewConfigContext_MultiApp(t *testing.T) {
	cfg := &config.Config{
		DefaultApplication: "api",
		DefaultEnvironment: "dev",
		DefaultBackend:     config.BackendAWS,
		AWS:                &config.AWSConfig{Region: "us-east-1"},
		OnePass:            &config.OnePassConfig{Vault: "Dev"},
		Applications: map[string]*config.Application{
			"api": {
				Environments: map[string]config.Environment{
					"dev": config.NewEnvironment(
						config.IncludeEntry{Secret: "api/dev/secrets", Backend: config.BackendAWS},
						config.IncludeEntry{Secret: "api/dev/extra", Backend: config.Backend1Pass},
					),
					"prod": config.NewEnvironment(
						config.IncludeEntry{Secret: "api/prod/secrets"},
					),
				},
			},
			"web": {
				Environments: map[string]config.Environment{
					"dev": config.NewEnvironment(
						config.IncludeEntry{Secret: "web/dev/secrets", Backend: config.Backend1Pass},
					),
				},
			},
		},
	}

	ctx := NewConfigContext(cfg)
	require.NotNil(t, ctx)

	// Apps should be sorted
	assert.Equal(t, []string{"api", "web"}, ctx.Apps)

	// Envs should be sorted per app
	assert.Equal(t, []string{"dev", "prod"}, ctx.Envs["api"])
	assert.Equal(t, []string{"dev"}, ctx.Envs["web"])

	// Sources for api/dev: two sources with explicit backends
	apiDevSources := ctx.Sources["api/dev"]
	require.Len(t, apiDevSources, 2)
	assert.Equal(t, Source{Name: "api/dev/secrets", Backend: config.BackendAWS}, apiDevSources[0])
	assert.Equal(t, Source{Name: "api/dev/extra", Backend: config.Backend1Pass}, apiDevSources[1])

	// Sources for api/prod: no explicit backend, should resolve from config default
	apiProdSources := ctx.Sources["api/prod"]
	require.Len(t, apiProdSources, 1)
	assert.Equal(t, Source{Name: "api/prod/secrets", Backend: config.BackendAWS}, apiProdSources[0])

	// Sources for web/dev
	webDevSources := ctx.Sources["web/dev"]
	require.Len(t, webDevSources, 1)
	assert.Equal(t, Source{Name: "web/dev/secrets", Backend: config.Backend1Pass}, webDevSources[0])

	// Defaults
	assert.Equal(t, "api", ctx.DefaultApp)
	assert.Equal(t, "dev", ctx.DefaultEnv)
}

func TestNewConfigContext_BackendResolution(t *testing.T) {
	// Source with explicit backend field takes precedence
	cfg := &config.Config{
		DefaultBackend: config.BackendAWS,
		AWS:            &config.AWSConfig{Region: "us-east-1"},
		OnePass:        &config.OnePassConfig{Vault: "Dev"},
		Applications: map[string]*config.Application{
			"svc": {
				Environments: map[string]config.Environment{
					"local": config.NewEnvironment(
						config.IncludeEntry{Secret: "svc/local", Backend: config.Backend1Pass},
					),
				},
			},
		},
	}

	ctx := NewConfigContext(cfg)
	require.NotNil(t, ctx)

	sources := ctx.Sources["svc/local"]
	require.Len(t, sources, 1)
	assert.Equal(t, config.Backend1Pass, sources[0].Backend)
}

func TestNewConfigContext_SingleApp(t *testing.T) {
	cfg := &config.Config{
		Applications: map[string]*config.Application{
			"myapp": {
				Environments: map[string]config.Environment{
					"staging": config.NewEnvironment(
						config.IncludeEntry{Secret: "myapp/staging", AWS: &config.AWSConfig{Region: "eu-west-1"}},
					),
				},
			},
		},
	}

	ctx := NewConfigContext(cfg)
	require.NotNil(t, ctx)

	assert.Equal(t, []string{"myapp"}, ctx.Apps)
	assert.Equal(t, []string{"staging"}, ctx.Envs["myapp"])

	sources := ctx.Sources["myapp/staging"]
	require.Len(t, sources, 1)
	assert.Equal(t, "myapp/staging", sources[0].Name)
	assert.Equal(t, config.BackendAWS, sources[0].Backend)
}

func TestNewConfigContext_LegacyMode(t *testing.T) {
	cfg := &config.Config{
		DefaultEnvironment: "dev",
		AWS:                &config.AWSConfig{Region: "us-east-1"},
		Environments: map[string]config.Environment{
			"dev": config.NewEnvironment(
				config.IncludeEntry{Secret: "myapp/dev"},
			),
			"prod": config.NewEnvironment(
				config.IncludeEntry{Secret: "myapp/prod"},
			),
		},
	}

	ctx := NewConfigContext(cfg)
	require.NotNil(t, ctx)

	// Legacy mode uses empty string as app name
	assert.Equal(t, []string{""}, ctx.Apps)

	// Envs sorted under empty app key
	assert.Equal(t, []string{"dev", "prod"}, ctx.Envs[""])

	// Sources keyed as "/env" (empty app + "/" + env)
	devSources := ctx.Sources["/dev"]
	require.Len(t, devSources, 1)
	assert.Equal(t, "myapp/dev", devSources[0].Name)
	assert.Equal(t, config.BackendAWS, devSources[0].Backend)

	prodSources := ctx.Sources["/prod"]
	require.Len(t, prodSources, 1)
	assert.Equal(t, "myapp/prod", prodSources[0].Name)

	// Defaults
	assert.Equal(t, "", ctx.DefaultApp)
	assert.Equal(t, "dev", ctx.DefaultEnv)
}
