#!/bin/bash
set -o nounset

GOOS=linux GOARCH=amd64 go build -v || {
    echo "build failed!!!!!!!!!"
    exit 2
}
