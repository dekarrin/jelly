#!/bin/bash

cd "$(dirname $0)/../.."

mkdir -p tools/mocks/jelly
mockgen -destination tools/mocks/jelly/mock_response.go github.com/dekarrin/jelly ResponseGenerator
