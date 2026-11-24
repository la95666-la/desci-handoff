import type {
  OnchainParamsCurrent,
  ProjectListRow,
} from '#/api/joincare/project_content';

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import dayjs from 'dayjs';
import { describe, expect, it } from 'vitest';

import {
  v3PaymentTokenSafeAbi,
  v3ProjectTokenSafeAbi,
} from '../../v3_token_admission/v3-token-safe-abi';
import {
  formatFinancialDecimal,
  fundingContractPairLabel,
  getFundingContractPair,
  isExactFairLaunchPair,
  isLegacyV2FundLedgerSelectable,
} from '../funding-mode';
import { formatOnchainTime } from '../onchain-time';
import {
  isActivateOnchainAllowed,
  isCancelOnchainAllowed,
  isCreateOnchainAllowed,
  isDeleteAllowed,
  isDisableAllowed,
  isEndOnchainAllowed,
  isLifecycleMutationBlocked,
  isMarketingContentEditAllowed,
  isOnchainParamsViewAllowed,
  isPlatformFeeWithdrawable,
  isRestorable,
} from '../project-predicates';
import { isV3PolicyPreviewProjectCreated } from '../v3-policy-eligibility';
import { v3PolicySafeAbi } from '../v3-policy-safe-abi';

const contentEditorSource = () =>
  readFileSync(
    resolve(
      process.cwd(),
      'apps/web-antd/src/views/joincare/projects/content_editor.vue',
    ),
    'utf8',
  );

const projectViewSource = (name: string) =>
  readFileSync(
    resolve(process.cwd(), `apps/web-antd/src/views/joincare/projects/${name}`),
    'utf8',
  );

function row(overrides: Partial<ProjectListRow>): ProjectListRow {
  return {
    id: 1,
    project_id: '101',
    name_en: 'Demo',
    name_cn: '',
    tagline: '',
    category: '',
    status: 'Set',
    funding_mode: undefined,
    contract_version: 'unassigned',
    onchain_locked: false,
    launch_at: null,
    cover_url: '',
    created_at: '',
    updated_at: '',
    ...overrides,
  };
}

const capped = (overrides: Partial<ProjectListRow> = {}) =>
  row({
    contract_version: 'v2_stablecoin',
    funding_mode: 'capped_presale',
    ...overrides,
  });

const fair = (overrides: Partial<ProjectListRow> = {}) =>
  row({
    contract_version: 'fair_launch_v1',
    funding_mode: 'fair_launch',
    ...overrides,
  });

const v3 = (overrides: Partial<ProjectListRow> = {}) =>
  row({
    contract_version: 'ido_v3_stablecoin',
    funding_mode: 'capped_presale',
    ...overrides,
  });

