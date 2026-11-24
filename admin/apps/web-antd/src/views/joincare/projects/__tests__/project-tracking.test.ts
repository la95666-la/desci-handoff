import type { ProjectListRow } from '#/api/joincare/project_content';

import { describe, expect, it } from 'vitest';

import {
  trackingActions,
  trackingLabel,
  trackingPresentation,
} from '../project-tracking';

const row = (status: string) =>
  ({
    project_id: '9007199254740999',
    status,
    onchain_locked: true,
    contract_version: 'ido_v3_stablecoin',
    tracking: {
      mode: 'normal',
      finished: false,
      evidence_current: true,
      wake_block: 0,
      pending_reason: 'user_claims_pending',
      revision: 1,
    },
  }) as ProjectListRow;
describe('v3 independent closure/tracking', () => {
  it('separates settlement reason from tracking mode without changing actions', () => {
    const r = row('Failed');
    if (!r.tracking) throw new Error('fixture');
    r.tracking.pending_reason = 'user_refunds_pending';
    r.tracking.mode = 'maintenance';
    expect(trackingPresentation(r)).toMatchObject({
      settlement: '未结清',
      reason: '等待用户退款',
      mode: '12小时维护',
      modeColor: 'purple',
      settlementColor: 'warning',
    });
    r.tracking.pending_reason = 'not_verified';
    r.tracking.mode = 'normal';
    expect(trackingPresentation(r)).toMatchObject({
      reason: '尚未核验',
      mode: '正常跟踪',
      modeColor: 'blue',
    });
    expect(trackingActions(r).map((x) => x.action)).toEqual([
      'sync',
      'maintenance',
    ]);
  });
  it('never offers low-frequency mode to Active', () => {
    expect(
      trackingActions(row('Active')).some((x) => x.action === 'maintenance'),
    ).toBe(false);
  });
  it('offers maintenance to terminal projects but never archive before Finished', () => {
    const r = row('Failed');
    expect(trackingActions(r).map((x) => x.action)).toEqual([
      'sync',
      'maintenance',
    ]);
    expect(trackingLabel(r)).toContain('未结清');
  });
  it('archive is evidence-bound, not a lifecycle overwrite', () => {
    const r = row('Successful');
    if (!r.tracking) throw new Error('tracking fixture missing');
    r.tracking.finished = true;
    expect(trackingActions(r).some((x) => x.action === 'archive')).toBe(true);
    expect(trackingLabel(r)).toContain('Finished（原结果 Successful）');
    r.tracking.wake_block = 100;
    expect(trackingActions(r).some((x) => x.action === 'archive')).toBe(false);
  });
  it('default-off and unknown/V2 identities have no controls', () => {
    const r = row('Failed');
    r.tracking = undefined;
    expect(trackingActions(r)).toEqual([]);
    r.contract_version = 'v2_stablecoin';
    expect(trackingActions(r)).toEqual([]);
  });
});
