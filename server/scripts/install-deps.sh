#!/bin/bash

# Install all dependencies from go.mod (equivalent to npm install)

set -e

cd "$(dirname "$0")/.."

echo "📦 Installing Go dependencies..."

# Remove go.work if it exists (can cause issues)
if [ -f "go.work" ]; then
    echo "Removing go.work file..."
    rm -f go.work go.work.sum
fi

# Download all dependencies (equivalent to npm install)
echo "Downloading dependencies..."
go mod download

# Tidy up the module (adds missing, removes unused)
echo "Tidying module..."
go mod tidy

# Verify dependencies
echo "Verifying dependencies..."
go mod verify

echo ""
echo "✅ Dependencies installed successfully!"

