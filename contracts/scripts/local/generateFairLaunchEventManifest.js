'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { ethers } = require('ethers');
const local = require('./fairLaunchLocalE2E');

const ABI_PATH = path.join(local.REPOSITORY_ROOT, 'scripts/audit/golden/abi.golden.json');
const OUTPUT_PATH = path.join(local.REPOSITORY_ROOT, 'scripts/validation/fair-launch-events.generated.json');
const FROZEN_EVENT_COUNT = 32;

function main() {
  const abi = JSON.parse(fs.readFileSync(ABI_PATH, 'utf8'));
  const events = abi.filter((entry) => entry.type === 'event' && entry.name.startsWith('FairLaunch'));
  if (events.length !== FROZEN_EVENT_COUNT) throw new Error(`expected ${FROZEN_EVENT_COUNT} frozen Fair Launch events, got ${events.length}`);
  const rows = events.map((entry) => {
    const iface = new ethers.Interface([entry]);
    const fragment = iface.getEvent(entry.name);
    return {
      name: entry.name,
      signature: fragment.format('sighash'),
      topic0: fragment.topicHash.toLowerCase(),
      indexedMask: entry.inputs.map((input) => input.indexed === true),
      inputs: entry.inputs.map((input, index) => ({ ordinal: index, name: input.name, type: input.type, indexed: input.indexed === true })),
    };
  }).sort((a, b) => a.name.localeCompare(b.name));
  const payload = {
    schemaVersion: 'fair-launch-event-signature-index-manifest-v1',
    status: local.UNADMITTED_STATUS,
    sourceClassification: 'GENERATED_FROM_REVIEWER_FROZEN_ABI',
    source: 'scripts/audit/golden/abi.golden.json',
    input: {
      path: 'scripts/audit/golden/abi.golden.json',
      sha256: local.sha256(fs.readFileSync(ABI_PATH)),
      selection: 'type=event AND name startsWith FairLaunch',
      frozenEventCount: FROZEN_EVENT_COUNT,
    },
    eventCount: rows.length,
    events: rows,
  };
  payload.sha256 = local.sha256(local.canonicalJson(payload));
  fs.writeFileSync(OUTPUT_PATH, `${JSON.stringify(payload, null, 2)}\n`, 'utf8');
  console.log(`${rows.length} events -> ${OUTPUT_PATH}`);
}

main();