describe('funding mode / contract version exact pair', () => {
  it('keeps the saved V2 parameters viewable when locked, without opening Fair Launch or unknown identities', () => {
    expect(
      isOnchainParamsViewAllowed(
        capped({ onchain_locked: true, status: 'Successful' }),
      ),
    ).toBe(true);
    expect(isOnchainParamsViewAllowed(capped({ onchain_locked: false }))).toBe(
      true,
    );
    expect(
      isOnchainParamsViewAllowed(
        row({ funding_mode: 'capped_presale', contract_version: 'unassigned' }),
      ),
    ).toBe(true);
    expect(isOnchainParamsViewAllowed(fair({ onchain_locked: true }))).toBe(
      false,
    );
    expect(
      isOnchainParamsViewAllowed(
        row({ funding_mode: 'fair_launch', contract_version: 'v2_stablecoin' }),
      ),
    ).toBe(false);
  });

  it('does not couple the parameter view button to the on-chain lock and keeps the locked form read-only', () => {
    const list = projectViewSource('list.vue');
    const form = projectViewSource('onchain_params.vue');
    expect(list).toContain(
      '!isOnchainParamsViewAllowed(record as ProjectListRow)',
    );
    expect(form).toContain('<Form layout="vertical" :disabled="locked">');
    expect(form).toContain(':disabled="!canPreview || locked || !permissionChecked || !canWriteParams"');
    expect(form).toContain(':disabled="!preview || locked || !permissionChecked || !canWriteParams"');
    expect(form).toContain("access.permissions.includes('m2_project:write')");
    expect(form.match(/await refreshWritePermission\(\)/g)?.length).toBe(2);
    expect(form).toContain('const canWriteParams = ref(false)');
    expect(form).toContain('参数预览未完成');
    expect(form).toContain('locked.value = true;\n    loadError.value = true;');
  });

  it('requires an explicit V3 identity in the Set form and never treats a V3 Set as V2', () => {
    const form = projectViewSource('onchain_params.vue');
    expect(form).toContain(
      'start_time: formatOnchainTime(startTimeD.value, isV3.value)',
    );
    expect(form).toContain(
      'end_time: formatOnchainTime(endTimeD.value, isV3.value)',
    );
    expect(form).toContain(
      "isV3.value ? { contract_version: 'ido_v3_stablecoin' as const } : {}",
    );
    expect(form).toMatch(
      /currentVersion\.value === 'unassigned'\s*&&\s*currentFundingMode\.value === 'capped_presale'\s*&&\s*currentStatus\.value === 'Setup'/,
    );
    expect(form).toContain("cur.contract_version === 'ido_v3_stablecoin'");
    expect(form).toContain('<SelectOption value="ido_v3_stablecoin">');
    expect(form).toContain('<SelectOption v-if="!isV3" value="USDC">');
  });

  it('formats V3 preview and approval at exact seconds without changing V2 timestamps', () => {
    const input = dayjs('2030-01-01T07:00:53.586Z');
    expect(formatOnchainTime(input, true)).toBe('2030-01-01T07:00:53.000Z');
    expect(formatOnchainTime(input, false)).toBe('2030-01-01T07:00:53.586Z');
    expect(formatOnchainTime(undefined, true)).toBe('');
    expect(input.millisecond()).toBe(586);
  });

  it('requires an approved Safe handoff and an owner signature for API proposal', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    expect(policy).toContain('await getOnchainParams(projectId)');
    expect(policy).toContain('isV3PolicyPreviewProjectCreated(current)');
    expect(policy).toContain('请先创建 V3 项目');
    expect(policy).toContain(':disabled="!canWritePolicy || !projectCreated"');
    expect(policy).toContain(
      'getV3PolicySafeHandoff(projectId, reviewID.value)',
    );
    expect(policy).toContain('safeHandoff.preview_digest');
    expect(policy).toContain('精确 calldata');
    expect(policy).toContain('signV3SafeProposal(safeProposal.value)');
    expect(policy).toContain(
      'submitV3PolicySafeProposal(projectId, reviewID.value, signed)',
    );
    expect(policy).toContain('transactionBuilderUrl');
  });

  it('shows exact approved handoff fields and the V3 Proxy ABI without constructing calldata from a draft', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    const [method] = JSON.parse(v3PolicySafeAbi);
    expect(method).toMatchObject({
      name: 'setWhitelistPolicy',
      stateMutability: 'nonpayable',
      type: 'function',
      inputs: [
        { name: 'projectId', type: 'uint256' },
        { name: 'expectedVersion', type: 'uint64' },
        { name: 'enabled', type: 'bool' },
        { name: 'root', type: 'bytes32' },
      ],
      outputs: [],
    });
    expect(policy).toContain('title="第 1 步 · 核对只读候选"');
    expect(policy).toContain(
      'title="第 2 步 · 不可变工单详情 / 第二位管理员核对"',
    );
    expect(policy).toContain('title="第 3 步 · 已审批 Policy Safe 交接"');
    expect(policy).toContain('title="第 4 步 · Safe 执行后核验回执"');
    expect(policy).toContain('handoffMatchesReview');
    for (const field of [
      'governor',
      'proxy',
      'project_id',
      'expected_version',
      'enabled',
      'root',
      'calldata',
      'preview_digest',
    ]) {
      expect(policy).toContain(`safeHandoff.${field}`);
    }
    expect(policy).toContain('v3PolicySafeAbi');
    expect(policy).toContain('Builder 编码出的完整 Data');
    expect(policy).not.toContain('result.calldata');
  });

  it('discovers V3 policy work items without hand-entered IDs or premature Safe actions', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    expect(policy).toContain('listV3PolicyReviews(projectId)');
    expect(policy).toContain('inspectReview(record.review_id)');
    expect(policy).toContain(
      'selectedReview.value.initiator_admin_id !== currentAdminID.value',
    );
    expect(policy).toContain('v-if="approvedSelected"');
    expect(policy).toMatch(
      /v-if="\s*approvedSelected\s*&&\s*!policyMayHaveExecuted\s*&&\s*safeHandoff\s*&&\s*handoffMatchesReview\s*"/,
    );
    expect(policy).toContain('若链上已执行，请直接进行第 4 步原交易回执核验');
    expect(policy).toContain('Modal.confirm({');
    expect(policy).not.toContain('placeholder="独立候选 ID（十进制）"');
  });

  it('shows the V3 project review entrance to an approver without write authority', () => {
    const list = projectViewSource('list.vue');
    const policy = projectViewSource('v3_policy_preview.vue');
    expect(list).toContain("result.permissions.includes('m2_project:approve')");
    expect(list).toContain('(canWrite || canApprove) &&');
    expect(list).toContain("'capped_presale_v3'");
    expect(list).toContain('V3 名单治理/审批');
    expect(policy).toContain(
      "access.permissions.includes('m2_project:approve')",
    );
    expect(policy).toContain('canApprovePolicy.value &&');
    expect(policy).toContain('v-if="!canWritePolicy && canApprovePolicy"');
    expect(policy).toContain('v-if="canWritePolicy && result"');
    expect(policy).toContain(
      'v-if="selectedReview?.status === \'pending_second_admin\'"',
    );
  });

  it('uses two logged-in Admins for V3 review without external MFA', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    const api = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/api/joincare/project_content.ts',
      ),
      'utf8',
    );
    expect(policy).toContain('await startV3PolicyReview(projectId, {');
    expect(policy).toContain('await approveV3PolicyReview(');
    expect(policy).not.toContain('initiatorMFA');
    expect(policy).not.toContain('approverMFA');
    expect(
      api.slice(
        api.indexOf('export function startV3PolicyReview('),
        api.indexOf('export function observeV3PolicyExecution('),
      ),
    ).not.toContain('mfa_assertion_token');
  });

  it('sets bounded timeouts for every V3 policy step and reconciles ambiguous writes', () => {
    const api = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/api/joincare/project_content.ts',
      ),
      'utf8',
    );
    const safe = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/api/joincare/v3_safe_proposal.ts',
      ),
      'utf8',
    );
    const policy = projectViewSource('v3_policy_preview.vue');
    const scoped = api.slice(
      api.indexOf('export function startV3PolicyReview('),
      api.indexOf('export function previewOnchainParams('),
    );
    expect(scoped.match(/timeout: \d[\d_]+/g)).toHaveLength(6);
    expect(scoped.match(/timeout: 170_000/g)).toHaveLength(6);
    expect(
      safe.slice(
        safe.indexOf('export function previewV3PolicySafeProposal('),
        safe.indexOf('export function previewV3TokenSafeProposal('),
      ),
    ).toContain('timeout: 170_000');
    expect(policy).toContain('mutationUnknown.value = true');
    expect(policy).toContain('unknownDigest.value = digest');
    expect(policy).toContain('listV3PolicyReviews(projectId)');
  });

  it('explains the V3-only missing migration without claiming a timed-out write failed', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    expect(policy).toContain('V3_POLICY_REVIEW_SCHEMA_MIGRATION_REQUIRED');
    expect(policy).toContain('mutationUnknown.value = true');
    expect(policy).toContain('先刷新工单列表核对同一摘要');
  });

  it('labels the two externally observed Safe hashes and rejects a known reversed handoff', () => {
    const policy = projectViewSource('v3_policy_preview.vue');
    expect(policy).toContain(
      'safeProposal.value?.safe_tx_hash?.toLowerCase() === txHash',
    );
    expect(policy).toContain('Transaction Hash');
    expect(policy).toContain('Safe Tx Hash');
    expect(policy).toContain('不要重新执行交易');
  });

  it('keeps V3 governance preview closed before canonical creation', () => {
    const current = {
      contract_version: 'ido_v3_stablecoin',
      funding_mode: 'capped_presale',
      locked: false,
      status: 'Set',
    } as OnchainParamsCurrent;
    expect(isV3PolicyPreviewProjectCreated(current)).toBe(false);
    expect(isV3PolicyPreviewProjectCreated({ ...current, locked: true })).toBe(
      false,
    );
    expect(
      isV3PolicyPreviewProjectCreated({ ...current, status: 'Pending' }),
    ).toBe(false);
    expect(
      isV3PolicyPreviewProjectCreated({
        ...current,
        locked: true,
        status: 'Pending',
      }),
    ).toBe(true);
    expect(
      isV3PolicyPreviewProjectCreated({
        ...current,
        locked: true,
        status: 'Active',
      }),
    ).toBe(true);
    expect(
      isV3PolicyPreviewProjectCreated({
        ...current,
        locked: true,
        status: 'Successful',
      }),
    ).toBe(false);
    expect(
      isV3PolicyPreviewProjectCreated({
        ...current,
        locked: true,
        status: 'Pending',
        funding_mode: 'fair_launch',
      }),
    ).toBe(false);
  });

  it('keeps new V3 token proposals owner-signed and separate from Set admission', () => {
    const tokens = projectViewSource('../v3_token_admission/index.vue');
    expect(tokens).toContain('readV3ProjectToken(address)');
    expect(tokens).toContain('signV3SafeProposal(proposal)');
    expect(tokens).toContain('verifyV3ProjectTokenReceipt(');
    expect(tokens).toContain('支付币仍仅为 USDT');
    expect(tokens).toContain('submitV3RuntimeRegistration()');
    expect(tokens).toContain('此步骤不批准任何项目 Set');
    expect(tokens).toContain('精确 calldata');
    expect(tokens).toContain('Owner Safe');
    expect(tokens).toContain('Transaction Builder');
    expect(tokens).toContain('v3ProjectTokenSafeAbi');
    expect(tokens).toContain('v3PaymentTokenSafeAbi');
    for (const [abi, name] of [
      [v3PaymentTokenSafeAbi, 'configurePaymentToken'],
      [v3ProjectTokenSafeAbi, 'configureProjectToken'],
    ] as const) {
      const [method] = JSON.parse(abi);
      expect(method).toMatchObject({
        type: 'function',
        name,
        stateMutability: 'nonpayable',
        inputs: [
          { name: 'token', type: 'address' },
          { name: 'allowed', type: 'bool' },
        ],
      });
    }
  });

  it('exports a V3 Set review draft without claiming it is signed or submitted', () => {
    const form = projectViewSource('onchain_params.vue');
    expect(form).toContain('OFFLINE REVIEW DRAFT ONLY; NOT V3 SET ADMISSION');
    expect(form).toContain('downloadV3Draft');
    expect(form).toContain(
      'previewedRequest.value !== JSON.stringify(requestBody())',
    );
  });
  it('recognizes exact V3 read identity without making it a V2 financial route', () => {
    expect(getFundingContractPair(v3())).toBe('capped_presale_v3');
    expect(fundingContractPairLabel('capped_presale_v3')).toContain('IDO V3');
    expect(isLegacyV2FundLedgerSelectable(v3())).toBe(false);
    expect(
      getFundingContractPair(
        row({ contract_version: 'ido_v3_stablecoin', funding_mode: undefined }),
      ),
    ).toBe('unknown');
    expect(
      getFundingContractPair(
        row({
          contract_version: 'ido_v3_stablecoin',
          funding_mode: 'fair_launch',
        }),
      ),
    ).toBe('mismatch');
  });

  it('accepts only the existing writable pairs', () => {
    expect(getFundingContractPair(capped())).toBe('capped_presale_v2');
    expect(getFundingContractPair(fair())).toBe('fair_launch_v1');
    expect(isExactFairLaunchPair(fair())).toBe(true);
    expect(isLegacyV2FundLedgerSelectable(capped())).toBe(true);
    expect(isLegacyV2FundLedgerSelectable(fair())).toBe(false);
    expect(
      isLegacyV2FundLedgerSelectable(
        row({ funding_mode: 'fair_launch', contract_version: 'v2_stablecoin' }),
      ),
    ).toBe(false);
    expect(
      isLegacyV2FundLedgerSelectable(
        row({ funding_mode: undefined, contract_version: 'v2_stablecoin' }),
      ),
    ).toBe(false);
    expect(fundingContractPairLabel('fair_launch_v1')).toContain('Fair Launch');
    expect(formatFinancialDecimal('9007199254740993.000000000000000001')).toBe(
      '9007199254740993.000000000000000001',
    );
    expect(formatFinancialDecimal('1e18')).toBe('无效十进制字符串');
  });

  it('treats capped_presale as a schema placeholder for unassigned drafts', () => {
    expect(
      getFundingContractPair(
        row({
          contract_version: 'unassigned',
          funding_mode: 'capped_presale',
        }),
      ),
    ).toBe('historical_unassigned');
    expect(
      isRestorable(
        row({
          contract_version: 'unassigned',
          funding_mode: 'capped_presale',
          pre_archive_status: 'pre_launch',
          restore_allowed: true,
          status: 'archived',
        }),
      ),
    ).toBe(true);
  });

  it('fails closed for mismatch and unknown values', () => {
    expect(
      getFundingContractPair(
        row({ funding_mode: 'fair_launch', contract_version: 'v2_stablecoin' }),
      ),
    ).toBe('mismatch');
    expect(
      getFundingContractPair(
        row({
          funding_mode: 'capped_presale',
          contract_version: 'fair_launch_v1',
        }),
      ),
    ).toBe('mismatch');
    expect(
      getFundingContractPair(
        row({ funding_mode: 'future_mode', contract_version: 'future_v9' }),
      ),
    ).toBe('unknown');
    expect(
      getFundingContractPair(
        row({ contract_version: 'v2_stablecoin', funding_mode: undefined }),
      ),
    ).toBe('capped_presale_v2');
  });
});

