#!/bin/bash
set -e

docker run --rm -v "$PWD":/go/src/github.com/wanghengwei/monclient -w /go/src/github.com/wanghengwei/monclient golang:1.9 go build -v