#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=scripts/release/lib/release_common.sh
source "$ROOT/scripts/release/lib/release_common.sh"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
DIR="$TMP/images"; mkdir -p "$DIR"
ARCHIVE="$DIR/joincare-backend-release-test.tar.gz"

make_archive() {
  local kind="$1"
  release_python - "$ARCHIVE" "$kind" <<'PY'
import io,json,sys,tarfile
path,kind=sys.argv[1:]
tag='joincare/backend:release-test'
config='config.json'
layer='layer/layer.tar'
record={'Config':config,'RepoTags':[tag],'Layers':[layer]}
if kind == 'foreign': record['RepoTags']=['other/backend:x']
elif kind == 'duplicate-ref': record['Layers']=[layer,layer]
elif kind == 'traversal-ref': record['Layers']=['../layer.tar']
elif kind == 'manifest-config-ref': record['Config']='manifest.json'
elif kind == 'missing-config-field': record.pop('Config')
elif kind == 'missing-layers-field': record.pop('Layers')
elif kind == 'empty-layers': record['Layers']=[]
elif kind == 'two-records': pass
manifest=[record,record] if kind == 'two-records' else [record]
with tarfile.open(path,'w:gz',format=tarfile.USTAR_FORMAT) as tf:
 def reg(name,data):
  x=tarfile.TarInfo(name); x.size=len(data); x.mode=0o644; tf.addfile(x,io.BytesIO(data))
 def directory(name):
  x=tarfile.TarInfo(name); x.type=tarfile.DIRTYPE; x.mode=0o755; tf.addfile(x)
 def link(name):
  x=tarfile.TarInfo(name); x.type=tarfile.SYMTYPE; x.linkname='target'; tf.addfile(x)
 raw=json.dumps(manifest,separators=(',',':')).encode()
 strict_manifest={
  'duplicate-config-key': b'[{"Config":"config.json","Config":"other.json","RepoTags":["joincare/backend:release-test"],"Layers":["layer/layer.tar"]}]',
  'duplicate-repotags-key': b'[{"Config":"config.json","RepoTags":["joincare/backend:release-test"],"RepoTags":["joincare/backend:other"],"Layers":["layer/layer.tar"]}]',
  'duplicate-layers-key': b'[{"Config":"config.json","RepoTags":["joincare/backend:release-test"],"Layers":["layer/layer.tar"],"Layers":["other/layer.tar"]}]',
  'manifest-nonfinite': b'[{"Config":"config.json","RepoTags":["joincare/backend:release-test"],"Layers":["layer/layer.tar"],"Extra":NaN}]',
  'manifest-extra-key': b'[{"Config":"config.json","RepoTags":["joincare/backend:release-test"],"Layers":["layer/layer.tar"],"Extra":true}]',
 }
 raw=strict_manifest.get(kind,raw)
 if kind == 'bad-manifest-json': raw=b'{'
 if kind == 'duplicate-manifest': reg('manifest.json',raw); reg('manifest.json',raw)
 elif kind == 'manifest-link': link('manifest.json')
 elif kind == 'oversize-manifest': reg('manifest.json',b' '*((1024*1024)+1))
 else: reg('manifest.json',raw)
 if kind not in {'missing-config','missing-config-field','manifest-config-ref'}:
  if kind == 'config-dir': directory(config)
  elif kind == 'config-link': link(config)
  else:
   config_data=b'not-json' if kind == 'bad-config-json' else b'{}'
   if kind == 'duplicate-nested-config-key': config_data=b'{"config":{"Env":["A=1"],"Env":["A=2"]}}'
   elif kind == 'config-nonfinite': config_data=b'{"created":Infinity}'
   elif kind == 'config-negative-infinity': config_data=b'{"created":-Infinity}'
   reg(config,config_data)
 if kind not in {'missing-layer','missing-layers-field','empty-layers','traversal-ref'}:
  directory('layer')
  if kind == 'layer-dir': directory(layer)
  elif kind == 'layer-link': link(layer)
  else: reg(layer,b'layer')
 if kind == 'extra-file': reg('extra.txt',b'extra')
 if kind == 'extra-dir': directory('unused')
 if kind == 'traversal-member': reg('../escape',b'x')
PY
}
sidecar(){ (cd "$DIR" && sha256sum "$(basename "$ARCHIVE")" > "$(basename "$ARCHIVE").sha256"); }
reject(){ if (verify_backend_image_package "$DIR") >/dev/null 2>&1; then echo "accepted $1" >&2; exit 1; fi; }

make_archive valid; sidecar; [ "$(verify_backend_image_package "$DIR")" = "$(basename "$ARCHIVE")" ]
MALICIOUS="$TMP/malicious.py"; MARKER="$TMP/malicious-executed"
printf 'from pathlib import Path\nPath("%s").write_text("executed")\nraise SystemExit(0)\n' "$MARKER" > "$MALICIOUS"
RELEASE_SAFE_TAR_HELPER_OVERRIDE="$MALICIOUS" verify_backend_image_package "$DIR" >/dev/null
[ ! -e "$MARKER" ] || { echo 'environment-selected archive helper executed' >&2; exit 1; }
printf '' > "$ARCHIVE.sha256"; reject empty-sidecar
make_archive valid; printf '%064d  %s\n' 0 "$(basename "$ARCHIVE")" > "$ARCHIVE.sha256"; reject wrong-digest
sidecar; sed -i 's/joincare-backend-release-test.tar.gz/other.tar.gz/' "$ARCHIVE.sha256"; reject basename-mismatch
for kind in foreign duplicate-ref traversal-ref manifest-config-ref missing-config-field missing-layers-field empty-layers two-records missing-config missing-layer extra-file extra-dir config-dir config-link layer-dir layer-link bad-manifest-json bad-config-json duplicate-manifest manifest-link oversize-manifest traversal-member duplicate-config-key duplicate-repotags-key duplicate-layers-key duplicate-nested-config-key manifest-nonfinite config-nonfinite config-negative-infinity manifest-extra-key; do
  make_archive "$kind"; sidecar; reject "$kind"
