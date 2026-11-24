#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
dockerfile="$root/deploy/Dockerfile.backend.runtime"

grep -Eq '^FROM debian@sha256:[0-9a-f]{64}$' "$dockerfile" || {
  echo 'backend runtime base image must be digest-pinned' >&2
  exit 1
}
for binary in core job notice; do
  grep -Eq "^COPY --chmod=0755 artifacts/backend/joincare-${binary}[[:space:]]+/app/joincare-${binary}$" "$dockerfile" || {
    echo "backend runtime $binary COPY must set executable mode in the copy layer" >&2
    exit 1
  }
done
if grep -Eq '^RUN .*chmod.*joincare-' "$dockerfile"; then
  echo 'backend runtime must not copy-up all binaries in a chmod layer' >&2
  exit 1
fi
if grep -Eq '^FROM[[:space:]]+joincare/backend:' "$dockerfile"; then
  echo 'backend runtime must not inherit a previous Backend image' >&2
  exit 1
fi
echo 'backend runtime image size guard passed'
