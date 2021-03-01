#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Create output folder
mkdir -p $DIR/dist

cd $DIR

# Pre-req: go get github.com/markbates/pkger/cmd/pkger
GOBIN="${GOPATH:-~/go}"
$GOBIN/bin/pkger

GOOS=darwin GOARCH=amd64 go build -o dist/rpCheckup_darwin_amd64
GOOS=darwin GOARCH=arm64 go build -o dist/rpCheckup_darwin_arm64

GOOS=linux GOARCH=amd64 go build -o dist/rpCheckup_linux
