#!/usr/bin/env bash

set -euo pipefail

threshold="${COVERAGE_THRESHOLD:-90}"
profile="${1:-coverage.out}"
filtered_profile="${profile%.out}.filtered.out"
exclude_pattern="${COVERAGE_EXCLUDE_PATTERN:-github.com/.*/(cmd/api/|cmd/worker/|internal/migrate/)}"

go test ./... -coverprofile="$profile"

printf 'mode: set\n' > "$filtered_profile"
tail -n +2 "$profile" | grep -vE "$exclude_pattern" >> "$filtered_profile"

raw_total="$({ go tool cover -func="$profile" | awk '/^total:/ { gsub("%", "", $3); print $3 }'; } | tail -n 1)"
total="$({ go tool cover -func="$filtered_profile" | awk '/^total:/ { gsub("%", "", $3); print $3 }'; } | tail -n 1)"

printf 'Raw total coverage: %s%%\n' "$raw_total"
printf 'Filtered coverage: %s%%\n' "$total"
printf 'Excluded by default: %s\n' "$exclude_pattern"

awk -v total="$total" -v threshold="$threshold" 'BEGIN {
  if ((total + 0) < (threshold + 0)) {
    printf("coverage %.1f%% below threshold %.1f%%\n", total + 0, threshold + 0)
    exit 1
  }
}'