done

VERIFY="$ROOT/scripts/release/verify-test-runtime.sh"
mkdir -p "$TMP/directory-target"
if bash "$VERIFY" "$TMP/directory-target" >/dev/null 2>&1; then echo 'runtime verifier accepted a directory target' >&2; exit 1; fi
printf x > "$TMP/not-a-tar.zip"
if bash "$VERIFY" "$TMP/not-a-tar.zip" >/dev/null 2>&1; then echo 'runtime verifier accepted a non-.tar.gz target' >&2; exit 1; fi

# A packaged helper is evidence only: even valid outer inventory must reject a
# byte-different helper without executing it.
RUNTIME="$TMP/runtime"; mkdir -p "$RUNTIME"/{artifacts/backend,images,packages,deploy,scripts/release/lib}
cp "$ROOT/scripts/release/lib/safe_tar.py" "$RUNTIME/scripts/release/lib/safe_tar.py"
for bin in joincare-core joincare-job joincare-notice; do printf x > "$RUNTIME/artifacts/backend/$bin"; done
printf x > "$RUNTIME/deploy/Dockerfile.backend.runtime"
for file in \
  images/joincare-backend-release-x.tar.gz images/joincare-backend-release-x.tar.gz.sha256 \
  packages/joincare-frontend-release-x.tar.gz packages/joincare-frontend-release-x.tar.gz.sha256 \
  packages/joincare-admin-release-x.tar.gz packages/joincare-admin-release-x.tar.gz.sha256 \
  release-manifest.json README-DEPLOY.md; do printf x > "$RUNTIME/$file"; done
PACKAGED_MARKER="$TMP/packaged-helper-executed"
(cd "$RUNTIME" && find . -type f ! -name SHA256SUMS -printf '%P\0' | sort -z | xargs -0 sha256sum > SHA256SUMS)
OUTER="$TMP/runtime.tar.gz"
pack_runtime() {
  release_python - "$RUNTIME" "$OUTER" <<'PY'
from pathlib import Path
import sys, tarfile
root, output = Path(sys.argv[1]), Path(sys.argv[2])
with tarfile.open(output, "w:gz", format=tarfile.USTAR_FORMAT) as tf:
    for path in sorted(root.rglob("*")):
        info = tf.gettarinfo(str(path), path.relative_to(root).as_posix())
        if path.is_file():
            with path.open("rb") as source: tf.addfile(info, source)
        else: tf.addfile(info)
PY
  (cd "$TMP" && sha256sum runtime.tar.gz > runtime.tar.gz.sha256)
}
pack_runtime
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if bash "$VERIFY" "$OUTER" >/dev/null 2>&1; then echo 'runtime verifier unexpectedly extracted on Windows' >&2; exit 1; fi ;;
  *)
    EXTRACTED="$TMP/extracted-trusted"
    digest="$(verify_package_sidecar_digest "$TMP" runtime.tar.gz)"
    safe_extract_release_tar "$OUTER" "$EXTRACTED" runtime "$digest"
    verify_checksum_inventory "$EXTRACTED"
    cmp -s "$ROOT/scripts/release/lib/safe_tar.py" "$EXTRACTED/scripts/release/lib/safe_tar.py" \
      || { echo 'trusted packaged helper did not match repository helper' >&2; exit 1; } ;;
esac
printf 'from pathlib import Path\nPath("%s").write_text("executed")\n' "$PACKAGED_MARKER" > "$RUNTIME/scripts/release/lib/safe_tar.py"
(cd "$RUNTIME" && find . -type f ! -name SHA256SUMS -printf '%P\0' | sort -z | xargs -0 sha256sum > SHA256SUMS)
pack_runtime
case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*)
    if bash "$VERIFY" "$OUTER" >/dev/null 2>&1; then echo 'runtime verifier accepted a mismatched packaged helper on Windows' >&2; exit 1; fi ;;
  *)
    EXTRACTED="$TMP/extracted-mismatch"
    digest="$(verify_package_sidecar_digest "$TMP" runtime.tar.gz)"
    safe_extract_release_tar "$OUTER" "$EXTRACTED" runtime "$digest"
    verify_checksum_inventory "$EXTRACTED"
    if cmp -s "$ROOT/scripts/release/lib/safe_tar.py" "$EXTRACTED/scripts/release/lib/safe_tar.py"; then
      echo 'runtime evidence accepted a mismatched packaged helper' >&2; exit 1
    fi ;;
esac
[ ! -e "$PACKAGED_MARKER" ] || { echo 'packaged archive helper executed' >&2; exit 1; }

echo 'backend archive adversarial policy: PASS'
