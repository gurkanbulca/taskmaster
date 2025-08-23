#!/bin/bash
# Generate Go code from proto submodule

set -e

echo "🔄 Generating Go code from proto submodule..."

# Update submodule to latest
git submodule update --remote proto

# Clean old generated code
rm -rf api/proto/*/v1/generated

# Create directories
mkdir -p api/proto/auth/v1/generated
mkdir -p api/proto/task/v1/generated
mkdir -p api/proto/common/v1/generated

# Generate Go code from submodule
cd proto
make generate-go
cd ..

# Copy generated Go code to backend structure
cp -r proto/gen/go/auth/v1/* api/proto/auth/v1/generated/
cp -r proto/gen/go/task/v1/* api/proto/task/v1/generated/
cp -r proto/gen/go/common/v1/* api/proto/common/v1/generated/

echo "✅ Go code generated from proto submodule!"