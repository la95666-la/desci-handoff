#!/usr/bin/env bash
# OFFLINE REVIEW ONLY. No installation, service restart, migration or signing.
set -Eeuo pipefail
umask 077
TOOL_ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
REPO_ROOT="$(CDPATH= cd -- "$TOOL_ROOT/../.." && pwd -P)"
OUT="${1:-$TOOL_ROOT/dist/job-second-phase-review}"
[[ "$OUT" = /* ]] || { echo 'output path must be absolute' >&2; exit 2; }
[[ ! -e "$OUT" ]] || { echo 'output already exists; refusing overwrite' >&2; exit 2; }
mkdir -p -- "$OUT/bin"
trap 'rm -rf -- "$OUT"' ERR
# Snapshot inputs before compilation, then require the same inventory after
# compilation. A post-build hash alone cannot prove which bytes Go consumed.
(
  cd "$REPO_ROOT"
  git ls-files -z --cached --others --exclude-standard -- tools/jc-release | sort -z |
    xargs -0 -r sha256sum > "$OUT/SOURCE-SHA256SUMS"
  [[ -s "$OUT/SOURCE-SHA256SUMS" ]] || { echo 'empty source inventory' >&2; exit 1; }
)
(
  cd "$TOOL_ROOT"
  go test ./... -count=1
  go vet ./...
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$OUT/bin/jc-release-server" ./cmd/jc-release-server
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$OUT/bin/jc-agent" ./cmd/jc-agent
)
cp -- "$TOOL_ROOT/SECOND-PHASE-PREFLIGHT.md" "$OUT/SECOND-PHASE-PREFLIGHT.md"
cp -- "$TOOL_ROOT/README.md" "$OUT/README.md"
cp -- "$TOOL_ROOT/SECOND-PHASE-REVIEW.md" "$OUT/SECOND-PHASE-REVIEW.md"
(
  cd "$REPO_ROOT"
  # Include tracked AND untracked, non-ignored tool sources. HEAD alone cannot
  # identify binaries produced from this dirty worktree. Paths are relative to
  # the repository root for a later, read-only sha256sum -c comparison.
  git ls-files -z --cached --others --exclude-standard -- tools/jc-release | sort -z |
    xargs -0 -r sha256sum > "$OUT/SOURCE-AFTER-SHA256SUMS"
  cmp -s "$OUT/SOURCE-SHA256SUMS" "$OUT/SOURCE-AFTER-SHA256SUMS" || {
    echo 'source inventory changed during build' >&2; exit 1;
  }
  rm "$OUT/SOURCE-AFTER-SHA256SUMS"
  sha256sum -c --status "$OUT/SOURCE-SHA256SUMS"
  printf 'schema=joincare.job.offline-review.v1\n'
  printf 'source_commit=%s\n' "$(git rev-parse HEAD)"
  printf 'source_worktree=uncommitted-local-candidate\n'
  printf 'source_inventory=SOURCE-SHA256SUMS (paths relative to repository root; includes untracked tool files)\n'
  printf 'operational_boundary=review-only-NO-GO\n'
  printf 'target_os=linux\ntarget_arch=amd64\n'
) > "$OUT/REVIEW.txt"
(
  cd "$OUT"
  sha256sum bin/jc-release-server bin/jc-agent REVIEW.txt README.md SECOND-PHASE-PREFLIGHT.md SECOND-PHASE-REVIEW.md SOURCE-SHA256SUMS > SHA256SUMS
  sha256sum -c SHA256SUMS
)
echo "Offline review candidate: $OUT (NOT an installable or production-approved release)"
