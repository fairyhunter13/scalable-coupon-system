SHELL := /bin/bash
PATH := $(PWD)/bin:$(PATH)

APP_NAME := scalable-coupon-system
GO := go
GOFLAGS := -trimpath
CGO_ENABLED ?= 0
SOPS_AGE_KEY_FILE ?= $(HOME)/.config/sops/age/keys.txt

# Directories
COVERAGE_DIR := coverage
SECRETS_DIR := secrets

# Plaintext directories (gitignored, decrypted locally)
PLAINTEXT_REQUIREMENTS_DIR := project_requirements

# Helper functions
define log_info
	@echo "==> $(1)"
endef

define check_sops_key
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE). Run: age-keygen -o $(SOPS_AGE_KEY_FILE)" && exit 1)
endef

define check_file_exists
	@[ -f "$(1)" ] || (echo "Error: $(1) not found." && exit 1)
endef

.PHONY: all deps fmt lint vet test cover build docker-build docker-run \
	encrypt-requirements decrypt-requirements help security check

all: fmt lint vet security test

# --- Development Targets ---

deps:
	$(GO) mod download

fmt:
	gofmt -s -w .
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

lint:
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

vet:
	$(GO) vet ./...

security:
	@which gosec >/dev/null 2>&1 || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	@which govulncheck >/dev/null 2>&1 || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	gosec ./...
	govulncheck ./...

check: lint vet security

test:
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -v -race -timeout=180s -coverprofile=$(COVERAGE_DIR)/coverage.out ./...

cover: test
	$(GO) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/api

# --- Docker Targets ---

docker-build:
	docker-compose build

docker-run:
	docker-compose up -d

docker-down:
	docker-compose down -v

# --- Secrets Management (SOPS + age) ---

# Encrypt project_requirements/ -> secrets/project-requirements/ (binary .sops)
encrypt-requirements:
	$(call check_sops_key)
	@mkdir -p $(SECRETS_DIR)/project-requirements
	@set -euo pipefail; \
	if [ ! -d $(PLAINTEXT_REQUIREMENTS_DIR) ]; then \
		echo "$(PLAINTEXT_REQUIREMENTS_DIR) directory not found; nothing to encrypt"; \
		exit 0; \
	fi; \
	find $(PLAINTEXT_REQUIREMENTS_DIR) -type f | while IFS= read -r src; do \
		rel=$${src#$(PLAINTEXT_REQUIREMENTS_DIR)/}; \
		dest_dir="$(SECRETS_DIR)/project-requirements/$$(dirname "$$rel")"; \
		dest_file="$(SECRETS_DIR)/project-requirements/$$rel.sops"; \
		mkdir -p "$$dest_dir"; \
		echo "Encrypting $$src -> $$dest_file"; \
		SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done
	$(call log_info,Requirements encrypted to $(SECRETS_DIR)/project-requirements/)

# Decrypt secrets/project-requirements/ -> project_requirements/ (binary)
decrypt-requirements:
	$(call check_sops_key)
	@mkdir -p $(PLAINTEXT_REQUIREMENTS_DIR)
	@set -euo pipefail; \
	if [ ! -d $(SECRETS_DIR)/project-requirements ]; then \
		echo "$(SECRETS_DIR)/project-requirements not found; nothing to decrypt"; \
		exit 0; \
	fi; \
	find $(SECRETS_DIR)/project-requirements -type f -name '*.sops' | while IFS= read -r enc; do \
		rel=$${enc#$(SECRETS_DIR)/project-requirements/}; \
		rel_out=$${rel%.sops}; \
		dest_dir="$(PLAINTEXT_REQUIREMENTS_DIR)/$$(dirname "$$rel_out")"; \
		dest_file="$(PLAINTEXT_REQUIREMENTS_DIR)/$$rel_out"; \
		mkdir -p "$$dest_dir"; \
		echo "Decrypting $$enc -> $$dest_file"; \
		SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "$$enc" > "$$dest_file"; \
	done
	$(call log_info,Requirements decrypted to $(PLAINTEXT_REQUIREMENTS_DIR)/)

# --- Help ---

help:
	@echo "Scalable Coupon System - Makefile Commands"
	@echo ""
	@echo "Development:"
	@echo "  make deps              - Download Go dependencies"
	@echo "  make fmt               - Format code"
	@echo "  make lint              - Run linter"
	@echo "  make vet               - Run go vet"
	@echo "  make security          - Run security scans (gosec + govulncheck)"
	@echo "  make check             - Run all checks (lint + vet + security)"
	@echo "  make test              - Run tests with coverage"
	@echo "  make cover             - Generate coverage HTML report"
	@echo "  make build             - Build the application"
	@echo "  make all               - Run fmt, lint, vet, security, and test"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build      - Build Docker images"
	@echo "  make docker-run        - Start services with docker-compose"
	@echo "  make docker-down       - Stop and remove services"
	@echo ""
	@echo "Secrets (SOPS):"
	@echo "  make encrypt-requirements  - Encrypt project_requirements/ to secrets/"
	@echo "  make decrypt-requirements  - Decrypt secrets/ to project_requirements/"
	@echo ""
	@echo "Setup SOPS:"
	@echo "  1. Install: brew install sops age (or equivalent)"
	@echo "  2. Generate key: age-keygen -o ~/.config/sops/age/keys.txt"
	@echo "  3. Update .sops.yaml with your public key"
