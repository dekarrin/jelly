#!/bin/bash

# use this script to boot up the test server and see it in action

script_path="$(dirname "$0")"

cd "$0"

env CGO_ENABLED=0 go build -o jellytest cmd/jellytest/main.go

./jellytest -c cmd/jellytest/config.yml
