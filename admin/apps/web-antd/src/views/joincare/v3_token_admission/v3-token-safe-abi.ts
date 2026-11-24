// These are methods on the fixed V3 Proxy, not on the ERC20 token itself.
// Values/nonce are always re-read from the current admission + Safe proposal.
const method = (name: string) =>
  JSON.stringify(
    [
      {
        type: 'function',
        name,
        stateMutability: 'nonpayable',
        inputs: [
          { name: 'token', type: 'address' },
          { name: 'allowed', type: 'bool' },
        ],
        outputs: [],
      },
    ],
    null,
    2,
  );

export const v3PaymentTokenSafeAbi = method('configurePaymentToken');
export const v3ProjectTokenSafeAbi = method('configureProjectToken');
