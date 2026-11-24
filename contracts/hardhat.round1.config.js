'use strict';

// TEST ONLY: isolated in-process Fair Launch round-1 configuration.
// This file intentionally reads no environment variables and defines no RPC,
// deployer private key, localhost listener, testnet, or mainnet network.
require('@nomicfoundation/hardhat-toolbox');
require('@openzeppelin/hardhat-upgrades');

const FAIR_LAUNCH_SETTINGS = Object.freeze({
  optimizer: { enabled: true, runs: 200 },
  viaIR: true,
});

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
    ].map((source) => [source, { version: '0.8.22', settings: FAIR_LAUNCH_SETTINGS }])),
  },
  networks: {
    hardhat: {
      chainId: 31337,
      accounts: {
        count: 20,
        accountsBalance: '10000000000000000000000',
      },
    },
  },
  mocha: { timeout: 60000 },
};
