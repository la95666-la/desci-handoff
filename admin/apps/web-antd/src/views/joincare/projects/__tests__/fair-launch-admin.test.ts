import type { FairLaunchCapabilityEnvelope } from '#/api/joincare/fair_launch';
import type {
  ProjectContent,
  ProjectListRow,
} from '#/api/joincare/project_content';

import { describe, expect, it } from 'vitest';

import {
  FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE,
  fairLaunchActionReasonText,
  getFairLaunchActionAccess,
  isFairLaunchApprovalExecutionAvailable,
  isFairLaunchApprovalMutationAvailable,
  isFairLaunchApprovalOperation,
  isFairLaunchDurableAction,
  isFairLaunchEventContractVersion,
  loadExactFairLaunchExecutionQueue,
  runApprovalMutation,
} from '../fair-launch-admin';

function project(overrides: Partial<ProjectListRow> = {}): ProjectListRow {
  return {
    category: '',
    contract_version: 'fair_launch_v1',
    cover_url: '',
    created_at: '',
    funding_mode: 'fair_launch',
    id: 1,
    launch_at: null,
    name_cn: '',
    name_en: 'Fair',
    onchain_locked: true,
    project_id: '7',
    status: 'Pending',
    tagline: '',
    updated_at: '',
    ...overrides,
  };
}

function capability(
  overrides: Partial<FairLaunchCapabilityEnvelope> = {},
): FairLaunchCapabilityEnvelope {
  return {
    actions: {
      activate: {
        action: 'activate',
        approval_operation: 'fair_launch.action.activate',
        enabled: true,
        execution_domain: 'operator_durable_outbox',
        requires_four_eyes: true,
        requires_preview: true,
        requires_step_up_mfa: true,
        requires_three_party_separation: true,
      },
      create: {
        action: 'create',
        approval_operation: 'fair_launch.create',
        enabled: true,
        execution_domain: 'operator_durable_outbox',
        requires_four_eyes: true,
        requires_preview: true,
        requires_step_up_mfa: false,
        requires_three_party_separation: false,
      },
      release: {
        action: 'release',
        approval_operation: 'fair_launch.release',
        enabled: true,
        execution_domain: 'operator_durable_outbox',
        requires_four_eyes: true,
        requires_preview: true,
        requires_step_up_mfa: true,
        requires_three_party_separation: true,
        step_up_mfa_verified: false,
        forbidden_admin_ids: [10, 11],
      },
    },
    available: true,
    contract_version: 'fair_launch_v1',
    funding_mode: 'fair_launch',
    project_id: '7',
    runtime: {
      chain_id: 56,
      contract_version: 'fair_launch_v1',
      runtime_verified: true,
    },
    runtime_enabled: true,
    ...overrides,
  };
}

