#!/bin/bash

cd "$(dirname $0)/../.."

pwd

mkdir -p tools/mocks/jelly
mockgen -destination tools/mocks/jelly/mock_response.go github.com/dekarrin/jelly ResponseGenerator
mockgen -destination tools/mocks/jelly/mock_authenticator.go github.com/dekarrin/jelly Authenticator
mockgen -destination tools/mocks/jelly/mock_logger.go github.com/dekarrin/jelly Logger
mockgen -destination tools/mocks/jelly/mock_api.go github.com/dekarrin/jelly API