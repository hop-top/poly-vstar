# vstar developer build targets.
# Run from the repo root. Requires GNU make.
#
# The Go module lives in go/. Every go(1) target runs the toolchain
# against it via `go -C go`, which changes directory before anything
# else is parsed -- no subshell, and $(CURDIR)-relative paths in other
# targets keep working. Tools that are not the go driver (golangci-lint,
# gofumpt) read their config from the working directory instead and so
# need a real `cd go &&`.

COVER_PROFILE := coverage.out
COVER_HTML    := coverage.html

MARKDOWNLINT := markdownlint-cli2
SPEC_MD_GLOBS := spec/**/*.md \#spec/v*/CHANGELOG.md

.PHONY: help build test vet fmt fmt-check lint lint-spec cover tidy fixtures-verify registry-gen registry-check test-parity ci ci-go

help: ## Show available targets
	@echo "vstar targets:"
	@echo "  build            Compile all packages"
	@echo "  test             Run all tests with race detector"
	@echo "  vet              Run go vet"
	@echo "  fmt              Format with gofumpt (idempotent)"
	@echo "  fmt-check        Verify formatting without writing"
	@echo "  lint             Run golangci-lint"
	@echo "  lint-spec        Lint spec markdown via markdownlint-cli2"
	@echo "  cover            Produce coverage profile + HTML report (under go/)"
	@echo "  tidy             Run go mod tidy"
	@echo "  fixtures-verify  Regenerate fixtures + fail on drift"
	@echo "  registry-gen     Render language constants + docs from spec/registry/"
	@echo "  registry-check   Verify generated files match spec/registry/"
	@echo "  test-parity      Diff every language emitter against the Go reference"
	@echo "  ci               Full cross-language gate: registry-check + fixtures-verify"
	@echo "                   + ci-go + ci-ts + ci-py + ci-rs + ci-php + test-parity"
	@echo "                   (needs every toolchain; use ci-LANG for one language)"
	@echo ""
	@echo "Per-language gates (LANG = go | ts | py | rs | php):"
	@echo "  ci-LANG          One language's slice of 'ci' -- the fast shortcut"
	@echo "  lint-LANG        Lint one port (ts | py | rs | php)"
	@echo "  test-LANG        Test one port"
	@echo "  build-LANG       Build one port"
	@echo "  package-LANG     Produce one port's package artifact"

build: ## Compile all packages
	go -C go build ./...

test: ## Run all tests with race detector
	go -C go test -race ./...

vet: ## Run go vet
	go -C go vet ./...

fmt: ## Format with gofumpt (gofmt + extras); idempotent
	cd go && gofumpt -w .

fmt-check: ## Verify formatting without writing
	@diff_out=$$(cd go && gofumpt -l .); \
	if [ -n "$$diff_out" ]; then \
		echo "gofumpt: files need formatting (paths relative to go/):" >&2; \
		echo "$$diff_out" >&2; \
		exit 1; \
	fi

# golangci-lint resolves .golangci.yml from its working directory, and
# that config lives only in go/ -- running it from the root fails with
# `can't load config: unsupported version of the configuration: ""`.
lint: ## Run golangci-lint
	cd go && mise exec -- golangci-lint run ./...

# The spec tree carries its own markdownlint config at
# spec/.markdownlint-cli2.jsonc; markdownlint-cli2 resolves config from
# the working directory, so the target runs from spec/ and the globs
# are relative to it.
lint-spec: ## Lint spec markdown via markdownlint-cli2
	@command -v $(MARKDOWNLINT) >/dev/null 2>&1 || { \
		echo "error: $(MARKDOWNLINT) not found" >&2; \
		echo "install: npm install --global markdownlint-cli2" >&2; \
		exit 1; \
	}
	cd spec && $(MARKDOWNLINT) '**/*.md' '#v*/CHANGELOG.md'

# -coverprofile is resolved relative to the module directory, so the
# profile and the HTML report land at go/coverage.out and go/coverage.html.
cover: ## Produce coverage profile + HTML report
	go -C go test -race -coverprofile=$(COVER_PROFILE) -covermode=atomic ./...
	go -C go tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)

