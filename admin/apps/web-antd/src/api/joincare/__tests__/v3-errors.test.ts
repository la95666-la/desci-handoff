import { describe, expect, it } from 'vitest';

import { isAmbiguousOnchainCreateError } from '../onchain-create-error';
import { formatV3Diagnostic, v3DiagnosticMessage } from '../v3-errors';

describe('safe V3 diagnostics', () => {
  it('shows sender maintenance pending before intent without suggesting a signed recovery', () => {
    const data = {
      reason_code: 'SENDER_EVIDENCE_PENDING',
      stage: 'sender_readiness',
      submission_outcome: 'not_reserved',
      next_action: 'check_sender_maintenance',
    };
    const text = formatV3Diagnostic(data);
    expect(text).toContain('维护核验');
    expect(text).toContain('尚未建立新的提交记录');
    expect(text).toContain('sender_readiness');
    expect(
      isAmbiguousOnchainCreateError({ response: { status: 400, data } }),
    ).toBe(false);
    expect(text).not.toContain('已签名');
  });
  it('shows specific reason, phase and safe support references without raw messages', () => {
    const text = formatV3Diagnostic({
      reason_code: 'CREATE_DEADLINE_EXPIRED',
      stage: 'signature_persist',
      submission_id: '65',
      request_id: 'a'.repeat(24),
      message: 'SECRET https://private.example/key',
      submission_outcome: 'unsigned_cancelled',
    });
    expect(text).toContain('截止时间已过');
    expect(text).toContain('signature_persist');
    expect(text).toContain('#65');
    expect(text).toContain('未签名并取消');
    expect(text).not.toContain('SECRET');
    expect(text).not.toContain('private.example');
  });
  it('never renders arbitrary provider data or invents a reason for unknown codes', () => {
    expect(
      formatV3Diagnostic({ reason_code: 'unknown', message: 'secret' }),
    ).toBeNull();
    const text = formatV3Diagnostic({
      reason_code: 'UNAVAILABLE',
      stage: 'secret_password',
      request_id: 'unsafe',
      tx_hash: 'secret',
    });
    expect(text).not.toContain('secret_password');
    expect(text).not.toContain('unsafe');
    expect(v3DiagnosticMessage(new Error('SECRET'), 'safe fallback')).toBe(
      'safe fallback [IDO_CHECK_UNAVAILABLE]',
    );
  });
  it('keeps signed and unknown submission outcomes fenced, even on HTTP400', () => {
    const data = {
      reason_code: 'RPC_TIMEOUT',
      stage: 'broadcast_original',
      submission_outcome: 'signed_or_submitted',
      next_action: 'reconcile_original_do_not_resend',
      tx_hash: `0x${'a'.repeat(64)}`,
    };
    const error = { response: { status: 400, data } };
    expect(isAmbiguousOnchainCreateError(error)).toBe(true);
    expect(v3DiagnosticMessage(error, 'fallback')).toContain('切勿重复发送');
    expect(v3DiagnosticMessage(error, 'fallback')).toContain(data.tx_hash);
    expect(
      isAmbiguousOnchainCreateError({
        response: {
          status: 400,
          data: { submission_outcome: 'unsigned_cancelled' },
        },
      }),
    ).toBe(false);
  });
});

describe('accurate operator outcomes', () => {
  it('explains insufficient creation time without claiming signed or chain revert',()=>{
    const text=formatV3Diagnostic({reason_code:'CREATE_RUNWAY_INSUFFICIENT',stage:'approved_set',submission_outcome:'not_reserved',next_action:'prepare_future_set_and_review'});
    expect(text).toContain('截止时间过近');
    expect(text).toContain('CREATE_RUNWAY_INSUFFICIENT');
    expect(text).toContain('尚未建立新的提交记录');
    expect(text).not.toContain('已签名或结果未确认');
  });
  it('does not interpret a policy pre-commit rejection as a sender problem', () => {
    const text = formatV3Diagnostic({reason_code:'STATE_CHANGED', stage:'review_digest', submission_outcome:'not_reserved', next_action:'refresh_and_check'});
    expect(text).toContain('原因代码：STATE_CHANGED');
    expect(text).toContain('审批摘要核验（review_digest）');
    expect(text).not.toContain('sender');
    expect(text).not.toContain('未签名、未广播');
  });
  it.each(['SENDER_EVIDENCE_PENDING','SENDER_EVIDENCE_UNAVAILABLE'])('never claims unsigned for a post-sign %s', reason_code => {
    const text=formatV3Diagnostic({reason_code,stage:'signed_admission',submission_outcome:'signed_or_submitted',next_action:'reconcile_original_do_not_resend'});
    expect(text).toContain(reason_code);
    expect(text).toContain('切勿重复发送');
    expect(text).not.toContain('未建立新签名');
    expect(text).not.toContain('未签名、未广播');
  });
  it('reconcile instruction wins over conflicting not_reserved',()=>{
    const text=formatV3Diagnostic({reason_code:'NONCE_UNRESOLVED',submission_outcome:'not_reserved',next_action:'reconcile_original_do_not_resend'});
    expect(text).toContain('切勿重复发送');
    expect(text).not.toContain('尚未建立');
  });
});
