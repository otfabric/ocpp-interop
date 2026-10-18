# Self-documented Makefile (https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html)
# Run 'make' or 'make help' to list targets.
#
# Pre-push gate:  make ci
#
# Phase 1: validation only. The image, smoke, interop and publish targets of
# the sibling repositories (iec104-interop, modbus-interop) are added with the
# first adapter (PLAN.md, Phase 4).

.DEFAULT_GOAL := help

REGISTRY  ?= ghcr.io/otfabric
VERSION   ?= dev
PLATFORMS ?= linux/amd64,linux/arm64

# Registry of the images the tooling itself runs (shellcheck, Python). The
# workflows set TOOLS_REGISTRY=mirror.gcr.io, because anonymous pulls from
# Docker Hub are rate limited per runner address.
TOOLS_REGISTRY ?= docker.io

# Repeats a command that failed on a rate limit, a 5xx or a timeout, and only
# then. See scripts/retry.sh.
RETRY := scripts/retry.sh

PYTHON ?= python3

.PHONY: help all ci validate fixtures versions schemas lint clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_%-]+:.*?## / {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: validate ## Validate fixtures, schemas and versions.yaml

ci: validate lint ## Full local CI
	@echo ""
	@echo "All local CI checks passed."

# ── Validation ───────────────────────────────────────────────────────────────

validate: versions schemas fixtures ## Validate versions.yaml, the schemas and every fixture

versions: ## Check that versions.yaml agrees with the Dockerfiles
	@scripts/check-versions.sh

# Runs the Python checks locally when jsonschema is installed, otherwise in a
# python container, so that 'make validate' never silently skips the schema.
define with_jsonschema
	@if $(PYTHON) -c 'import jsonschema' 2>/dev/null; then \
		$(PYTHON) $(1); \
	else \
		echo "(jsonschema not installed locally: running in a container)"; \
		$(RETRY) docker run --rm -v "$(CURDIR):/work:ro" -w /work $(TOOLS_REGISTRY)/library/python:3.12-slim \
			sh -c 'pip install -q --root-user-action=ignore jsonschema && python $(1)'; \
	fi
endef

schemas: ## Check that every JSON schema is a valid draft 2020-12 schema
	@echo "Checking schemas..."
	$(call with_jsonschema,scripts/check-schemas.py)

fixtures: ## Validate every fixture against the schema and the cross-field rules
	@echo "Validating fixtures..."
	$(call with_jsonschema,scripts/validate-fixtures.py --require-schema)

lint: ## Shellcheck the scripts and byte-compile the Python ones
	@if command -v shellcheck >/dev/null; then \
		shellcheck --severity=warning scripts/*.sh; \
	else \
		$(RETRY) docker run --rm -v "$(CURDIR):/mnt:ro" -w /mnt $(TOOLS_REGISTRY)/koalaman/shellcheck:stable --severity=warning scripts/*.sh; \
	fi
	@$(PYTHON) -m py_compile scripts/*.py && rm -rf scripts/__pycache__
	@echo "lint ok"

clean: ## Remove reports
	@rm -rf reports