tidy: ## Run go mod tidy
	go -C go mod tidy

# The corpus is authored at spec/v0.1/conformance and mirrored into
# go/testdata, so both trees are diffed: a hand-edit to either side is
# drift. spec/behavior is the language-agnostic behavior fixture tree
# the same run regenerates. FIXTURE_PATHS is the whole generated
# surface.
FIXTURE_PATHS := spec/v0.1/conformance spec/behavior go/testdata go/codec/rfc5545/testdata/fuzz go/codec/rfc6350/testdata/fuzz

fixtures-verify: ## Regenerate spec corpus canonical/hash + mirror into go/testdata; fail on drift
	go -C go run ./cmd/fixtures-verify
	@if ! git diff --quiet -- $(FIXTURE_PATHS); then \
		echo "fixtures-verify: drift detected against committed corpus." >&2; \
		echo "Run 'make fixtures-verify' locally and commit the regenerated files," >&2; \
		echo "or revert the implementation change that caused the drift." >&2; \
		git --no-pager diff --stat -- $(FIXTURE_PATHS) >&2; \
		exit 1; \
	fi
	@untracked=$$(git ls-files --others --exclude-standard -- $(FIXTURE_PATHS)); \
	if [ -n "$$untracked" ]; then \
		echo "fixtures-verify: untracked files in the generated corpus:" >&2; \
		echo "$$untracked" >&2; \
		exit 1; \
	fi

# `ci` is the full matrix: the shared gates, then every language.
#
# It was Go-only, which meant a contributor ran it, saw green, and had
# checked one of five implementations -- the four ports went
# unverified locally and only failed later on CI. The shared gates run
# first because registry-check and fixtures-verify regenerate the
# tables and corpus every port reads: a drift there explains the port
# failures that would otherwise follow it, so reporting it first is
# the more useful diagnostic.
#
# test-parity runs last. It is the only target that needs all five
# toolchains at once, and a port that cannot even build its own suite
# will not produce a comparable emitter document either -- so its own
# ci-<lang> failing first names the cause more precisely than a
# parity harness error would.
#
# Running this needs every toolchain installed (Go, pnpm, uv, cargo,
# composer/php). For a single language, use the ci-<lang> shortcut:
#
#     make ci-go     make ci-ts    make ci-py
#     make ci-rs     make ci-php
#
# Each is exactly this aggregate's slice for that language, so a green
# shortcut means that language's leg of `ci` is green too.
ci: registry-check fixtures-verify ci-go ci-ts ci-py ci-rs ci-php test-parity ## Full cross-language CI gate (all five languages + parity)

# The Go reference's own leg. registry-check and fixtures-verify are
# deliberately NOT here: they are repo-wide gates that happen to be
# implemented in Go, and `ci` runs them once for every language rather
# than once per language.
ci-go: vet fmt-check lint cover ## Go reference gate only (vet + fmt-check + lint + cover)

# spec/registry/*.json holds the tables every language port must carry
# identically (diagnostic codes, the RFC property allow-list, extension
# scopes, value vocabularies). tools/registry/gen.py renders them into
# per-language constants and into docs/validate-codes.md, so a table is
# authored once and never retyped per port.
PYTHON := python3
REGISTRY_GEN := $(PYTHON) tools/registry/gen.py

registry-gen: ## Render language constants + docs from spec/registry/
	$(REGISTRY_GEN) gen

registry-check: ## Verify the generated files match spec/registry/; fail on drift
	$(REGISTRY_GEN) gen --check

# tools/parity/ is the cross-language parity harness. Every language
# port ships an emitter that reads spec/ and prints one JSON document;
# the orchestrator diffs each enabled language against the Go reference
# and fails on any difference. Standard-library Python only, so it runs
# on a bare runner before any port's toolchain is installed.
# tools/parity/README.md is the normative emitter contract.
test-parity: ## Diff every enabled language emitter against the Go reference
	$(PYTHON) tools/parity/parity.py
