import type { OnchainParamsCurrent } from '#/api/joincare/project_content';

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import {
  isV3PolicyPreviewProjectCreated,
  isV3PolicyReadableProject,
} from '../v3-policy-eligibility';

const current = {
  contract_version: 'ido_v3_stablecoin',
  funding_mode: 'capped_presale',
  locked: true,
  status: 'Failed',
} as OnchainParamsCurrent;

describe('v3 read-only terminal policy access', () => {
  it('allows terminal reads without allowing governance preview', () => {
    for (const status of ['Successful', 'Failed', 'Canceled', 'Cancelled']) {
      expect(isV3PolicyReadableProject({ ...current, status })).toBe(true);
      expect(isV3PolicyPreviewProjectCreated({ ...current, status })).toBe(
        false,
      );
    }
    for (const status of ['Pending', 'Active']) {
      expect(isV3PolicyReadableProject({ ...current, status })).toBe(true);
      expect(isV3PolicyPreviewProjectCreated({ ...current, status })).toBe(
        true,
      );
    }
  });

  it('rejects uncreated, unlocked or incompatible projects', () => {
    for (const patch of [
      { locked: false },
      { status: 'Set' },
      { status: 'Setup' },
      { status: 'archived' },
      { status: 'unknown' },
      { contract_version: 'v2_stablecoin' },
      { funding_mode: 'fair_launch' },
    ]) {
      expect(isV3PolicyReadableProject({ ...current, ...patch })).toBe(false);
    }
  });

  it('allows halted chain observation but never a governance preview', () => {
    const halted = { ...current, status: 'V3ReorgHalted' };
    expect(isV3PolicyReadableProject(halted)).toBe(true);
    expect(isV3PolicyPreviewProjectCreated(halted)).toBe(false);
    const source = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/views/joincare/projects/v3_policy_preview.vue',
      ),
      'utf8',
    );
    expect(source).toContain("status?.scanner_state === 'reorg_halted'");
    expect(source).toContain('项目扫描已暂停');
  });

  it('routes policy reads through read gate while preview still uses mutation gate', () => {
    const s = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/views/joincare/projects/v3_policy_preview.vue',
      ),
      'utf8',
    );
    const read = s.slice(
      s.indexOf('async function refreshPolicyContext'),
      s.indexOf('async function refreshCurrentMembers'),
    );
    expect(read).toContain('if (!policyReadable.value) return');
    expect(read).not.toContain('if (!projectCreated.value) return');
    const preview = s.slice(
      s.indexOf('async function preview()'),
      s.indexOf('async function startReview()'),
    );
    expect(preview).toContain('if (!projectCreated.value)');
    expect(s).toContain(':disabled="!canWritePolicy || !projectCreated"');
    expect(s).not.toContain('治理规则与当前链上身份');
    expect(s).not.toContain(
      '完整名单替换，不是追加。Active 不可关闭白名单；审批不等于生效。',
    );
  });
});
