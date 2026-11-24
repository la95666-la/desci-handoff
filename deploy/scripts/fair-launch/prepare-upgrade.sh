#!/usr/bin/env bash
# Offline-only complete dispatcher/facet-set upgrade-plan builder. No RPC or signing.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "$ROOT/scripts/fair-launch/lib/gate.sh"
fl_require_deployment_gate
: "${JC_FAIR_LAUNCH_PREVIOUS_IMPLEMENTATION:?previous implementation required}"
: "${JC_FAIR_LAUNCH_NEW_IMPLEMENTATION:?new implementation required}"
: "${JC_FAIR_LAUNCH_STORAGE_DIFF_DIGEST:?storage diff digest required}"
: "${JC_FAIR_LAUNCH_CONTRACT_COMMIT:?contract commit required}"
for k in JC_FAIR_LAUNCH_SOURCE_TREE_DIGEST JC_FAIR_LAUNCH_ABI_DIGEST JC_FAIR_LAUNCH_SELECTOR_MAP_DIGEST JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST JC_FAIR_LAUNCH_RELEASE_ATTESTATION_DIGEST; do fl_require_hex64 "$k"; done
fl_require_address JC_FAIR_LAUNCH_PREVIOUS_IMPLEMENTATION
fl_require_address JC_FAIR_LAUNCH_NEW_IMPLEMENTATION
fl_require_hex64 JC_FAIR_LAUNCH_STORAGE_DIFF_DIGEST
out="${1:-$ROOT/.stage/fair-launch-upgrade-plan.json}"
mkdir -p "$(dirname "$out")"
node - "$out" "$JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST" <<'NODE'
const fs=require('fs'),crypto=require('crypto');
const [out,dependencyPath]=process.argv.slice(2);
const text=fs.readFileSync(dependencyPath,'utf8').trim(),dependency=JSON.parse(text),digest=crypto.createHash('sha256').update(text).digest('hex');
if(digest!==process.env.JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_DIGEST||dependency.environment!=='mainnet'||dependency.chainId!==Number(process.env.JC_FAIR_LAUNCH_CHAIN_ID)||dependency.solc?.build!=='0.8.22+commit.4fc1097e'||dependency.solc?.optimizerRuns!==200||dependency.solc?.viaIR!==true||dependency.source?.commit!==process.env.JC_FAIR_LAUNCH_CONTRACT_COMMIT||dependency.source?.treeDigest!==process.env.JC_FAIR_LAUNCH_SOURCE_TREE_DIGEST||dependency.deploymentAttestationDigest!==process.env.JC_FAIR_LAUNCH_DEPLOYMENT_ATTESTATION_DIGEST||dependency.releaseAttestationDigest!==process.env.JC_FAIR_LAUNCH_RELEASE_ATTESTATION_DIGEST||dependency.sourceRegistry?.revision!==Number(process.env.JC_FAIR_LAUNCH_SOURCE_REGISTRY_REVISION)||dependency.sourceRegistry?.digest!==process.env.JC_FAIR_LAUNCH_SOURCE_REGISTRY_DIGEST||dependency.aggregateAbiDigest!==process.env.JC_FAIR_LAUNCH_ABI_DIGEST||dependency.selectorMapDigest!==process.env.JC_FAIR_LAUNCH_SELECTOR_MAP_DIGEST||dependency.storageLayoutDigest!==process.env.JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST||dependency.generation!==Number(process.env.JC_FAIR_LAUNCH_WATERMARK_GENERATION)||dependency.previousManifestDigest!==(process.env.JC_FAIR_LAUNCH_PREVIOUS_DEPENDENCY_MANIFEST_DIGEST||null))throw Error('upgrade dependency exact release/source/watermark/predecessor binding mismatch');
const plan={schemaVersion:'fair-launch-upgrade-plan-v2',chainId:56,proxyAddress:process.env.JC_FAIR_LAUNCH_PROXY_ADDRESS,previousImplementation:process.env.JC_FAIR_LAUNCH_PREVIOUS_IMPLEMENTATION,newImplementation:process.env.JC_FAIR_LAUNCH_NEW_IMPLEMENTATION,storageDiffDigest:process.env.JC_FAIR_LAUNCH_STORAGE_DIFF_DIGEST,dependencyManifest:dependency,dependencyManifestDigest:digest,governanceTimelockAddress:process.env.JC_FAIR_LAUNCH_GOVERNANCE_TIMELOCK_ADDRESS,deploymentAttestationDigest:dependency.deploymentAttestationDigest,releaseAttestationDigest:dependency.releaseAttestationDigest,sourceRegistryRevision:dependency.sourceRegistry.revision,sourceRegistryDigest:dependency.sourceRegistry.digest,watermarkDependencyManifestDigest:digest,releaseAdmissionDependencyManifestDigest:digest,capabilityManifestDigest:process.env.JC_FAIR_LAUNCH_CAPABILITY_MANIFEST_DIGEST,changeTicket:process.env.JC_FAIR_LAUNCH_CHANGE_TICKET};
fs.writeFileSync(out,JSON.stringify(plan)+'\n');
NODE
sha256sum "$out" > "$out.sha256"
echo "offline upgrade plan written: $out"
