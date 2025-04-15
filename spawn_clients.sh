#!/usr/bin/env bash

n="${1:-1}"

for i in $(seq $n); do
  go run ./cmd/client &
done

wait