describe('existing v2_stablecoin production predicates', () => {
  it('exposes supported V3 lifecycle actions but keeps Owner Safe cancel and content restore closed', () => {
    const active = v3({
      status: 'Active',
      onchain_locked: true,
      end_time: '2000-01-01T00:00:00Z',
    });
    expect(isLifecycleMutationBlocked(v3())).toBe(false);
    expect(isOnchainParamsViewAllowed(v3())).toBe(true);
    expect(isCreateOnchainAllowed(v3({ status: 'Set' }))).toBe(true);
    expect(
      isCreateOnchainAllowed(v3({ status: 'Set', onchain_locked: true })),
    ).toBe(false);
    expect(
      isCreateOnchainAllowed(
        v3({
          status: 'Set',
          active_submission: {
            id: 1,
            action: 'ido_v3_create',
            state: 'signed',
          },
        }),
      ),
    ).toBe(false);
    expect(
      isActivateOnchainAllowed(v3({ status: 'Pending', onchain_locked: true })),
    ).toBe(true);
    expect(isEndOnchainAllowed(active)).toBe(true);
    expect(isCancelOnchainAllowed(active)).toBe(false);
    expect(
      isPlatformFeeWithdrawable(
        v3({ status: 'Successful', onchain_locked: true }),
      ),
    ).toBe(true);
    // Backend derives V3 withdrawal from canonical events, even when the
    // executed fee amount is zero. Never offer another withdrawal afterward.
    for (const withdrawn of [1]) {
      expect(
        isPlatformFeeWithdrawable(
          v3({
            status: 'Successful',
            onchain_locked: true,
            platform_fee_withdrawn: withdrawn,
          }),
        ),
      ).toBe(false);
    }
    expect(
      isRestorable(
        v3({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          restore_allowed: true,
        }),
      ),
    ).toBe(false);
  });
  it('preserves exact capped presale create and activate behavior', () => {
    expect(isCreateOnchainAllowed(capped({ status: 'Set' }))).toBe(true);
    expect(
      isActivateOnchainAllowed(
        capped({ status: 'Pending', onchain_locked: true }),
      ),
    ).toBe(true);
  });

  it('centralizes the read-only boundary for Fair Launch, mismatch and unknown', () => {
    expect(isLifecycleMutationBlocked(fair())).toBe(true);
    expect(
      isLifecycleMutationBlocked(
        row({ funding_mode: 'fair_launch', contract_version: 'v2_stablecoin' }),
      ),
    ).toBe(true);
    expect(
      isLifecycleMutationBlocked(
        row({ funding_mode: 'future_mode', contract_version: 'future_v9' }),
      ),
    ).toBe(true);
    expect(isLifecycleMutationBlocked(capped())).toBe(false);
    expect(isMarketingContentEditAllowed(fair())).toBe(true);
  });

  it('rejects Fair Launch and mismatched identities for every chain action', () => {
    const fairActive = fair({
      status: 'Active',
      onchain_locked: true,
      end_time: '2000-01-01T00:00:00Z',
    });
    const mismatch = row({
      funding_mode: 'fair_launch',
      contract_version: 'v2_stablecoin',
      status: 'Successful',
      onchain_locked: true,
    });

    expect(isCreateOnchainAllowed(fair({ status: 'Set' }))).toBe(false);
    expect(
      isActivateOnchainAllowed(
        fair({ status: 'Pending', onchain_locked: true }),
      ),
    ).toBe(false);
    expect(isEndOnchainAllowed(fairActive)).toBe(false);
    expect(isCancelOnchainAllowed(fairActive)).toBe(false);
    expect(isPlatformFeeWithdrawable(mismatch)).toBe(false);
  });

  it('continues to reject wrong status and active submissions', () => {
    expect(isCreateOnchainAllowed(capped({ status: 'Pending' }))).toBe(false);
    expect(
      isCreateOnchainAllowed(
        capped({
          status: 'Set',
          active_submission: { action: 'v2_create', id: 9, state: 'pending' },
        }),
      ),
    ).toBe(false);
  });
});

