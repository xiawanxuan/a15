#!/bin/bash

set -e

APP_NAME="astro-scheduler"
BINARY_NAME="astro-scheduler"

echo "Building $APP_NAME for Linux AMD64..."

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/$BINARY_NAME ./cmd/server

echo "Build complete: build/$BINARY_NAME"

if command -v file &>/dev/null; then
    file build/$BINARY_NAME
fi
