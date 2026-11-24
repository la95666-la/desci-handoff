#!/usr/bin/env bash
# Machine-verifiable negative matrix for the BSC mainnet Fair Launch candidate
# package admission. This does not build, upload, Apply, migrate or start work.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VALIDATOR="$ROOT/scripts/release/validate-fair-launch-release-admission.js"
SOURCE="$ROOT/scripts/release/admission/fair-launch-test-server-release-admission-v1.json"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

export VITE_REOWN_PROJECT_ID=0123456789abcdef0123456789abcdef
set -a
# shellcheck disable=SC1091
source "$ROOT/scripts/release/profiles/test-server-fair-launch-bsc-mainnet.env"
set +a
# shellcheck disable=SC1091
source "$ROOT/scripts/release/lib/release_common.sh"

COMMITS='{"contract":"fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41","backend":"f29c4062a199c615d8c8372d0850ece0ed09fbc0","frontend":"58cf098fc965a72c98d0278f31f0b2b4b427b1e4","admin":"5ffbb2b97c0a1752ac079753d2b0d2c7fafb6ff5"}'
export FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS="$COMMITS"

fail() { printf 'FAIL %s\n' "$*" >&2; exit 1; }
pass() { printf 'PASS %s\n' "$*"; }
rejects() {
    local file="$1" label="$2"
    if node "$VALIDATOR" source "$file" "$ROOT" >"$TMP/out" 2>&1; then
        fail "$label accepted"
    fi
    pass "rejects: $label"
}
mutate() {
    local name="$1" expression="$2"
    node - "$SOURCE" "$TMP/$name.json" "$expression" <<'NODE'
const fs=require('fs'); const [src,dst,expression]=process.argv.slice(2);
const value=JSON.parse(fs.readFileSync(src,'utf8'));
Function('value', expression)(value);
fs.writeFileSync(dst,JSON.stringify(value,null,2)+'\n');
NODE
}

validate_release_profile test-server-fair-launch-bsc-mainnet
node "$VALIDATOR" source "$SOURCE" "$ROOT" >/dev/null
pass 'exact Fair Launch profile and admission accepted'

(
    RELEASE_PROFILE=test-server-mainnet-checkin
    if validate_release_profile test-server-fair-launch-bsc-mainnet >/dev/null 2>&1; then exit 0; else exit 1; fi
) && fail 'old CheckIn profile identity accepted as Fair Launch' || pass 'rejects: old CheckIn profile identity mix'

mutate chain 'value.chain.chainId=97'
rejects "$TMP/chain.json" 'chainId drift'
mutate proxy 'value.chain.proxyAddress="0x0000000000000000000000000000000000000001"'
rejects "$TMP/proxy.json" 'proxy drift'
mutate start 'value.chain.startBlock++'
rejects "$TMP/start.json" 'start block drift'
mutate finality 'value.chain.confirmationPolicy="12_blocks"'
rejects "$TMP/finality.json" 'finality policy drift'
mutate runtime 'value.components[0].runtimeCodeHash="0x"+"00".repeat(32)'
rejects "$TMP/runtime.json" 'component runtime hash drift'
mutate commit 'value.identities.backend.commit="0".repeat(40)'
rejects "$TMP/commit.json" 'backend commit drift'
mutate migration 'value.migrations[5].sha256="0".repeat(64)'
rejects "$TMP/migration.json" 'migration checksum drift'
mutate apply 'value.permissions.operationalApplyPermitted=true'
rejects "$TMP/apply.json" 'operational Apply permission'
mutate upload 'value.permissions.artifactUploadPermitted=true'
rejects "$TMP/upload.json" 'artifact upload permission'
mutate token 'value.paymentTokens[0].enabledForNewProjects=true'
rejects "$TMP/token.json" 'unexecuted payment-token admission'

FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS='{"contract":"fe29c78c6c281cfeaa6507e8659e1d0c4dd45e41","backend":"0000000000000000000000000000000000000000","frontend":"58cf098fc965a72c98d0278f31f0b2b4b427b1e4","admin":"5ffbb2b97c0a1752ac079753d2b0d2c7fafb6ff5"}'
export FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS
rejects "$SOURCE" 'actual backend worktree commit drift'
export FAIR_LAUNCH_RELEASE_COMPONENT_COMMITS="$COMMITS"

cp "$SOURCE" "$TMP/duplicate.json"
release_python - "$TMP/duplicate.json" <<'PY'
import sys
p=sys.argv[1]
s=open(p,encoding='utf-8').read()
s=s.replace('  "revision": 1,','  "revision": 1,\n  "revision": 2,',1)
open(p,'w',encoding='utf-8').write(s)
PY
rejects "$TMP/duplicate.json" 'duplicate JSON key'

mkdir -p "$TMP/package"
cp "$SOURCE" "$TMP/package/fair-launch-release-authority.json"
node - "$SOURCE" "$TMP/package/release-manifest.json" "$(git -C "$ROOT" rev-parse HEAD)" <<'NODE'
const fs=require('fs'); const [src,dst,deployCommit]=process.argv.slice(2); const a=JSON.parse(fs.readFileSync(src));
const sha=require('crypto').createHash('sha256').update(fs.readFileSync(src)).digest('hex');
const manifest={
 schema:'joincare.test-server-release-candidate.v1', admission_status:'CANDIDATE_PACKAGE_BUILD_ADMITTED',
 profile:a.profile, environment:a.environment,
 admission:{id:a.admissionId,revision:a.revision,sha256:sha,package_build_permitted:true,operational_apply_permitted:false},
 identities:{deploy:{commit:deployCommit},contract:a.identities.contract,backend:a.identities.backend,frontend:a.identities.frontend,admin:a.identities.admin},
 fair_launch:{chain:a.chain,bundle:a.bundle,components:a.components,authorities:a.authorities,paymentTokens:a.paymentTokens,migrations:a.migrations,
   capabilities:{walletSubmissionPermitted:false,adminDispatchPermitted:false,materializationPermitted:false}}
};
fs.writeFileSync(dst,JSON.stringify(manifest,null,2)+'\n');
NODE
node "$VALIDATOR" package "$TMP/package/fair-launch-release-authority.json" "$TMP/package" "$ROOT" >/dev/null
pass 'packaged authority is checksum-bound to candidate manifest'

cp "$TMP/package/release-manifest.json" "$TMP/package/release-manifest.good"
node - "$TMP/package/release-manifest.json" <<'NODE'
const fs=require('fs'),p=process.argv[2],m=JSON.parse(fs.readFileSync(p));m.identities.backend.commit='0'.repeat(40);fs.writeFileSync(p,JSON.stringify(m,null,2)+'\n');
NODE
if node "$VALIDATOR" package "$TMP/package/fair-launch-release-authority.json" "$TMP/package" "$ROOT" >/dev/null 2>&1; then fail 'package manifest commit drift accepted'; else pass 'rejects: packaged manifest commit drift'; fi
mv "$TMP/package/release-manifest.good" "$TMP/package/release-manifest.json"
node - "$TMP/package/release-manifest.json" <<'NODE'
const fs=require('fs'),p=process.argv[2],m=JSON.parse(fs.readFileSync(p));m.fair_launch.capabilities.materializationPermitted=true;fs.writeFileSync(p,JSON.stringify(m,null,2)+'\n');
NODE
if node "$VALIDATOR" package "$TMP/package/fair-launch-release-authority.json" "$TMP/package" "$ROOT" >/dev/null 2>&1; then fail 'package materialization capability accepted'; else pass 'rejects: packaged materialization capability'; fi

printf '\nFair Launch release admission matrix: PASS\n'
