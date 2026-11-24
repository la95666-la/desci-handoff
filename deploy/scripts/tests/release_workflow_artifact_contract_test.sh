#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/release/lib/release_common.sh
source "$ROOT/scripts/release/lib/release_common.sh"
WORKFLOW="$ROOT/.github/workflows/release.yml"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT

# The workflow is archive-disabled until immutable third-party Action SHAs are
# independently admitted, but its retained evidence contract must remain exact.
grep -Fq 'if: ${{ false }}' "$WORKFLOW"
! grep -Eq 'uses:[[:space:]]+' "$WORKFLOW"
grep -Fq 'FAIR_LAUNCH_REVIEWED_DEPLOY_REF: ${{ inputs.deploy_sha }}' "$WORKFLOW"
grep -Fq '[ "$D" = "$CURRENT_DEPLOY" ]' "$WORKFLOW"
grep -Fq "'joincare.release-evidence.v1'" "$ROOT/scripts/release/build-test-runtime.sh"
grep -Fq 'ADMISSION_STATUS="UNADMITTED_OFFLINE_EVIDENCE_ONLY"' "$ROOT/scripts/release/build-test-runtime.sh"
grep -Fq "'joincare.test-server-release-candidate.v1'" "$ROOT/scripts/release/build-test-runtime.sh"
grep -Fq 'CANDIDATE_PACKAGE_BUILD_ADMITTED' "$ROOT/scripts/release/build-test-runtime.sh"
grep -Fq 'identities:' "$ROOT/scripts/release/build-test-runtime.sh"
grep -Fq 'manifest identities must be exactly deploy, contract, backend, frontend, admin' "$ROOT/scripts/release/verify-test-runtime.sh"
# Any future admitted workflow must publish the builder-created adjacent sidecar under
# its exact basename. A generic self-generated tarball.sha256 is not verifier-compatible.
grep -Fq 'S="$T.sha256"' "$WORKFLOW"
grep -Fq 'TARBALL_SIDECAR_PATH=$S' "$WORKFLOW"
grep -Fq 'TARBALL_PATH=$T' "$WORKFLOW"
grep -Fq 'TARBALL_SIDECAR_PATH=$S' "$WORKFLOW"
! grep -Eq '(^|[[:space:]/])tarball\.sha256([[:space:]]|$)' "$WORKFLOW"

PKG="$TMP/pkg"
mkdir -p "$PKG"/{artifacts/backend,images,packages,deploy,scripts/release/lib}
cp "$ROOT/scripts/release/lib/safe_tar.py" "$PKG/scripts/release/lib/safe_tar.py"
printf x > "$PKG/deploy/Dockerfile.backend.runtime"
printf 'offline evidence only\n' > "$PKG/README-DEPLOY.md"
# Minimal Linux/amd64 ELF identity accepted by the offline verifier.
for bin in joincare-core joincare-job joincare-notice; do
  printf '\177ELF\002\001\001\000\000\000\000\000\000\000\000\000\002\000\076\000' > "$PKG/artifacts/backend/$bin"
