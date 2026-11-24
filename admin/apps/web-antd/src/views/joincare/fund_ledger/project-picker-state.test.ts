import type { ProjectListRow } from '#/api/joincare/project_content';

import { describe, expect, it, vi } from 'vitest';

import {
  clearLegacyV2ProjectSelection,
  legacyV2ProjectOptions,
  reconcileLegacyV2ProjectSelection,
} from './project-picker-state';

function project(
  projectId: number,
  fundingMode: ProjectListRow['funding_mode'],
  contractVersion: string,
): ProjectListRow {
  return {
    category: '',
    contract_version: contractVersion,
    cover_url: '',
    created_at: '',
    funding_mode: fundingMode,
    id: projectId,
    launch_at: null,
    name_cn: '',
    name_en: `Project ${projectId}`,
    onchain_locked: false,
    project_id: String(projectId),
    status: 'Set',
    tagline: '',
    updated_at: '',
  };
}

describe('legacy V2 project picker state', () => {
  it('admits only explicit capped_presale/v2_stablecoin rows', () => {
    const options = legacyV2ProjectOptions([
      project(1, 'capped_presale', 'v2_stablecoin'),
      project(2, 'fair_launch', 'fair_launch_v1'),
      project(3, undefined, 'v2_stablecoin'),
      project(4, 'fair_launch', 'v2_stablecoin'),
    ]);

    expect(options.map((row) => row.project_id)).toEqual(['1']);
  });

  it('clears and invalidates a stale selection after list refresh', () => {
    const invalidateLoadedProject = vi.fn();
    const state = {
      projectId: '9' as string | undefined,
      invalidateLoadedProject,
    };

    reconcileLegacyV2ProjectSelection(state, [
      project(1, 'capped_presale', 'v2_stablecoin'),
    ]);

    expect(state.projectId).toBeUndefined();
    expect(invalidateLoadedProject).toHaveBeenCalledOnce();
  });

  it('clears and invalidates loaded state when list loading fails', () => {
    const invalidateLoadedProject = vi.fn();
    const state = {
      projectId: '1' as string | undefined,
      invalidateLoadedProject,
    };

    clearLegacyV2ProjectSelection(state);

    expect(state.projectId).toBeUndefined();
    expect(invalidateLoadedProject).toHaveBeenCalledOnce();
  });
});
