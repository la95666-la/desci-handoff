import { requestClient } from '#/api/request';

export interface V3SafeProposal {
  safe: string;
  to: string;
  data: string;
  nonce: string;
  safe_tx_hash: string;
  sender?: string;
  signature?: string;
}

/** Only the connected Safe owner signs. Never send API keys or private keys to the browser. */
export async function signV3SafeProposal(proposal: V3SafeProposal) {
  const provider = (
    window as Window & {
      ethereum?: {
        request: (args: {
          method: string;
          params?: unknown[];
        }) => Promise<unknown>;
      };
    }
  ).ethereum;
  if (!provider || !/^0x[\da-fA-F]{64}$/.test(proposal.safe_tx_hash)) {
    throw new Error('Safe 签署钱包不可用');
  }
  const chain = await provider.request({ method: 'eth_chainId' });
  if (chain !== '0x38') throw new Error('请先切换到 BSC 主网');
  const accounts = await provider.request({ method: 'eth_requestAccounts' });
  if (!Array.isArray(accounts) || !/^0x[\da-fA-F]{40}$/.test(accounts[0])) {
    throw new Error('请连接 Safe Owner 钱包');
  }
  // personal_sign on the 32-byte Safe hash; server recovers this exact owner and
  // converts the wallet v to Safe's eth_sign mode. No arbitrary calldata signing.
  const signature = await provider.request({
    method: 'personal_sign',
    params: [proposal.safe_tx_hash, accounts[0]],
  });
  if (typeof signature !== 'string') throw new Error('签名未返回');
  return {
    safe_tx_hash: proposal.safe_tx_hash,
    sender: accounts[0],
    signature,
  };
}

export function previewV3PolicySafeProposal(
  projectId: string,
  reviewId: string,
) {
  return requestClient.get<V3SafeProposal>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}/safe-proposal`,
    { timeout: 170_000 },
  );
}

export function submitV3PolicySafeProposal(
  projectId: string,
  reviewId: string,
  signed: {
    safe_tx_hash: string;
    sender: string;
    signature: string;
  },
) {
  return requestClient.post<V3SafeProposal>(
    `/v1/projects/${projectId}/ido-v3/policy/reviews/${reviewId}/safe-proposal`,
    signed,
    { timeout: 170_000 },
  );
}

export function previewV3TokenSafeProposal(asset: 'testcat' | 'usdt') {
  return requestClient.get<V3SafeProposal>(
    `/v1/ido-v3/token-admission/${asset}/safe-proposal`,
    { timeout: 60_000 },
  );
}

export function previewV3ProjectTokenSafeProposal(address: string) {
  return requestClient.get<V3SafeProposal>(
    `/v1/ido-v3/project-tokens/${address}/safe-proposal`,
    { timeout: 95_000 },
  );
}

export function submitV3ProjectTokenSafeProposal(
  address: string,
  signed: {
    safe_tx_hash: string;
    sender: string;
    signature: string;
  },
) {
  return requestClient.post<V3SafeProposal>(
    `/v1/ido-v3/project-tokens/${address}/safe-proposal`,
    signed,
    { timeout: 95_000 },
  );
}

export function submitV3TokenSafeProposal(
  asset: 'testcat' | 'usdt',
  signed: {
    safe_tx_hash: string;
    sender: string;
    signature: string;
  },
) {
  return requestClient.post<V3SafeProposal>(
    `/v1/ido-v3/token-admission/${asset}/safe-proposal`,
    signed,
    { timeout: 60_000 },
  );
}
