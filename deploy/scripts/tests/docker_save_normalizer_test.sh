#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "$ROOT/scripts/release/lib/release_common.sh"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
SOURCE="$TMP/source.tar.gz"; OUTPUT="$TMP/images/joincare-backend-normalized.tar.gz"
mkdir -p "$TMP/images"

make_source() {
  local mutation="${1:-valid}"
  release_python - "$SOURCE" "$mutation" <<'PY'
import hashlib,io,json,sys,tarfile
path,mutation=sys.argv[1:]
data=b'layer'; digest=hashlib.sha256(data).hexdigest(); layer=f'blobs/sha256/{digest}'; config='blobs/sha256/'+'1'*64
source={'digest':'sha256:'+digest,'mediaType':'application/vnd.oci.image.layer.v1.tar','size':len(data)}
record={'Config':config,'RepoTags':['joincare/backend:test'],'Layers':[layer],'LayerSources':{'sha256:'+digest:source}}
if mutation=='unknown-key': record['Unknown']=True
elif mutation=='bad-digest': source['digest']='sha256:'+'2'*64
elif mutation=='bad-size': source['size']+=1
elif mutation=='bad-media': source['mediaType']='application/octet-stream'
elif mutation=='extra-source': record['LayerSources']['sha256:'+'3'*64]=dict(source)
elif mutation=='bad-layer-content': data=b'other'
with tarfile.open(path,'w:gz',format=tarfile.USTAR_FORMAT) as tf:
 def reg(name,payload):
  x=tarfile.TarInfo(name);x.size=len(payload);x.mode=0o644;tf.addfile(x,io.BytesIO(payload))
 def directory(name):
  x=tarfile.TarInfo(name);x.type=tarfile.DIRTYPE;x.mode=0o755;tf.addfile(x)
 reg('manifest.json',json.dumps([record],separators=(',',':')).encode())
 directory('blobs');directory('blobs/sha256')
 reg(config,b'{}');reg(layer,data)
 reg('index.json',b'{}');reg('oci-layout',b'{}');reg('repositories',b'{}')
PY
}

make_source valid
release_python "$ROOT/scripts/release/lib/normalize_docker_save.py" "$SOURCE" "$OUTPUT"
(cd "$TMP/images" && sha256sum "$(basename "$OUTPUT")" > "$(basename "$OUTPUT").sha256")
[ "$(verify_backend_image_package "$TMP/images")" = "$(basename "$OUTPUT")" ]

for mutation in unknown-key bad-digest bad-size bad-media extra-source bad-layer-content; do
  make_source "$mutation"
  if release_python "$ROOT/scripts/release/lib/normalize_docker_save.py" "$SOURCE" "$OUTPUT" >/dev/null 2>&1; then
    echo "normalizer accepted $mutation" >&2; exit 1
  fi
done

echo 'docker save normalizer policy: PASS'
