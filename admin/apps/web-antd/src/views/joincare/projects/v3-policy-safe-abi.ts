// Exact V3 Proxy method for the Policy Safe Transaction Builder. The four
// parameters are read from an approved, live safe-handoff response, never from
// the editable candidate preview. No Safe transaction is created here.
export const v3PolicySafeAbi = JSON.stringify(
  [
    {
      type: 'function',
      name: 'setWhitelistPolicy',
      stateMutability: 'nonpayable',
      inputs: [
        { name: 'projectId', type: 'uint256' },
        { name: 'expectedVersion', type: 'uint64' },
        { name: 'enabled', type: 'bool' },
        { name: 'root', type: 'bytes32' },
      ],
      outputs: [],
    },
  ],
  null,
  2,
);