describe('fair launch admin capability gate', () => {
  it('blocks lifecycle, governance and withdrawal controls for a verified runtime', () => {
    for (const action of [
      'activate',
      'set_operator',
      'withdraw_project_proceeds',
    ] as const) {
      const result = getFairLaunchActionAccess({
        action,
        capabilities: capability(),
        permissions: ['m2_project:execute'],
        project: project(),
      });
      expect(result).toMatchObject({
        allowed: false,
        reason: 'materialization_unavailable',
      });
      expect(fairLaunchActionReasonText(result)).toBe(
        FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE,
      );
    }

    const mismatch = getFairLaunchActionAccess({
      action: 'create',
      capabilities: capability(),
      permissions: ['m2_project:execute'],
      project: project({ contract_version: 'v2_stablecoin' }),
    });
    expect(mismatch).toMatchObject({
      allowed: false,
      reason: 'identity_mismatch',
    });
  });

  it('fails closed when capability is absent, unavailable, disabled or unverified', () => {
    for (const capabilities of [
      null,
      capability({ available: false }),
      capability({ runtime_enabled: false }),
      capability({
        runtime: {
          chain_id: 56,
          contract_version: 'fair_launch_v1',
          runtime_verified: false,
        },
      }),
    ]) {
      expect(
        getFairLaunchActionAccess({
          action: 'create',
          capabilities,
          permissions: ['m2_project:execute'],
          project: project(),
        }).allowed,
      ).toBe(false);
    }
  });

  it('keeps Fair Launch approval rows fully audit-only while non-Fair mutations remain unchanged', async () => {
    for (const operation of [
      'fair_launch.action.release',
      '  FAIR_LAUNCH.ACTION.FUTURE  ',
      'project.fair_launch.configure',
    ]) {
      expect(isFairLaunchApprovalOperation(operation)).toBe(true);
      expect(isFairLaunchApprovalExecutionAvailable(operation)).toBe(false);
      expect(isFairLaunchApprovalMutationAvailable(operation)).toBe(false);

      let called = false;
      await expect(
        runApprovalMutation(operation, async () => {
          called = true;
          return 'mutated';
        }),
      ).rejects.toThrow(FAIR_LAUNCH_MATERIALIZATION_UNAVAILABLE);
      expect(called).toBe(false);
    }

    const mutate = async () => 'non-fair-mutated';
    expect(isFairLaunchApprovalOperation('project.update_content')).toBe(false);
    expect(
      isFairLaunchApprovalExecutionAvailable('project.update_content'),
    ).toBe(true);
    await expect(
      runApprovalMutation('project.update_content', mutate),
    ).resolves.toBe('non-fair-mutated');
  });

  it('classifies trimmed case-insensitive current and future Fair Launch operations', () => {
    for (const action of [
      'fair_launch_create',
      ' FAIR_LAUNCH_RELEASE ',
      '\tfair_launch_future_governance\n',
    ]) {
      expect(isFairLaunchDurableAction(action)).toBe(true);
    }
    expect(isFairLaunchDurableAction('v2_create')).toBe(false);
    expect(isFairLaunchDurableAction('project_end')).toBe(false);
    expect(isFairLaunchEventContractVersion(' Fair_Launch_V1 ')).toBe(true);
    expect(isFairLaunchEventContractVersion(' fair_launch_v2_future ')).toBe(
      true,
    );
    expect(isFairLaunchEventContractVersion('v2_stablecoin')).toBe(false);
  });

  it('loads the direct execution queue only after trusted exact identity admission', async () => {
    const exactProject = project() as unknown as ProjectContent;
    const items = { items: [] };
    let queueCalls = 0;
    const loadQueue = async () => {
      queueCalls += 1;
      return items;
    };

    await expect(
      loadExactFairLaunchExecutionQueue({
        loadProject: async () => exactProject,
        loadQueue,
        projectId: 7,
      }),
    ).resolves.toBe(items);
    expect(queueCalls).toBe(1);

    for (const untrusted of [
      project({ funding_mode: 'capped_presale' }),
      project({ contract_version: 'v2_stablecoin' }),
      project({ funding_mode: 'future_mode', contract_version: 'future_v9' }),
    ]) {
      await expect(
        loadExactFairLaunchExecutionQueue({
          loadProject: async () => untrusted as unknown as ProjectContent,
          loadQueue,
          projectId: 7,
        }),
      ).rejects.toThrow('fair_launch_project_identity_unavailable');
    }
    expect(queueCalls).toBe(1);

    let projectCalls = 0;
    await expect(
      loadExactFairLaunchExecutionQueue({
        loadProject: async () => {
          projectCalls += 1;
          return exactProject;
        },
        loadQueue,
        projectId: Number.NaN,
      }),
    ).rejects.toThrow('fair_launch_project_identity_unavailable');
    expect(projectCalls).toBe(0);
    expect(queueCalls).toBe(1);
  });

  it('does not infer an action descriptor from lifecycle status', () => {
    const withoutEnd = capability();
    const result = getFairLaunchActionAccess({
      action: 'end',
      capabilities: withoutEnd,
      permissions: ['m2_project:execute'],
      project: project({
        end_time: '2000-01-01T00:00:00Z',
        status: 'Active',
      }),
    });
    expect(result.reason).toBe('materialization_unavailable');
  });
});
