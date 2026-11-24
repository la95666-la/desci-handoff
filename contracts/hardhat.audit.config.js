require("@nomicfoundation/hardhat-toolbox");
require("@openzeppelin/hardhat-upgrades");

module.exports = {
  solidity: {
    version: "0.8.22",
    settings: {
      optimizer: { enabled: true, runs: 200 },
      viaIR: true,
      outputSelection: {
        "*": {
          "*": [
            "abi",
            "metadata",
            "evm.bytecode.object",
            "evm.deployedBytecode.object",
            "storageLayout"
          ],
          "": ["ast"]
        }
      }
    }
  },
  paths: {
    sources: "./contracts-audit",
    artifacts: "./artifacts-audit",
    cache: "./cache-audit",
    tests: "./test-audit"
  }
};
