import { readFileSync } from 'node:fs';

import { describe, expect, it } from 'vitest';

function source(relativePath: string): string {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8');
}

const legacyLedgerSources = [
  '../../../../api/joincare/fund_ledger.ts',
  '../../../../store/fund-ledger.ts',
  '../../fund_ledger/reconciliation.vue',
  '../../fund_ledger/transactions.vue',
  '../../fund_ledger/audit.vue',
].map((relativePath) => source(relativePath));

describe('fair Launch Admin materialization freeze', () => {
  it('removes Fair Launch concepts from legacy V2 ledger and reconciliation', () => {
    const combined = legacyLedgerSources.join('\n');

    for (const forbidden of [
      'fair_launch_v1',
      'fair_launch_contribution',
      'chain_fair_launch_',
      'Fair Launch reconciliation',
      'Fair Launch 贡献',
      'final_price_numerator_raw',
      'final_price_denominator_raw',
    ]) {
      expect(combined).not.toContain(forbidden);
    }
  });

  it('keeps the Set route as a read-only summary without sensitive inputs or calls', () => {
    const setPage = source('../fair_launch_set.vue');

    expect(setPage).toContain('getFairLaunchSet');
    expect(setPage).toContain('fair_launch_materialization_unavailable');
    expect(setPage).toContain('只读摘要');
    for (const forbidden of [
      'previewFairLaunchSet',
      'submitFairLaunchSet',
      'getFairLaunchCapabilities',
      'myPermissions',
      'mfaAssertionToken',
      'mfa_assertion_token',
      'AInput',
      'InputNumber',
      'DatePicker',
      '@click=',
      '<Form',
      '<Button',
    ]) {
      expect(setPage).not.toContain(forbidden);
    }
  });

  it('removes enabled Fair Set mutation navigation from the project list', () => {
    const list = source('../list.vue');

    expect(list).not.toContain('openFairLaunchSet');
    expect(list).not.toContain('Fair Set');
    expect(list).not.toContain('/fair-launch/set');
  });

  it('does not expose legacy V2 ledger or reconciliation navigation', () => {
    const operations = source('../fair_launch_operations.vue');

    expect(operations).toContain(
      'Fair Launch canonical ledger / reconciliation API 尚不可用',
    );
    expect(operations).toContain('Fair Launch fund ledger unavailable');
    expect(operations).toContain('Fair Launch reconciliation unavailable');
    expect(operations).not.toContain(
      "openOps('/joincare/fund-ledger/transactions')",
    );
    expect(operations).not.toContain(
      "openOps('/joincare/fund-ledger/reconciliation')",
    );
    expect(operations).not.toContain('previewFairLaunchAction');
    expect(operations).not.toContain('submitFairLaunchAction');
    expect(operations).not.toContain('mfaAssertionToken');
    expect(operations).not.toContain('Durable submissions');
    expect(operations).not.toContain("openOps('/joincare/chain-transactions')");
  });

  it('keeps the execution queue read-only and never submits execution', () => {
    const queue = source('../fair_launch_execution_queue.vue');

    expect(queue).toContain('fair_launch_materialization_unavailable');
    expect(queue).toContain('append-only transitions 仅供只读审计');
    expect(queue).toContain('loadExactFairLaunchExecutionQueue');
    expect(queue).toContain('getProjectContent');
    expect(queue).not.toContain('executeFairLaunchAction');
    expect(queue).not.toContain('mfaAssertionToken');
    expect(queue).not.toContain('@click="execute(');
  });

  it('keeps Fair Launch approvals visible but removes approval/MFA execution', () => {
    const approvals = source('../../approvals/index.vue');

    expect(approvals).toContain('Fair Launch approvals are read-only');
    expect(approvals).toContain('isFairLaunchApprovalMutationAvailable');
    expect(approvals).toContain('runApprovalMutation');
    expect(approvals).toContain('批准、驳回、撤单与第三执行入口全部不可用');
    expect(approvals).not.toContain('mfaAssertionToken');
  });

  it('defensively blocks Fair Launch cancel and replay mutations in generic operations', () => {
    const transactions = source('../../chain_transactions/index.vue');
    const events = source('../../chain_events/index.vue');

    expect(transactions).toContain('isFairLaunchDurableAction(row.action)');
    expect(transactions).toContain('isFairLaunchDurableAction(target.action)');
    expect(events).toContain(
      'isFairLaunchEventContractVersion(row.contract_version)',
    );
    expect(events).toContain(
      'isFairLaunchEventContractVersion(target.contract_version)',
    );
  });

  it('fails closed on list errors and stale legacy V2 selections', () => {
    const picker = source('../../fund_ledger/components/ProjectPicker.vue');
    const state = source('../../fund_ledger/project-picker-state.ts');

    expect(picker).toContain('legacyV2ProjectOptions');
    expect(picker).toContain('reconcileLegacyV2ProjectSelection');
    expect(picker).toContain('clearLegacyV2ProjectSelection(store)');
    expect(picker).toContain(
      '!options.value.some((project) => project.project_id === value)',
    );
    expect(picker).toContain(':disabled="store.projectId === undefined"');
    expect(state).toContain('state.projectId = undefined');
    expect(state).toContain('state.invalidateLoadedProject()');
  });
});
