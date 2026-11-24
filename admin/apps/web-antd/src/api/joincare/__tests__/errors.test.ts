import type { TranslateFn } from '../errors';

import { describe, expect, it } from 'vitest';

import zhPage from '../../../locales/langs/zh-CN/page.json';
import {
  containsForbiddenInternal,
  extractTokenReserveAmounts,
  formatProjectError,
  formatTokenReserveInsufficientMessage,
  isProjectTokenReserveInsufficient,
  normalizeErrorCode,
} from '../errors';

/** 用 zh-CN 真实语言包构造 translate，验证最终上屏文案 */
function zhTranslate(): TranslateFn {
  const messages = zhPage as Record<string, any>;
  return (key: string, params?: Record<string, unknown>) => {
    let value: any = messages;
    for (const part of key.split('.')) {
      value = value?.[part];
    }
    if (typeof value !== 'string') {
      return key;
    }
    return value.replaceAll(/\{(\w+)\}/g, (_match, name: string) =>
      String(params?.[name] ?? ''),
    );
  };
}

describe('normalizeErrorCode / isProjectTokenReserveInsufficient', () => {
  it('大小写、前后缀、连字符变体均能识别', () => {
    const variants = [
      'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      'project_token_reserve_insufficient',
      'ERR_PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      'err-project-token-reserve-insufficient',
      'Project.Token.Reserve.Insufficient',
    ];
    for (const code of variants) {
      expect(isProjectTokenReserveInsufficient(code)).toBe(true);
    }
    expect(isProjectTokenReserveInsufficient('PROJECT_BAD_ID')).toBe(false);
    expect(isProjectTokenReserveInsufficient(undefined)).toBe(false);
    expect(normalizeErrorCode('project-token reserve')).toBe(
      'PROJECT_TOKEN_RESERVE',
    );
  });
});

describe('extractTokenReserveAmounts', () => {
  it('从 data 子对象提取 current / minimum / shortage', () => {
    const body = {
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      data: { current: 100, minimum: 500, shortage: 400 },
    };
    expect(extractTokenReserveAmounts(body)).toEqual({
      current: 100,
      minimum: 500,
      shortage: 400,
    });
  });

  it('支持响应体根级金额字段（含大数 decimal 字符串，不丢精度）', () => {
    const body = {
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      current: '1200000000000000000000',
      minimum: '5000000000000000000000',
      shortage: '3800000000000000000000',
    };
    expect(extractTokenReserveAmounts(body)).toEqual({
      current: '1200000000000000000000',
      minimum: '5000000000000000000000',
      shortage: '3800000000000000000000',
    });
  });

  it('支持 data 与根级混合 + 别名键', () => {
    const body = {
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      data: { current_reserve: 10 },
      min_reserve: 30,
      deficit: 20,
    };
    expect(extractTokenReserveAmounts(body)).toEqual({
      current: 10,
      minimum: 30,
      shortage: 20,
    });
  });

  it('无金额时返回 null；无效 body 返回 null', () => {
    expect(extractTokenReserveAmounts({ code: 'X' })).toBeNull();
    expect(extractTokenReserveAmounts(null)).toBeNull();
    expect(extractTokenReserveAmounts('not-an-object')).toBeNull();
  });
});

describe('formatTokenReserveInsufficientMessage', () => {
  const t = zhTranslate();

  it('三个金额齐全时拼出「当前/最低要求/缺少」', () => {
    const message = formatTokenReserveInsufficientMessage(
      { current: 100, minimum: 500, shortage: 400 },
      t,
    );
    expect(message).toContain('项目币储备不足');
    expect(message).toContain('当前 100');
    expect(message).toContain('最低要求 500');
    expect(message).toContain('缺少 400');
  });

  it('金额不齐时只展示已有字段', () => {
    const message = formatTokenReserveInsufficientMessage(
      { current: '10.5', shortage: '3.5' },
      t,
    );
    expect(message).toContain('当前 10.5');
    expect(message).toContain('缺少 3.5');
    expect(message).not.toContain('最低要求');
  });

  it('无金额时回退到通用文案', () => {
    const message = formatTokenReserveInsufficientMessage(null, t);
    expect(message).toBe(
      '项目币储备不足，请补充项目币后重试。',
    );
  });
});

describe('formatProjectError', () => {
  const t = zhTranslate();

  it('命中码并从 data 显示金额，不暴露原始 message', () => {
    const body = {
      status: false,
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      message:
        'VM Exception: execution reverted: durable estimateGas failed, reserve_current=1 min=5',
      data: { current: 1, minimum: 5, shortage: 4 },
    };
    const message = formatProjectError(body, t);
    expect(message).toContain('项目币储备不足');
    expect(message).toContain('当前 1');
    expect(message).toContain('最低要求 5');
    expect(message).toContain('缺少 4');
    expect(message).not.toMatch(/durable|estimateGas|VM Exception/i);
    expect(message).not.toContain('execution reverted');
  });

  it('命中码但无金额时给通用文案，且不回退原始 message', () => {
    const body = {
      status: false,
      code: 'project_token_reserve_insufficient',
      message:
        'durable outbox estimateGas VM Exception cannot reserve token',
    };
    const message = formatProjectError(body, t);
    expect(message).toBe('项目币储备不足，请补充项目币后重试。 错误码：PROJECT_TOKEN_RESERVE_INSUFFICIENT');
    expect(message).not.toMatch(/durable|estimateGas|VM Exception/i);
  });

  it('不从 message 文本提取金额，避免解析或暴露内部错误', () => {
    const body = {
      status: false,
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      message: 'current 2.5, minimum 10, shortage 7.5',
    };
    expect(formatProjectError(body, t)).toBe(
      '项目币储备不足，请补充项目币后重试。 错误码：PROJECT_TOKEN_RESERVE_INSUFFICIENT',
    );
  });

  it('按 decimals 将真实后端 raw 响应格式化为 SMRT 可读金额', () => {
    const body = {
      status: false,
      code: 'PROJECT_TOKEN_RESERVE_INSUFFICIENT',
      message: 'project token reserve is insufficient',
      data: {
        current: '4000000000000000000000',
        required: '5000000000000000000000',
        shortfall: '1000000000000000000000',
        symbol: 'SMRT',
        decimals: 18,
      },
    };
    const message = formatProjectError(body, t);
    expect(message).toContain('当前 4,000 SMRT');
    expect(message).toContain('最低要求 5,000 SMRT');
    expect(message).toContain('缺少 1,000 SMRT');
    expect(message).not.toMatch(/4000000000000000000000|durable|estimateGas|VM Exception/i);
  });

  it('已知项目码显示中文及代码，未知错误不猜测原因', () => {
    expect(
      formatProjectError(
        { status: false, code: 'PROJECT_BAD_ID', message: 'invalid id' },
        t,
      ),
    ).toContain('错误码：PROJECT_BAD_ID');
    expect(formatProjectError({ status: true }, t)).toBeNull();
    expect(formatProjectError(null, t)).toBeNull();
  });
});

describe('containsForbiddenInternal', () => {
  it('识别 durable / estimateGas / VM Exception / execution reverted', () => {
    expect(containsForbiddenInternal('durable outbox submission')).toBe(true);
    expect(containsForbiddenInternal('estimateGas failed')).toBe(true);
    expect(containsForbiddenInternal('VM Exception while processing')).toBe(
      true,
    );
    expect(containsForbiddenInternal('execution reverted')).toBe(true);
    expect(containsForbiddenInternal('项目币储备不足')).toBe(false);
    expect(containsForbiddenInternal('')).toBe(false);
  });
});
