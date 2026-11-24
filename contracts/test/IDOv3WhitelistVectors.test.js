const { expect } = require('chai');
const { ethers } = require('hardhat');
const vectors = require('./fixtures/ido-v3-whitelist-v1.json');
const snapshots = require('./fixtures/ido-v3-snapshot-v1.json');
const { SCHEMA, SNAPSHOT_SCHEMA, buildTree, leaf, commitSnapshot } = require('../scripts/lib/idoV3Whitelist');

describe('IDO V3 whitelist golden vectors and input discipline', function () {
  it('commits to a complete canonical member snapshot, scope and effective version', () => {
    expect(SNAPSHOT_SCHEMA).to.equal(snapshots.snapshotSchema);
    vectors.cases.forEach((item, index) => {
      const actual = commitSnapshot(item.scope, item.wallets, '1');
      expect(actual).to.deep.equal(snapshots.cases[index]);
      expect(commitSnapshot(item.scope, [...item.wallets].reverse(), '1')).to.deep.equal(actual);
      expect(commitSnapshot(item.scope, item.wallets, '2').digest).not.to.equal(actual.digest);
      expect(commitSnapshot({ ...item.scope, projectId: '1' }, item.wallets, '1').digest)
        .not.to.equal(actual.digest);
    });
    expect(() => commitSnapshot(vectors.cases[0].scope, vectors.cases[0].wallets, '0'))
      .to.throw('invalid targetVersion');
    expect(() => commitSnapshot(vectors.cases[0].scope, vectors.cases[0].wallets, '18446744073709551616'))
      .to.throw('invalid targetVersion');
  });
  it('reconstructs every committed root and proof independently of wallet input order', () => {
    expect(SCHEMA).to.equal(vectors.schema);
    for (const item of vectors.cases) {
      const { scope, wallets } = item;
      const tree = buildTree(scope, wallets);
      const reversed = buildTree(scope, [...wallets].reverse());
      expect(tree.root).to.equal(item.root);
      expect(reversed.root).to.equal(tree.root);
      expect(tree.leaves.map(({ wallet, hash }) => ({ wallet, hash }))).to.deep.equal(item.leaves);
      for (const { wallet, hash, proof } of item.proofs) {
        expect(leaf(scope, wallet)).to.equal(hash);
        expect(tree.proofFor(wallet)).to.deep.equal(proof);
        expect(proof.reduce((current, sibling) => ethers.keccak256(ethers.concat(
          BigInt(current) < BigInt(sibling) ? [current, sibling] : [sibling, current],
        )), hash)).to.equal(tree.root);
      }
    }
  });

  it('rejects empty lists, duplicate wallet casing, invalid addresses and unsafe project IDs', () => {
    const scope = vectors.cases[0].scope;
    const buyer = vectors.cases[0].wallets[0];
    expect(() => buildTree(scope, [])).to.throw('invalid whitelist size');
    expect(() => buildTree(scope, [buyer, buyer.toLowerCase()])).to.throw('duplicate wallet');
    expect(() => buildTree(scope, [ethers.ZeroAddress])).to.throw('invalid wallet');
    expect(() => buildTree(scope, ['not-a-wallet'])).to.throw('invalid wallet');
    expect(() => buildTree({ ...scope, projectId: Number.MAX_SAFE_INTEGER + 1 }, [buyer]))
      .to.throw('invalid projectId');
    const tree = buildTree(scope, [buyer]);
    expect(() => tree.proofFor('0x0000000000000000000000000000000000000001'))
      .to.throw('wallet is not in the whitelist');
  });
});
