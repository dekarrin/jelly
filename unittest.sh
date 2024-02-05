#!/bin/bash

# use this script to run unit tetss. Unless the `--all` flag is specified, runs
# in short mode.

set -e

script_path="$(dirname "$0")"

cd "$script_path"

if [ "$1" = '--all' ]; then
    go test ./...
else
    go test ./... -short
fi