describe('isRestorable（启用/恢复）', () => {
  it('后端允许且历史未分配项目归档前状态可信 → 可恢复', () => {
    expect(
      isRestorable(
        row({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          restore_allowed: true,
        }),
      ),
    ).toBe(true);
  });

  it('fair Launch、unknown/mismatch archived rows fail closed even if backend says allowed', () => {
    expect(
      isRestorable(
        fair({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          restore_allowed: true,
        }),
      ),
    ).toBe(false);
    expect(
      isRestorable(
        row({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          funding_mode: 'fair_launch',
          contract_version: 'v2_stablecoin',
          restore_allowed: true,
        }),
      ),
    ).toBe(false);
    expect(
      isRestorable(
        row({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          funding_mode: 'future_mode',
          contract_version: 'future_v9',
        }),
      ),
    ).toBe(false);
  });

  it('v2_stablecoin cannot be restored even when backend says allowed', () => {
    expect(
      isRestorable(
        capped({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          restore_allowed: true,
        }),
      ),
    ).toBe(false);
  });

  it('onchain locked and active submission remain non-restorable', () => {
    expect(
      isRestorable(
        row({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          onchain_locked: true,
        }),
      ),
    ).toBe(false);
    expect(
      isRestorable(
        row({
          status: 'archived',
          pre_archive_status: 'pre_launch',
          active_submission: { action: 'v2_create', id: 1, state: 'pending' },
        }),
      ),
    ).toBe(false);
  });
});

