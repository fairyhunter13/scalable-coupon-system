#!/bin/bash
#
# check-version-consistency.sh
#
# Verifies that Go version is consistent across:
# - go.mod (line: go X.Y.Z)
# - Dockerfile (line: FROM golang:X.Y-alpine)
# - .github/workflows/ci.yml (env: GO_VERSION: 'X.Y.Z')
#
# Compares major.minor only (patch versions can differ)
#
# Usage:
#   ./scripts/check-version-consistency.sh
#   make version-check
#

set -e

# Ensure we're running from project root (where go.mod exists)
if [ ! -f "go.mod" ]; then
    # Try to find project root by looking for go.mod
    SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
    PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
    if [ -f "$PROJECT_ROOT/go.mod" ]; then
        cd "$PROJECT_ROOT"
    else
        echo "::error::Must be run from project root (where go.mod exists)"
        exit 1
    fi
fi

# Verify all required files exist
for f in go.mod Dockerfile .github/workflows/ci.yml; do
    if [ ! -f "$f" ]; then
        echo "::error::Required file not found: $f"
        exit 1
    fi
done

# Extract version from go.mod (format: "go 1.25.5" -> "1.25")
GOMOD_VERSION=$(grep '^go ' go.mod | awk '{print $2}' | cut -d. -f1,2)

# Extract version from Dockerfile (format: "FROM golang:1.25-alpine" -> "1.25")
DOCKERFILE_VERSION=$(grep '^FROM golang:' Dockerfile | grep -oE 'golang:[0-9]+\.[0-9]+' | cut -d: -f2)

# Extract version from CI workflow (format: "GO_VERSION: '1.25.5'" -> "1.25")
# Use sed to extract the quoted version value more precisely
CI_VERSION=$(grep "^[[:space:]]*GO_VERSION:" .github/workflows/ci.yml | sed -E "s/.*['\"]([0-9]+\.[0-9]+)[^'\"]*['\"].*/\1/")

# Check if all versions were extracted successfully
if [ -z "$GOMOD_VERSION" ]; then
    echo "::error::Failed to extract Go version from go.mod"
    exit 1
fi

if [ -z "$DOCKERFILE_VERSION" ]; then
    echo "::error::Failed to extract Go version from Dockerfile"
    exit 1
fi

if [ -z "$CI_VERSION" ]; then
    echo "::error::Failed to extract Go version from .github/workflows/ci.yml"
    exit 1
fi

# Compare versions
if [ "$GOMOD_VERSION" = "$DOCKERFILE_VERSION" ] && [ "$GOMOD_VERSION" = "$CI_VERSION" ]; then
    echo "::notice::Version consistency check passed - all files specify Go $GOMOD_VERSION"
    echo ""
    echo "Versions found:"
    echo "  go.mod:      $GOMOD_VERSION"
    echo "  Dockerfile:  $DOCKERFILE_VERSION"
    echo "  ci.yml:      $CI_VERSION"
    exit 0
else
    echo "::error::Version mismatch detected"
    echo "::error::  go.mod: $GOMOD_VERSION"
    echo "::error::  Dockerfile: $DOCKERFILE_VERSION"
    echo "::error::  CI workflow: $CI_VERSION"
    echo "::error::All Go versions must match (major.minor)"
    echo ""
    echo "To fix this issue, update the following files to use the same Go version:"
    echo "  - go.mod: Line starting with 'go X.Y'"
    echo "  - Dockerfile: Line starting with 'FROM golang:X.Y-alpine'"
    echo "  - .github/workflows/ci.yml: GO_VERSION env variable"
    exit 1
fi
