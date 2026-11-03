#!/bin/bash

go mod tidy

# Pre-build tool dependencies declared in go.mod (tool directive)
go tool gocover-cobertura -h > /dev/null 2>&1
go tool govulncheck -h > /dev/null 2>&1
