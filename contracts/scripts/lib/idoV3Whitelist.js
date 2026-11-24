// Deterministic IDO V3 Merkle encoding. This module has no RPC or signer.
// Go and browser implementations must match the committed vector fixtures.
'use strict';

const { AbiCoder, ZeroAddress, concat, getAddress, id, keccak256 } = require('ethers');

const SCHEMA = id('JOINCARE_IDO_V3_WHITELIST_LEAF_V1');
const coder = AbiCoder.defaultAbiCoder();
const MAX_WALLETS = 4096;

function positiveUint256(value, field) {
  let parsed;
  try {
    if (typeof value !== 'bigint' && typeof value !== 'string' && typeof value !== 'number') throw Error();
    if (typeof value === 'number' && !Number.isSafeInteger(value)) throw Error();
    if (typeof value === 'string' && !/^(0|[1-9]\d*)$/.test(value)) throw Error();
    parsed = BigInt(value);
  } catch { throw new Error(`invalid ${field}`); }
  if (parsed <= 0n || parsed >= 1n << 256n) throw new Error(`invalid ${field}`);
  return parsed;
}

function address(value, field) {
  if (typeof value !== 'string') throw new Error(`invalid ${field}`);
  let normalized;
  try { normalized = getAddress(value); } catch { throw new Error(`invalid ${field}`); }
  if (normalized === ZeroAddress) throw new Error(`invalid ${field}`);
  return normalized;
}

function identity(chainId, proxyAddress, projectId) {
  return {
    chainId: positiveUint256(chainId, 'chainId'),
    proxyAddress: address(proxyAddress, 'proxyAddress'),
    projectId: positiveUint256(projectId, 'projectId'),
  };
}

function leaf(scopedIdentity, wallet) {
  const buyer = address(wallet, 'wallet');
  const scope = identity(scopedIdentity.chainId, scopedIdentity.proxyAddress, scopedIdentity.projectId);
  return keccak256(coder.encode(
    ['bytes32', 'uint256', 'address', 'uint256', 'address'],
    [SCHEMA, scope.chainId, scope.proxyAddress, scope.projectId, buyer],
  ));
}

function sortedPair(left, right) {
  return keccak256(concat(BigInt(left) < BigInt(right) ? [left, right] : [right, left]));
}

/**
 * Leaves sorted by their 32-byte hash (ascending). Adjacent pairs use OZ's
 * commutative sorted hash. An odd final node is promoted unchanged, never
 * duplicated. No empty root is valid for an enabled policy.
 */
function buildTree(scopedIdentity, wallets) {
  const scope = identity(scopedIdentity.chainId, scopedIdentity.proxyAddress, scopedIdentity.projectId);
  if (!Array.isArray(wallets) || wallets.length === 0 || wallets.length > MAX_WALLETS) {
    throw new Error('invalid whitelist size');
  }
  const seen = new Set();
  const entries = wallets.map((wallet) => {
    const normalized = address(wallet, 'wallet');
    const key = normalized.toLowerCase();
    if (seen.has(key)) throw new Error('duplicate wallet');
    seen.add(key);
    return { wallet: normalized, hash: leaf(scope, normalized) };
  }).sort((a, b) => a.hash.localeCompare(b.hash));

  const levels = [entries.map((entry) => entry.hash)];
  while (levels.at(-1).length > 1) {
    const current = levels.at(-1);
    const next = [];
    for (let index = 0; index < current.length; index += 2) {
      next.push(index + 1 < current.length
        ? sortedPair(current[index], current[index + 1])
        : current[index]);
    }
    levels.push(next);
  }

  return {
    root: levels.at(-1)[0],
    leaves: entries,
    proofFor(wallet) {
      const buyer = address(wallet, 'wallet').toLowerCase();
      let index = entries.findIndex((entry) => entry.wallet.toLowerCase() === buyer);
      if (index < 0) throw new Error('wallet is not in the whitelist');
      const proof = [];
      for (let depth = 0; depth < levels.length - 1; depth++) {
        const sibling = index ^ 1;
        if (sibling < levels[depth].length) proof.push(levels[depth][sibling]);
        index = Math.floor(index / 2);
      }
      return proof;
    },
  };
}

// Offline candidate snapshot digest. It is evidence for later immutable
// approval, not a signature or proof of on-chain execution/finality.
const SNAPSHOT_SCHEMA = id('JOINCARE_IDO_V3_WHITELIST_SNAPSHOT_V1');
function commitSnapshot(scopedIdentity, wallets, targetVersion) {
  const scope = identity(scopedIdentity.chainId, scopedIdentity.proxyAddress, scopedIdentity.projectId);
  const version = positiveUint256(targetVersion, 'targetVersion');
  if (version > (1n << 64n) - 1n) throw new Error('invalid targetVersion');
  const tree = buildTree(scope, wallets);
  const membersDigest = keccak256(concat(tree.leaves.map(({ hash }) => hash)));
  return {
    root: tree.root,
    membersDigest,
    digest: keccak256(coder.encode(
      ['bytes32', 'uint256', 'address', 'uint256', 'uint256', 'bytes32', 'bytes32', 'uint256'],
      [SNAPSHOT_SCHEMA, scope.chainId, scope.proxyAddress, scope.projectId, version,
        tree.root, membersDigest, BigInt(tree.leaves.length)],
    )),
    targetVersion: version.toString(),
    memberCount: tree.leaves.length,
  };
}

module.exports = { SCHEMA, SNAPSHOT_SCHEMA, MAX_WALLETS, identity, leaf, buildTree, commitSnapshot };
