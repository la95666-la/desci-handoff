#!/usr/bin/env bash
set -euo pipefail
fl_die(){ echo "fair-launch-gate: $*" >&2; exit 1; }
fl_require_file(){ [ -f "$1" ] || fl_die "missing $2: $1"; }
fl_require_hex64(){ [[ ${!1:-} =~ ^[0-9a-f]{64}$ ]] || fl_die "$1 must be lowercase 64-hex"; }
fl_require_address(){ [[ ${!1:-} =~ ^0x[0-9a-fA-F]{40}$ ]] || fl_die "$1 must be a 20-byte address"; }
fl_load_dependency_manifest(){
  : "${JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST:?canonical dependency manifest path required}"
  fl_require_file "$JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST" 'dependency manifest'
  local verified
  verified=$(node "$ROOT/scripts/fair-launch/verify-dependency-manifest.js" "$JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST" --json --require-admission) || fl_die 'dependency manifest verification failed'
  JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_DIGEST=$(node -e 'const v=JSON.parse(process.argv[1]);process.stdout.write(v.digest)' "$verified")
  export JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_DIGEST
  JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_CANONICAL=$(node -e 'const fs=require("fs");const {verify}=require(process.argv[2]);process.stdout.write(verify(process.argv[1]).canonical)' "$JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST" "$ROOT/scripts/fair-launch/verify-dependency-manifest.js")
  fl_require_hex64 JC_FAIR_LAUNCH_DEPENDENCY_MANIFEST_DIGEST
}
fl_require_deployment_gate(){
  [ -z "${JC_FAIR_LAUNCH_TEST_AUTHORITY+x}" ] || fl_die 'JC_FAIR_LAUNCH_TEST_AUTHORITY is forbidden in production entrypoints'
  [ "${JC_FAIR_LAUNCH_DEPLOYMENT_APPROVED:-0}" = 1 ] || fl_die 'deployment approval flag is not 1'
  [ "${JC_FAIR_LAUNCH_CHAIN_ID:-0}" = 56 ] || fl_die 'chainId must be 56'
  for k in JC_FAIR_LAUNCH_PROXY_ADDRESS JC_FAIR_LAUNCH_IMPLEMENTATION_ADDRESS JC_FAIR_LAUNCH_OPERATOR_ADDRESS JC_FAIR_LAUNCH_GOVERNANCE_TIMELOCK_ADDRESS; do fl_require_address "$k"; done
  for k in JC_FAIR_LAUNCH_DEPLOYMENT_ATTESTATION_DIGEST JC_FAIR_LAUNCH_CAPABILITY_MANIFEST_DIGEST JC_FAIR_LAUNCH_SOURCE_REGISTRY_DIGEST; do fl_require_hex64 "$k"; done
  fl_load_dependency_manifest
  [ "${JC_FAIR_LAUNCH_SOURCE_REGISTRY_REVISION:-0}" -gt 0 ] || fl_die 'source registry revision must be positive'
  [ "${JC_FAIR_LAUNCH_WATERMARK_GENERATION:-0}" -gt 0 ] || fl_die 'watermark generation must be positive'
  : "${JC_FAIR_LAUNCH_DEPENDENCY_AUTHORITY_FILE:?repo-outside dependency authority required}"
  : "${JC_FAIR_LAUNCH_MONOTONIC_STATE_FILE:?repo-outside monotonic state required}"
  [ "${JC_FAIR_LAUNCH_START_BLOCK:-0}" -gt 0 ] || fl_die 'start block must be positive'
  [ -n "${JC_FAIR_LAUNCH_CHANGE_TICKET:-}" ] || fl_die 'change ticket is required'
  [ -n "${JC_FAIR_LAUNCH_APPROVED_BY:-}" ] || fl_die 'approval identity is required'
  [ -n "${JC_FAIR_LAUNCH_SIGN_POLICY_GATEWAY_URL:-}" ] || fl_die 'Sign Policy Gateway URL required'
  [ -n "${JC_FAIR_LAUNCH_WEB3SIGNER_URL:-}" ] || fl_die 'Web3Signer URL required'
}
