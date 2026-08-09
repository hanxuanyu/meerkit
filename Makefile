SHELL := /bin/sh
.DEFAULT_GOAL := help

GO ?= go
NPM ?= npm

WEB_DIR ?= web
DOCS_DIR ?= docs
PLUGIN_OUTPUT ?= dist/plugins
RELEASE_OUTPUT ?= dist/releases
TARGETS ?= $(shell $(GO) env GOOS)/$(shell $(GO) env GOARCH)
PLUGIN ?=
KEY_PREFIX ?= keys/meerkit-official
SIGN_KEY ?= $(MEERKIT_PLUGIN_SIGN_KEY)
KEY_ID ?= $(MEERKIT_PLUGIN_KEY_ID)
VERSION ?= $(MEERKIT_VERSION)
BACKEND_ARGS ?=
FRONTEND_ARGS ?=

.PHONY: help deps dev dev-backend dev-frontend frontend-build prepare-frontend-assets \
	docs-dev docs-build docs-preview \
	package-plugins package-plugin package-release plugins release generate-key keygen clean reset

help:
	@printf '%s\n' \
		'Meerkit development and release commands:' \
		'' \
		'  make deps               Install frontend and Go dependencies' \
		'  make dev                Start the backend and Vite frontend' \
		'  make dev-backend        Start only the Go backend' \
		'  make dev-frontend       Start only the Vite frontend' \
		'  make frontend-build     Build frontend production assets' \
		'  make docs-dev           Start the VitePress documentation site' \
		'  make docs-build         Build the documentation site' \
		'  make docs-preview       Preview the built documentation site' \
		'  make package-plugins    Package all publishable plugins' \
		'  make package-plugin     Package PLUGIN=<plugin directory>' \
		'  make package-release    Build complete production archives' \
		'  make generate-key       Generate an Ed25519 signing key pair' \
		'  make clean              Remove generated build artifacts' \
		'  make reset              Remove build artifacts and local runtime state' \
		'' \
		'Common overrides:' \
		'  TARGETS=linux/amd64,linux/arm64  SIGN_KEY=path  KEY_ID=name' \
		'  VERSION=v0.1.0  KEY_PREFIX=keys/meerkit-official' \
		'  BACKEND_ARGS="--config config.yaml"  FRONTEND_ARGS="--host 0.0.0.0"'

deps:
	$(GO) mod download
	$(NPM) --prefix $(WEB_DIR) ci
	$(NPM) --prefix $(DOCS_DIR) ci

prepare-frontend-assets:
	@if [ ! -f "$(WEB_DIR)/dist/index.html" ]; then \
		echo "Frontend assets are missing; building them for the Go embed..."; \
		$(NPM) --prefix "$(WEB_DIR)" run build; \
	fi

dev: prepare-frontend-assets
	@set -eu; \
	backend_pid=''; \
	frontend_pid=''; \
	cleanup() { \
		trap - EXIT HUP INT TERM; \
		for pid in "$$backend_pid" "$$frontend_pid"; do \
			[ -z "$$pid" ] || kill "$$pid" 2>/dev/null || true; \
		done; \
		for pid in "$$backend_pid" "$$frontend_pid"; do \
			[ -z "$$pid" ] || wait "$$pid" 2>/dev/null || true; \
		done; \
	}; \
	trap cleanup EXIT; \
	trap 'exit 130' HUP INT TERM; \
	$(MAKE) --no-print-directory dev-backend & backend_pid=$$!; \
	$(MAKE) --no-print-directory dev-frontend & frontend_pid=$$!; \
	while kill -0 "$$backend_pid" 2>/dev/null && kill -0 "$$frontend_pid" 2>/dev/null; do \
		sleep 1; \
	done; \
	status=0; \
	if ! kill -0 "$$backend_pid" 2>/dev/null; then \
		wait "$$backend_pid" || status=$$?; \
	else \
		wait "$$frontend_pid" || status=$$?; \
	fi; \
	exit "$$status"

dev-backend: prepare-frontend-assets
	$(GO) run . serve $(BACKEND_ARGS)

dev-frontend:
	$(NPM) --prefix $(WEB_DIR) run dev -- $(FRONTEND_ARGS)

frontend-build:
	$(NPM) --prefix $(WEB_DIR) run build

docs-dev:
	$(NPM) --prefix $(DOCS_DIR) run dev

docs-build:
	$(NPM) --prefix $(DOCS_DIR) run build

docs-preview:
	$(NPM) --prefix $(DOCS_DIR) run preview

package-plugins:
	@MEERKIT_PLUGIN_SIGN_KEY="$(SIGN_KEY)" \
		MEERKIT_PLUGIN_KEY_ID="$(KEY_ID)" \
		./scripts/package-plugins.sh "$(PLUGIN_OUTPUT)" "$(TARGETS)"

package-plugin:
	@set -eu; \
	if [ -z "$(PLUGIN)" ]; then \
		echo 'PLUGIN is required, for example: make package-plugin PLUGIN=plugins/http' >&2; \
		exit 2; \
	fi; \
	if [ -n "$(SIGN_KEY)" ] && [ -n "$(KEY_ID)" ]; then \
		./scripts/package-plugins.sh --plugin "$(PLUGIN)" --output "$(PLUGIN_OUTPUT)" --targets "$(TARGETS)" --sign-key "$(SIGN_KEY)" --key-id "$(KEY_ID)"; \
	elif [ -z "$(SIGN_KEY)" ] && [ -z "$(KEY_ID)" ]; then \
		./scripts/package-plugins.sh --plugin "$(PLUGIN)" --output "$(PLUGIN_OUTPUT)" --targets "$(TARGETS)"; \
	else \
		echo 'SIGN_KEY and KEY_ID must be set together' >&2; \
		exit 2; \
	fi

package-release:
	@MEERKIT_VERSION="$(VERSION)" \
		MEERKIT_PLUGIN_SIGN_KEY="$(SIGN_KEY)" \
		MEERKIT_PLUGIN_KEY_ID="$(KEY_ID)" \
		./scripts/package.sh "$(RELEASE_OUTPUT)" "$(TARGETS)"

generate-key:
	./scripts/package-plugins.sh --generate-key "$(KEY_PREFIX)"

clean:
	rm -rf ./dist ./web/dist ./docs/.vitepress/.temp ./docs/.vitepress/cache ./docs/.vitepress/dist ./.gocache
	rm -f ./meerkit ./meerkit.exe

reset: clean
	rm -rf ./data ./logs
	rm -f ./config.yaml ./*.db ./*.db-shm ./*.db-wal
	@printf '%s\n' 'Project reset complete; keys/ and installed dependencies were preserved.'

plugins: package-plugins
release: package-release
keygen: generate-key
