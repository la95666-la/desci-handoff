import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { formatIdoCode, idoErrorMessage, isIdoRequest, storedIdoFailure } from '../ido-errors';

describe('IDO operator Chinese + English diagnostics', () => {
  it('explains permission denial without parsing backend message text',()=>{
    const text=idoErrorMessage({code:'ADMIN_PERM_DENIED',required_permission:'m2_project:write',message:'SECRET missing permission'});
    expect(text).toContain('项目编辑');expect(text).toContain('m2_project:write');
    expect(text).toContain('ADMIN_PERM_DENIED');expect(text).toContain('不是链上参数校验失败');
    expect(text).not.toContain('SECRET');
    expect(idoErrorMessage({code:'ADMIN_PERM_DENIED',message:'missing permission m2_project:write'})).not.toContain('m2_project:write');
  });
  it.each([
    'PROJECT_ONCHAIN_PARAMS_INVALID','PROJECT_ONCHAIN_PARAMS_FAILED','PROJECT_EXECUTE_REVOKED',
    'PROJECT_CREATE_ONCHAIN_FAILED','PROJECT_ACTIVATE_ONCHAIN_FAILED','PROJECT_END_ONCHAIN_FAILED',
    'PROJECT_CANCEL_ONCHAIN_FAILED','PLATFORM_FEE_WITHDRAW_FAILED','IDO_PAUSE_FAILED','IDO_UNPAUSE_FAILED',
    'ADMIN_APPRV_APPROVE_FAILED','ADMIN_APPRV_REJECT_FAILED','ADMIN_APPRV_CANCEL_FAILED',
    'V3_POLICY_PREVIEW_UNAVAILABLE','V3_POLICY_STATUS_UNAVAILABLE','V3_POLICY_REVIEW_UNAVAILABLE',
    'V3_POLICY_APPROVAL_UNAVAILABLE','V3_POLICY_EXECUTION_UNAVAILABLE','V3_POLICY_SAFE_HANDOFF_UNAVAILABLE',
    'SAFE_PROPOSAL_UNAVAILABLE','TRACKING_CHANGE_REJECTED','V3_ORIGINAL_CONTINUATION_REJECTED',
    'CHAIN_TX_NOT_CANCELLABLE','V3_PROJECT_TOKEN_RECEIPT_UNVERIFIED','V3_RUNTIME_UNAVAILABLE',
  ])('explains %s without leaking backend text',code=>{
    const text = idoErrorMessage({response:{data:{code,message:'https://rpc/SECRET password SQL'}}});
    expect(text).toContain(code);
    expect(text).toMatch(/[\u4E00-\u9FFF]/);
    expect(text).not.toContain('SECRET');
    expect(formatIdoCode({code})).not.toBeNull();
  });
  it('prefers specific reason and reports interface code as separate metadata',()=>{
    const text=idoErrorMessage({code:'PROJECT_ACTIVATE_ONCHAIN_FAILED',reason_code:'NONCE_UNRESOLVED',stage:'nonce_check',submission_outcome:'signed_or_submitted'});
    expect(text).toContain('NONCE_UNRESOLVED');
    expect(text).toContain('PROJECT_ACTIVATE_ONCHAIN_FAILED');
    expect(text).toContain('切勿重复发送');
  });
  it('does not invent failure cause or unsigned status for unknown/transport outcomes',()=>{
    for(const error of [new Error('SECRET'),{code:'ERR_NETWORK'},{response:{status:504,data:{code:'NEW_SERVER_CODE',message:'SECRET'}}}]){
      const text=idoErrorMessage(error);
      expect(text).toContain('未获得成功确认');
      expect(text).not.toContain('未广播');
      expect(text).not.toContain('SECRET');
      expect(text).toMatch(/(?:IDO_RESULT_UNCONFIRMED|ERR_NETWORK|NEW_SERVER_CODE)/);
    }
    expect(idoErrorMessage({code:'https://private/SECRET'})).not.toContain('SECRET');
  });
  it('recognizes all request lanes without affecting unrelated settings/login',()=>{
    for(const path of ['/v1/projects/1/create-onchain','/admin/v1/projects/1/ido-v3/policy/approve','/v1/approvals/1/approve','/v1/chain-transactions/1/v3-original/broadcast','/v1/ido-v3/sender-readiness']) expect(isIdoRequest(path)).toBe(true);
    expect(isIdoRequest('/v1/settings')).toBe(false);
    expect(isIdoRequest('/v1/auth/login')).toBe(false);
  });
  it('does not infer semantics from stored provider or SQL text',()=>{
    expect(storedIdoFailure('SECRET raw_tx https://rpc')).toContain('IDO_STORED_ERROR_UNCLASSIFIED');
    expect(storedIdoFailure('SECRET raw_tx https://rpc')).not.toContain('SECRET');
    expect(storedIdoFailure(null)).toBe('—');
  });
  it('key IDO pages never render arbitrary error.message in their catch handlers',()=>{
    for (const page of ['projects/list.vue','projects/onchain_params.vue','approvals/index.vue','chain_transactions/index.vue','v3_token_admission/index.vue']){
      const source=readFileSync(resolve(process.cwd(),'apps/web-antd/src/views/joincare',page),'utf8');
      expect(source).not.toContain('error?.message');
      expect(source).not.toContain('error?.response?.data?.message');
    }
  });
});
