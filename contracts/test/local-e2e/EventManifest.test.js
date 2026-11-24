'use strict';

const { expect } = require('chai');
const fs = require('node:fs');
const path = require('node:path');
const { ethers } = require('hardhat');
const local = require('../../scripts/local/fairLaunchLocalE2E');

describe('generated frozen Fair Launch event manifest', function () {
  it('contains exactly 32 frozen structural signatures and indexed masks', function () {
    const generated = JSON.parse(fs.readFileSync(path.join(local.REPOSITORY_ROOT, 'scripts/validation/fair-launch-events.generated.json'), 'utf8'));
    const abi = JSON.parse(fs.readFileSync(path.join(local.REPOSITORY_ROOT, 'scripts/audit/golden/abi.golden.json'), 'utf8'));
    const events = abi.filter((entry) => entry.type === 'event' && entry.name.startsWith('FairLaunch'));
    expect(generated.schemaVersion).to.equal('fair-launch-event-signature-index-manifest-v1');
    expect(generated.status).to.equal(local.UNADMITTED_STATUS);
    expect(generated.sourceClassification).to.equal('GENERATED_FROM_REVIEWER_FROZEN_ABI');
    expect(generated.source).to.equal('scripts/audit/golden/abi.golden.json');
    expect(generated.input).to.deep.equal({
      path: generated.source,
      sha256: local.sha256(fs.readFileSync(path.join(local.REPOSITORY_ROOT, generated.source))),
      selection: 'type=event AND name startsWith FairLaunch',
      frozenEventCount: 32,
    });
    expect(generated.eventCount).to.equal(32);
    expect(generated.events).to.have.length(32);
    expect(new Set(generated.events.map((row) => row.name)).size).to.equal(32);
    expect(new Set(generated.events.map((row) => row.signature)).size).to.equal(32);
    expect(new Set(generated.events.map((row) => row.topic0)).size).to.equal(32);
    for (const entry of events) {
      const row = generated.events.find((candidate) => candidate.name === entry.name);
      const fragment = new ethers.Interface([entry]).getEvent(entry.name);
      expect(row, entry.name).to.exist;
      expect(row.signature).to.equal(fragment.format('sighash'));
      expect(row.topic0).to.equal(fragment.topicHash.toLowerCase());
      expect(row.indexedMask).to.deep.equal(entry.inputs.map((input) => input.indexed === true));
      expect(row.inputs).to.deep.equal(entry.inputs.map((input, ordinal) => ({ ordinal, name: input.name, type: input.type, indexed: input.indexed === true })));
    }
    const unsigned = { ...generated };
    delete unsigned.sha256;
    expect(generated.sha256).to.equal(local.sha256(local.canonicalJson(unsigned)));
  });
});
