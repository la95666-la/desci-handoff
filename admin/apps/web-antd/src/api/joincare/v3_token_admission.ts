import { requestClient } from '#/api/request';

export type V3AdmissionAssetName = 'testcat' | 'usdt';

export interface V3AdmissionAsset {
  symbol: string;
  token: string;
  method: string;
  allowed: boolean;
  decimals: number;
  simulation: 'not_requested' | 'passed';
}

export interface V3AdmissionStatus {
  chain_id: number;
  proxy: string;
  owner_safe: string;
  safe_threshold: number;
  safe_nonce: string;
  block: number;
  block_hash: string;
  confirmations: number;
  usdt: V3AdmissionAsset;
  testcat: V3AdmissionAsset;
}

export interface V3AdmissionReceipt {
  asset: V3AdmissionAssetName;
  tx_hash: string;
  block: number;
  block_hash: string;
  confirmations: number;
  allowed: boolean;
  decimals: number;
}

export interface V3ProjectTokenStatus {
  chain_id: number;
  proxy: string;
  owner_safe: string;
  block: number;
  block_hash: string;
  confirmations: number;
  asset: V3AdmissionAsset;
}

export function readV3ProjectToken(address: string) {
  return requestClient.get<V3ProjectTokenStatus>(
    `/v1/ido-v3/project-tokens/${address}`,
    { timeout: 55_000 },
  );
}

/** Enqueues a four-eyes review of the fixed V3 deployment; never registers from the browser. */
export function submitV3RuntimeRegistration() {
  return requestClient.post<{ approval_id: number; status: string }>(
    '/v1/ido-v3/runtime-registration',
    {},
    { timeout: 65_000 },
  );
}

export function verifyV3ProjectTokenReceipt(address: string, txHash: string) {
  return requestClient.get<V3AdmissionReceipt>(
    `/v1/ido-v3/project-tokens/${address}/receipt`,
    { params: { tx_hash: txHash }, timeout: 45_000 },
  );
}

export function readV3TokenAdmission() {
  // The backend checks the approved commercial RPC at a confirmed block with a 25s deadline.
  // Only these read-only V3 checks need more than the global 10s API timeout.
  return requestClient.get<V3AdmissionStatus>('/v1/ido-v3/token-admission', {
    timeout: 35_000,
  });
}

export function verifyV3TokenAdmissionReceipt(
  asset: V3AdmissionAssetName,
  txHash: string,
) {
  return requestClient.get<V3AdmissionReceipt>(
    `/v1/ido-v3/token-admission/${asset}/receipt`,
    { params: { tx_hash: txHash }, timeout: 35_000 },
  );
}
