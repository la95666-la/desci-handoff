'use strict';

// TEST ONLY: persistent loopback Hardhat network for Fair Launch product E2E.
// In-process compile/tests require no environment variables. The external
// localE2E network exists only when its exact loopback identity is supplied;
// no default endpoint, private key or mnemonic is defined here.
require('@nomicfoundation/hardhat-toolbox');
require('@openzeppelin/hardhat-upgrades');

const FAIR_LAUNCH_SETTINGS = Object.freeze({
  optimizer: { enabled: true, runs: 200 },
  viaIR: true,
});

const fairLaunchOverride = () => ({
  version: '0.8.22',
  settings: FAIR_LAUNCH_SETTINGS,
});

function externalLocalNetwork() {
  const value = process.env.JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL;
  if (value === undefined) return {};
  let url;
  try { url = new URL(value); } catch { throw new Error('JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL must be exact http://127.0.0.1:<explicit-port>'); }
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !/^\d+$/.test(url.port) ||
      Number(url.port) < 1 || Number(url.port) > 65535 || url.pathname !== '/' ||
      url.username || url.password || url.search || url.hash || url.origin !== value) {
    throw new Error('JC_FAIR_LAUNCH_LOCAL_E2E_RPC_URL must be exact http://127.0.0.1:<explicit-port>');
  }
  return { localE2E: { url: url.origin, chainId: 31337 } };
}

module.exports = {
  solidity: {
    compilers: [
      { version: '0.8.30', settings: { optimizer: { enabled: true, runs: 200 }, viaIR: true } },
      { version: '0.8.22', settings: FAIR_LAUNCH_SETTINGS },
    ],
    overrides: Object.fromEntries([
      'contracts/JoinCareFairLaunchV1.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1Shared.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1CoreFacet.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1ReleaseFacet.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1FundsFacet.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1AdminFacet.sol',
      'contracts/fairlaunch/JoinCareFairLaunchV1ViewsFacet.sol',
      'contracts/libraries/JoinCareFairLaunchV1Logic.sol',
      'contracts/libraries/JoinCareFairLaunchV1Runtime.sol',
      'contracts/mocks/FairLaunchERC1967Proxy.sol',
      'contracts/mocks/FairLaunchTokenMock.sol',
      'contracts-local-e2e/FairLaunchSafeExecutor.sol',
      'contracts-local-e2e/FairLaunchLocalE2EImports.sol',
    ].map((source) => [source, fairLaunchOverride()])),
  },
  networks: {
    hardhat: {
      chainId: 31337,
      accounts: {
        count: 20,
        accountsBalance: '10000000000000000000000',
      },
    },
    ...externalLocalNetwork(),
  },
  paths: {
    sources: './contracts-local-e2e',
    tests: './test/local-e2e',
    artifacts: './artifacts-local-e2e',
    cache: './cache-local-e2e',
  },
  mocha: { timeout: 120000 },
};
