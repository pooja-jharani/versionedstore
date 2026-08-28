#!/usr/bin/env bash
set -e
go build -o bin/versionedstore ./cmd/versionedstore
echo "Built ./bin/versionedstore"
