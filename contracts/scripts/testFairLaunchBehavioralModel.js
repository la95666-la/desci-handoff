#!/usr/bin/env node
'use strict';

// Local-only launcher for the explicitly non-production behavioral model.
const { spawnSync } = require('node:child_process');
const path = require('node:path');

const hardhatCli = require.resolve('hardhat/internal/cli/cli.js');
const testFile = path.join('test', 'JoinCareFairLaunchV1BehavioralModel.test.js');
const result = spawnSync(process.execPath, [hardhatCli, 'test', testFile], {
  cwd: path.resolve(__dirname, '..'),
  env: { ...process.env, JC_FAIR_LAUNCH_MODEL_ONLY: '1' },
  stdio: 'inherit',
});

process.exit(result.status === null ? 1 : result.status);
