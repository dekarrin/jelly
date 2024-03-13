#!/bin/bash

# use this script to run unit tetss. Unless the `--all` flag is specified, runs
# in short mode.

set -e

script_path="$(dirname "$0")"

cd "$script_path/../.."

pkgs=$(go list ./... | grep -v github.com/dekarrin/jelly/tools)

if [ "$1" = '--all' ]; then
    go test $pkgs -count 1
else
    go test $pkgs -short -count 1
fi