# --- Port scaffolds -------------------------------------------------
#
# The four port trees (ts/ py/ rs/ php/) each carry their own manifest,
# lockfile and toolchain. Verbs follow poly-cite: lint-/test-/build-/
# package-<lang>.
#
# Each port also has a ci-<lang> aggregate (lint + test + build, the
# same three steps its .github/workflows/ci-<lang>.yml runs), and `ci`
# above runs all five. The Go-only `ci` this replaced let a contributor
# see green having checked one implementation of five.
#
# ci-<lang> is also the shortcut when you are working in one tree: it
# is that language's slice of `ci` exactly, so it needs only that
# language's toolchain, and green means that leg of `ci` is green.
#
# Every target runs from the repo root via `cd <dir> &&`, because each
# tool resolves its config from the working directory.

PNPM     ?= pnpm
UV       ?= uv
CARGO    ?= cargo
COMPOSER ?= composer

.PHONY: \
	lint-ts test-ts build-ts package-ts ci-ts \
	lint-py test-py build-py package-py ci-py \
	lint-rs test-rs build-rs package-rs ci-rs \
	lint-php test-php build-php package-php ci-php

# TypeScript. `pnpm install --ignore-scripts` is the gate's first step in
# every target: the tools live in ts/node_modules, so a target invoked on
# a fresh checkout would otherwise fail on a missing binary rather than a
# real finding.
lint-ts: ## Lint the TypeScript port (eslint)
	cd ts && $(PNPM) install --ignore-scripts && $(PNPM) lint

test-ts: ## Run the TypeScript port's tests (vitest)
	cd ts && $(PNPM) install --ignore-scripts && $(PNPM) test

build-ts: ## Build the TypeScript port (tsc)
	cd ts && $(PNPM) run ci:build

package-ts: build-ts ## Pack the TypeScript port into a tarball
	cd ts && $(PNPM) pack

ci-ts: lint-ts test-ts build-ts ## TypeScript port gate (lint + test + build)

# Python. uv owns the venv and the lockfile; `uv run` syncs before it
# runs, so no separate install target is needed.
lint-py: ## Lint the Python port (ruff + mypy)
	cd py && $(UV) run ruff check .
	cd py && $(UV) run ruff format --check .
	cd py && $(UV) run mypy

test-py: ## Run the Python port's tests (pytest)
	cd py && $(UV) run pytest

build-py: ## Build the Python port's sdist + wheel
	cd py && $(UV) build

package-py: build-py ## Build the Python port's distributions

ci-py: lint-py test-py build-py ## Python port gate (lint + test + build)

# Rust. `cargo fmt` skips src/generated/ via #[rustfmt::skip] on the
# module declaration; see rs/src/generated/mod.rs.
lint-rs: ## Lint the Rust port (rustfmt + clippy)
	cd rs && $(CARGO) fmt --check
	cd rs && $(CARGO) clippy --all-targets --all-features -- -D warnings

test-rs: ## Run the Rust port's tests
	cd rs && $(CARGO) test

build-rs: ## Build the Rust port
	cd rs && $(CARGO) build

package-rs: ## Validate the Rust port's package contents
	cd rs && $(CARGO) package --allow-dirty --no-verify

ci-rs: lint-rs test-rs build-rs ## Rust port gate (lint + test + build)

# PHP. Composer installs into php/vendor; the tools are all dev deps.
lint-php: ## Lint the PHP port (phpstan + php-cs-fixer)
	cd php && $(COMPOSER) install --no-interaction --no-progress
	cd php && ./vendor/bin/phpstan analyse --no-progress
	cd php && ./vendor/bin/php-cs-fixer check

test-php: ## Run the PHP port's tests (phpunit)
	cd php && $(COMPOSER) install --no-interaction --no-progress
	cd php && ./vendor/bin/phpunit

build-php: ## Validate the PHP port's package metadata
	cd php && $(COMPOSER) validate --strict

package-php: build-php ## Validate the PHP port's package metadata

ci-php: lint-php test-php build-php ## PHP port gate (lint + test + build)
