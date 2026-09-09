# Compatibility entry points. Tool versions are managed by mise.toml.
.PHONY: build test test-coverage lint install clean tidy fmt check release-snapshot

# Preserve make VERSION=... GIT_COMMIT=... BUILD_DATE=... overrides.
TASK_VARS = $(foreach var,VERSION GIT_COMMIT BUILD_DATE,$(if $(filter undefined,$(origin $(var))),,$(var)="$($(var))"))

build test test-coverage lint install clean tidy fmt check release-snapshot:
	task $@ $(TASK_VARS)