describe('disable/delete lifecycle guards', () => {
  it('allows only unassigned historical content rows', () => {
    const historical = row({ contract_version: 'unassigned', status: 'Set' });
    expect(isDisableAllowed(historical)).toBe(true);
    expect(isDeleteAllowed(historical)).toBe(true);
  });

  it('rejects Fair Launch, mismatch, unknown, capped V2 and active submissions', () => {
    const mismatch = row({
      funding_mode: 'fair_launch',
      contract_version: 'v2_stablecoin',
    });
    const unknown = row({
      funding_mode: 'future_mode',
      contract_version: 'future_v9',
    });
    for (const project of [fair(), mismatch, unknown, capped()]) {
      expect(isDisableAllowed(project)).toBe(false);
      expect(isDeleteAllowed(project)).toBe(false);
    }
    expect(
      isDisableAllowed(
        row({
          contract_version: 'unassigned',
          active_submission: { action: 'v2_create', id: 1, state: 'pending' },
        }),
      ),
    ).toBe(false);
  });
});

describe('project content cover upload persistence', () => {
  it('persists the uploaded cover URL immediately', () => {
    const source = contentEditorSource();

    expect(source).toContain(
      'await updateProjectContent(projectId.value, { cover_url: res.url });',
    );
    expect(source).toContain("message.success('Cover 上传并保存成功');");
  });
});

describe('project content launch time clearing', () => {
  it('keeps launch_at empty until Save serializes the clear operation', () => {
    const source = contentEditorSource();

    expect(source).not.toContain('v-model:value="launchAtProxy"');
    expect(source).toContain(':value="launchAtValue"');
    expect(source).toContain('@update:value="updateLaunchAt"');
    expect(source).toContain(':allow-clear="false"');
    expect(source).toContain('function clearLaunchAt()');
    expect(source).toContain('if (launchAtClearPending.value) return;');
    expect(source).toContain('form.launch_at = null;');
    expect(source).toContain("launch_at: form.launch_at || ''");
  });
});