done
cat > "$PKG/release-manifest.json" <<'JSON'
{
  "schema": "joincare.release-evidence.v1",
  "admission_status": "UNADMITTED_OFFLINE_EVIDENCE_ONLY",
  "profile": "test-server-mainnet-checkin",
  "built_at": "20260909-000000",
  "chain": {
    "checkin_chain_id": 56,
    "checkin_address": "0x0000000000000000000000000000000000000001",
    "trusted_project_chains": [97]
  },
  "identities": {
    "deploy": {"commit":"4444444444444444444444444444444444444444"},
    "contract": {"commit":"5555555555555555555555555555555555555555"},
    "backend": {"commit":"1111111111111111111111111111111111111111"},
    "frontend": {"commit":"2222222222222222222222222222222222222222"},
    "admin": {"commit":"3333333333333333333333333333333333333333"}
  },
  "components": {
    "backend": {"commit":"1111111111111111111111111111111111111111","lockfile":{"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
    "frontend": {"commit":"2222222222222222222222222222222222222222","lockfile":{"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}},
    "admin": {"commit":"3333333333333333333333333333333333333333","lockfile":{"sha256":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}}
  }
}
JSON

# Complete minimal docker-save closure.
DOCKER="$TMP/docker"; mkdir -p "$DOCKER/layer"
printf '{}' > "$DOCKER/config.json"
printf layer > "$DOCKER/layer/layer.tar"
printf '[{"Config":"config.json","RepoTags":["joincare/backend:workflow-test"],"Layers":["layer/layer.tar"]}]' > "$DOCKER/manifest.json"
tar -czf "$PKG/images/joincare-backend-workflow-test.tar.gz" -C "$DOCKER" manifest.json config.json layer/layer.tar
(cd "$PKG/images" && sha256sum joincare-backend-workflow-test.tar.gz > joincare-backend-workflow-test.tar.gz.sha256)

# Minimal strict static archives. Frontend includes the exact trusted bundle markers.
for comp in frontend admin; do
  mkdir -p "$TMP/$comp/dist"
  printf '<html></html>' > "$TMP/$comp/dist/index.html"
  if [ "$comp" = frontend ]; then
    printf 'var e={VITE_CHECKIN_ADDRESS:"0x0000000000000000000000000000000000000001",VITE_CHECKIN_CHAIN_ID:"56",VITE_TRUSTED_PROJECT_CHAINS:"97",VITE_HARDHAT_RPC:""}' > "$TMP/$comp/dist/app.js"
  fi
  tar -czf "$PKG/packages/joincare-$comp-workflow-test.tar.gz" -C "$TMP/$comp" dist
  (cd "$PKG/packages" && sha256sum "joincare-$comp-workflow-test.tar.gz" > "joincare-$comp-workflow-test.tar.gz.sha256")
done
(cd "$PKG" && find . -type f ! -name SHA256SUMS -printf '%P\0' | sort -z | xargs -0 sha256sum > SHA256SUMS)

BUILT="$TMP/built/joincare-release-test-server-mainnet-checkin-workflow-test.tar.gz"
mkdir -p "$(dirname "$BUILT")"
release_python - "$PKG" "$BUILT" <<'PY'
from pathlib import Path
import sys,tarfile
root,out=Path(sys.argv[1]),Path(sys.argv[2])
with tarfile.open(out,'w:gz',format=tarfile.USTAR_FORMAT) as tf:
    for path in sorted(root.rglob('*')):
        info=tf.gettarinfo(str(path),path.relative_to(root).as_posix())
        if path.is_file():
            with path.open('rb') as source: tf.addfile(info,source)
        else: tf.addfile(info)
PY
(cd "$(dirname "$BUILT")" && sha256sum "$(basename "$BUILT")" > "$(basename "$BUILT").sha256")

# Simulate actions/download-artifact preserving both original adjacent names.
DOWNLOAD="$TMP/downloaded/release-tarball"; mkdir -p "$DOWNLOAD"
cp "$BUILT" "$BUILT.sha256" "$DOWNLOAD/"
DOWNLOADED="$DOWNLOAD/$(basename "$BUILT")"
[ -f "$DOWNLOADED.sha256" ]
digest="$(verify_package_sidecar_digest "$DOWNLOAD" "$(basename "$DOWNLOADED")")"
[ "$digest" = "$(sha256sum "$DOWNLOADED" | awk '{print $1}')" ]
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) : ;; # Native no-follow extraction intentionally unavailable.
  *)
    if command -v node >/dev/null 2>&1; then
      bash "$ROOT/scripts/release/verify-test-runtime.sh" "$DOWNLOADED" --profile test-server-mainnet-checkin >/dev/null
      cp "$PKG/release-manifest.json" "$TMP/manifest-good.json"
      for mutation in missing extra badhex; do
        cp "$TMP/manifest-good.json" "$PKG/release-manifest.json"
        node - "$PKG/release-manifest.json" "$mutation" <<'NODE'
const fs=require('fs'); const p=process.argv[2], mutation=process.argv[3]; const m=JSON.parse(fs.readFileSync(p));
if(mutation==='missing') delete m.identities.deploy;
if(mutation==='extra') m.identities.job={commit:'6666666666666666666666666666666666666666'};
if(mutation==='badhex') m.identities.contract.commit='not-a-commit';
fs.writeFileSync(p,JSON.stringify(m));
NODE
        (cd "$PKG" && find . -type f ! -name SHA256SUMS -printf '%P\0' | sort -z | xargs -0 sha256sum > SHA256SUMS)
        release_python - "$PKG" "$BUILT" <<'PY'
from pathlib import Path
import sys,tarfile
root,out=Path(sys.argv[1]),Path(sys.argv[2])
with tarfile.open(out,'w:gz',format=tarfile.USTAR_FORMAT) as tf:
 for path in sorted(root.rglob('*')):
  info=tf.gettarinfo(str(path),path.relative_to(root).as_posix())
  if path.is_file():
   with path.open('rb') as source: tf.addfile(info,source)
  else: tf.addfile(info)
PY
        (cd "$(dirname "$BUILT")" && sha256sum "$(basename "$BUILT")" > "$(basename "$BUILT").sha256")
        if bash "$ROOT/scripts/release/verify-test-runtime.sh" "$BUILT" >/dev/null 2>&1; then echo "accepted invalid manifest identity set: $mutation" >&2; exit 1; fi
      done
      cp "$TMP/manifest-good.json" "$PKG/release-manifest.json"
    else
      # Minimal Linux environments still prove the downloaded artifact naming,
      # external digest and safe outer-archive contract without a source build.
      EXTRACTED="$TMP/extracted"
      safe_extract_release_tar "$DOWNLOADED" "$EXTRACTED" runtime "$digest"
      verify_checksum_inventory "$EXTRACTED"
      cmp -s "$ROOT/scripts/release/lib/safe_tar.py" "$EXTRACTED/scripts/release/lib/safe_tar.py"
    fi ;;
esac

echo 'release workflow adjacent sidecar contract: PASS'
