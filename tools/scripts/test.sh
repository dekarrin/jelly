#!/bin/bash

# use this script to boot up the test server and see it in action

set -e

script_path="$(dirname "$0")"

cd "$script_path/../.."

echo "Building..."
env CGO_ENABLED=0 go build -o jellytest cmd/jellytest/*.go

echo "Running..."
./jellytest -c cmd/jellytest/jelly.yml
