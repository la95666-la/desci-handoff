import type { ProjectListRow } from '#/api/joincare/project_content';

export type FundingMode = 'capped_presale' | 'fair_launch';
export type ProjectContractVersion =
  | 'fair_launch_v1'
  | 'ido_v3_stablecoin'
  | 'unassigned'
  | 'v1_legacy_unverified'
  | 'v2_stablecoin';

export type FundingContractPair =
  | 'capped_presale_v2'
  | 'capped_presale_v3'
  | 'fair_launch_v1'
  | 'historical_unassigned'
  | 'mismatch'
  | 'unknown';

const knownFundingModes = new Set<FundingMode>([
  'capped_presale',
  'fair_launch',
]);
const knownContractVersions = new Set<ProjectContractVersion>([
  'fair_launch_v1',
  'ido_v3_stablecoin',
  'unassigned',
  'v1_legacy_unverified',
  'v2_stablecoin',
]);

/**
 * Funding mode and contract version form one discriminated identity.
 * Never infer Fair Launch from either field independently.
 */
export function getFundingContractPair(
  row: Pick<ProjectListRow, 'contract_version' | 'funding_mode'>,
): FundingContractPair {
  const mode = row.funding_mode;
  const version = row.contract_version;

  if (mode === 'fair_launch' && version === 'fair_launch_v1') {
    return 'fair_launch_v1';
  }
  if (mode === 'capped_presale' && version === 'v2_stablecoin') {
    return 'capped_presale_v2';
  }
  // Recognize exact V3 identity for read-only admin visibility. The existing
  // V2 action predicates and ledger remain exclusive to capped_presale_v2.
  if (mode === 'capped_presale' && version === 'ido_v3_stablecoin') {
    return 'capped_presale_v3';
  }

  // Unassigned / legacy-unverified projects have no authoritative on-chain
  // contract identity yet. Their funding_mode can still be the schema-required
  // capped_presale placeholder, so classify them as historical content projects
  // rather than as a mode/contract mismatch. Lifecycle restoration remains gated
  // by Backend restore_allowed, archived source status and immutable-fact checks.
  if (version === 'unassigned' || version === 'v1_legacy_unverified') {
    return 'historical_unassigned';
  }

  // Compatibility for backend rows created before funding_mode was projected.
  // Historical v2 rows are defined by the frozen migration as capped_presale.
  if (mode === undefined && version === 'v2_stablecoin') {
    return 'capped_presale_v2';
  }

  const modeKnown = knownFundingModes.has(mode as FundingMode);
  const versionKnown = knownContractVersions.has(
    version as ProjectContractVersion,
  );
  if (modeKnown && versionKnown) {
    return 'mismatch';
  }
  return 'unknown';
}

export function isExactFairLaunchPair(
  row: Pick<ProjectListRow, 'contract_version' | 'funding_mode'>,
): boolean {
  return getFundingContractPair(row) === 'fair_launch_v1';
}

/** Legacy Fund Ledger/Reconciliation admits only the explicit capped-presale V2 identity. */
export function isLegacyV2FundLedgerSelectable(
  row: Pick<ProjectListRow, 'contract_version' | 'funding_mode'>,
): boolean {
  return (
    row.funding_mode === 'capped_presale' &&
    row.contract_version === 'v2_stablecoin'
  );
}

export function formatFinancialDecimal(
  value: null | string | undefined,
): string {
  if (typeof value !== 'string') return '未提供';
  const text = value.trim();
  return /^\d+(?:\.\d+)?$/.test(text) ? text : '无效十进制字符串';
}

export function fundingContractPairLabel(pair: FundingContractPair): string {
  const labels: Record<FundingContractPair, string> = {
    capped_presale_v2: 'Capped Presale · V2',
    capped_presale_v3: 'IDO V3 · Capped Presale',
    fair_launch_v1: 'Fair Launch · V1',
    historical_unassigned: '未分配',
    mismatch: '模式/合约不匹配',
    unknown: '未知模式/合约',
  };
  return labels[pair];
}
