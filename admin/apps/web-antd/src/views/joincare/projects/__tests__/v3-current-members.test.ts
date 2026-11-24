import type {
  V3PolicyPreview,
  V3PolicyReviewRow,
  V3PolicyStatus,
} from '#/api/joincare/project_content';

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { loadV3CurrentMembers } from '../v3-current-members';

const root = `0x${'a'.repeat(64)}`;
const wallet = `0x${'1'.repeat(40)}`;
const live = {
  chain_id: '56',
  project_id: '43264392090877953',
  proxy: `0x${'2'.repeat(40)}`,
  enabled: true,
  root,
  version: '3',
} as V3PolicyStatus;
const row = {
  review_id: '9',
  project_id: live.project_id,
  enabled: true,
  target_version: '3',
  status: 'observed_not_proof',
  member_count: 1,
  preview_digest: root,
} as V3PolicyReviewRow;
const detail = {
  ...live,
  target_version: '3',
  member_count: 1,
  preview_digest: root,
  canonical_members: [wallet],
} as unknown as V3PolicyPreview;

describe('current enabled V3 wallet list', () => {
  it('loads separately from candidate input and discards stale async results', () => {
    const source = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/views/joincare/projects/v3_policy_preview.vue',
      ),
      'utf8',
    );
    const loader = source.slice(
      source.indexOf('watch('),
      source.indexOf('// A timed-out POST'),
    );
    expect(loader).toContain('request === currentMembersRequest');
    expect(loader).toContain('currentMembers.value = null');
    expect(loader).not.toContain('wallets.value =');
    expect(source).toContain('aria-label="当前白名单钱包地址"');
    expect(source).toContain('@click="refreshCurrentMembers"');
  });
  it('loads exact executed snapshot and rereads chain commitment before displaying', async () => {
    const calls: string[] = [];
    const result = await loadV3CurrentMembers(
      live,
      [row],
      async (id) => {
        calls.push(id);
        return detail;
      },
      async () => {
        calls.push('fresh');
        return live;
      },
    );
    expect(result).toEqual({
      members: [wallet],
      reviewId: '9',
      root,
      version: '3',
    });
    expect(calls).toEqual(['9', 'fresh']);
    result.members.push('changed');
    expect(detail.canonical_members).toEqual([wallet]);
  });

  it('never substitutes pending, merely approved, old-version or another project lists', async () => {
    for (const patch of [
      { status: 'pending_second_admin' },
      { status: 'approved_pending_safe' },
      { status: 'expired' },
      { target_version: '2' },
      { project_id: 'other' },
      { enabled: false },
    ]) {
      await expect(
        loadV3CurrentMembers(
          live,
          [{ ...row, ...patch } as V3PolicyReviewRow],
          async () => {
            throw new Error('must not load');
          },
          async () => live,
        ),
      ).rejects.toThrow('current_members_unavailable');
    }
    await expect(
      loadV3CurrentMembers(
        { ...live, enabled: false },
        [row],
        async () => detail,
        async () => live,
      ),
    ).rejects.toThrow('policy_disabled');
    await expect(
      loadV3CurrentMembers(
        live,
        [row, { ...row, review_id: '10' }],
        async () => detail,
        async () => live,
      ),
    ).rejects.toThrow('current_members_unavailable');
  });

  it('rejects wrong identity, root, digest, counts and invalid members', async () => {
    for (const patch of [
      { chain_id: '97' },
      { project_id: 'other' },
      { proxy: wallet },
      { root: `0x${'b'.repeat(64)}` },
      { target_version: '4' },
      { enabled: false },
      { preview_digest: 'wrong' },
      { member_count: 2 },
      { canonical_members: [] },
      { canonical_members: ['not-an-address'] },
      { canonical_members: [`0x${'0'.repeat(40)}`] },
    ]) {
      await expect(
        loadV3CurrentMembers(
          live,
          [row],
          async () => ({ ...detail, ...patch }),
          async () => live,
        ),
      ).rejects.toThrow('current_members_mismatch');
    }
  });

  it('fails closed on request errors and policy changes during loading', async () => {
    await expect(
      loadV3CurrentMembers(
        live,
        [row],
        async () => {
          throw new Error('network');
        },
        async () => live,
      ),
    ).rejects.toThrow('network');
    await expect(
      loadV3CurrentMembers(
        live,
        [row],
        async () => detail,
        async () => ({ ...live, version: '4' }),
      ),
    ).rejects.toThrow('policy_changed');
  });
});
