#!/usr/bin/env bash
# Generates an offline deployment plan only. It never calls RPC or a signer.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
source "$ROOT/scripts/fair-launch/lib/gate.sh"
fl_require_deployment_gate
: "${JC_FAIR_LAUNCH_CONTRACT_COMMIT:?contract commit required}"
: "${JC_FAIR_LAUNCH_ABI_DIGEST:?ABI digest required}"
: "${JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST:?storage digest required}"
: "${JC_FAIR_LAUNCH_SELECTOR_MAP_DIGEST:?selector map digest required}"
: "${JC_FAIR_LAUNCH_SOURCE_TREE_DIGEST:?source tree digest required}"
: "${JC_FAIR_LAUNCH_RELEASE_ATTESTATION_DIGEST:?release attestation required}"
fl_require_hex64 JC_FAIR_LAUNCH_ABI_DIGEST
fl_require_hex64 JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST
fl_require_hex64 JC_FAIR_LAUNCH_SELECTOR_MAP_DIGEST
fl_require_hex64 JC_FAIR_LAUNCH_SOURCE_TREE_DIGEST
fl_require_hex64 JC_FAIR_LAUNCH_RELEASE_ATTESTATION_DIGEST
out="${1:-$ROOT/.stage/fair-launch-deployment-plan.json}"
mkdir -p "$(dirname "$out")"
node - "$out" "$JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST" <<'NODE'
const fs=require('fs'),crypto=require('crypto');
const [out,dependencyPath]=process.argv.slice(2);
const dependencyText=fs.readFileSync(dependencyPath,'utf8').trim(),dependency=JSON.parse(dependencyText);
const dependencyDigest=crypto.createHash('sha256').update(dependencyText).digest('hex');
if(dependencyDigest!==process.env.JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_DIGEST)throw Error('canonical dependency manifest digest mismatch');
if(dependency.environment!=='mainnet'||dependency.chainId!==Number(process.env.JC_FAIR_LAUNCH_CHAIN_ID)||dependency.solc?.build!=='0.8.22+commit.4fc1097e'||dependency.solc?.optimizerRuns!==200||dependency.solc?.viaIR!==true||dependency.source?.commit!==process.env.JC_FAIR_LAUNCH_CONTRACT_COMMIT||dependency.source?.treeDigest!==process.env.JC_FAIR_LAUNCH_SOURCE_TREE_DIGEST||dependency.deploymentAttestationDigest!==process.env.JC_FAIR_LAUNCH_DEPLOYMENT_ATTESTATION_DIGEST||dependency.releaseAttestationDigest!==process.env.JC_FAIR_LAUNCH_RELEASE_ATTESTATION_DIGEST||dependency.sourceRegistry?.revision!==Number(process.env.JC_FAIR_LAUNCH_SOURCE_REGISTRY_REVISION)||dependency.sourceRegistry?.digest!==process.env.JC_FAIR_LAUNCH_SOURCE_REGISTRY_DIGEST||dependency.aggregateAbiDigest!==process.env.JC_FAIR_LAUNCH_ABI_DIGEST||dependency.selectorMapDigest!==process.env.JC_FAIR_LAUNCH_SELECTOR_MAP_DIGEST||dependency.storageLayoutDigest!==process.env.JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST||dependency.generation!==Number(process.env.JC_FAIR_LAUNCH_WATERMARK_GENERATION)||dependency.previousManifestDigest!==(process.env.JC_FAIR_LAUNCH_PREVIOUS_DEPENDENCY_MANIFEST_DIGEST||null))throw Error('dependency manifest exact release/source/watermark/predecessor binding mismatch');
const plan={schemaVersion:'fair-launch-deployment-plan-v2',chainId:56,contractCommit:process.env.JC_FAIR_LAUNCH_CONTRACT_COMMIT,proxyAddress:process.env.JC_FAIR_LAUNCH_PROXY_ADDRESS,implementationAddress:process.env.JC_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS,operatorAddress:process.env.JC_FAIR_LAUNCH_OPERATOR_ADDRESS,governanceTimelockAddress:process.env.JC_FAIR_LAUNCH_GOVERNANCE_TIMELOCK_ADDRESS,abiDigest:process.env.JC_FAIR_LAUNCH_ABI_DIGEST,storageLayoutDigest:process.env.JC_FAIR_LAUNCH_STORAGE_LAYOUT_DIGEST,dependencyManifest:dependency,dependencyManifestDigest:dependencyDigest,deploymentAttestationDigest:process.env.JC_FAIR_LAUNCH_DEPLOYMENT_ATTESTATION_DIGEST,releaseAttestationDigest:dependency.releaseAttestationDigest,sourceRegistryRevision:dependency.sourceRegistry.revision,sourceRegistryDigest:dependency.sourceRegistry.digest,watermarkDependencyManifestDigest:dependencyDigest,releaseAdmissionDependencyManifestDigest:dependencyDigest,capabilityManifestDigest:process.env.JC_FAIR_LAUNCH_CAPABILITY_MANIFEST_DIGEST,changeTicket:process.env.JC_FAIR_LAUNCH_CHANGE_TICKET};
fs.writeFileSync(out,JSON.stringify(plan)+'\n');
NODE
sha256sum "$out" > "$out.sha256"
echo "offline plan written: $out"
