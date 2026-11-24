import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { senderReadinessAllowsSubmit, senderReadinessLabel } from '../sender-readiness';

describe('sender readiness before new submission', () => {
  it('unknown/pending/active never authorizes submit', () => {
    expect(senderReadinessAllowsSubmit(null)).toBe(false);
    expect(senderReadinessAllowsSubmit({ ready: false, reason_code: 'SENDER_EVIDENCE_PENDING' })).toBe(false);
    expect(senderReadinessAllowsSubmit({ ready: true, active_submission_id: '96' })).toBe(false);
    expect(senderReadinessAllowsSubmit({ ready: true })).toBe(true);
    expect(senderReadinessLabel({ ready: false, reason_code: 'SENDER_EVIDENCE_PENDING' })).toContain('不会自动重提');
    expect(senderReadinessLabel({ ready: false, active_submission_id: '96' })).toContain('切勿重发');
  });
  it('polling only reads and disposes; writes require a fresh gate', () => {
    const source = readFileSync(resolve(process.cwd(), 'apps/web-antd/src/views/joincare/projects/list.vue'), 'utf8');
    const poll = source.slice(source.indexOf('async function refreshSenderReadiness'), source.indexOf('async function requireSenderBeforeSubmit'));
    expect(poll).toContain('getV3SenderReadiness()');
    expect(poll).not.toContain('submit(');
    expect(source.match(/await requireSenderBeforeSubmit\(row\)/g)?.length).toBe(2);
    expect(source).toContain('senderDisposed = true');
    expect(source).toContain('if (senderChecking.value)');
  });
});
