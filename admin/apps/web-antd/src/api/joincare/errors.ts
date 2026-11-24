/**
 * JoinCare admin 业务错误统一处理。
 *
 * 后端错误响应形如：{ status: false, code?: string, message?: string, error?: string, data?: any }
 * 这里只负责「识别稳定业务错误并生成用户可读文案」。
 * 错误识别与金额解析全部基于结构化 code / data / 响应体字段，
 * 绝不按 message 文本匹配或正则提取，避免误判与暴露底层细节（durable / estimateGas / VM Exception）。
 * 纯函数部分均可单测；i18n 文案通过 translate 参数注入，默认使用应用 $t。
 */

import { $t } from '@vben/locales';

import { formatIdoCode, idoErrorMessage } from './ido-errors';
import { formatV3Diagnostic, v3ErrorBody } from './v3-errors';

/** 稳定业务错误码：项目币储备不足（链上创建/激活时校验） */
export const PROJECT_TOKEN_RESERVE_INSUFFICIENT_CODE =
  'PROJECT_TOKEN_RESERVE_INSUFFICIENT';

/** 任何情况下都不应展示给用户的底层错误特征串（durable outbox / 链层异常等） */
const FORBIDDEN_INTERNAL_MARKERS: RegExp[] = [
  /durable/i,
  /estimate\s*gas/i,
  /vm\s*exception/i,
  /execution\s*reverted/i,
  /internal\s*json[-_ ]?rpc/i,
  /https?:\/\//i,
  /(?:password|private[_ ]?key|raw_tx|bearer\s)/i,
];

export type TranslateFn = (
  key: string,
  params?: Record<string, unknown>,
) => string;

const defaultTranslate: TranslateFn = (key, params) => $t(key, params ?? {});

/** 储备金额（项目币），值保持后端原样展示（可能是大数 decimal 字符串，避免精度损失） */
export interface TokenReserveAmounts {
  current?: number | string;
  minimum?: number | string;
  shortage?: number | string;
  decimals?: number;
  symbol?: string;
}

function asRecord(value: unknown): null | Record<string, any> {
  if (value && typeof value === 'object') {
    return value as Record<string, any>;
  }
  return null;
}

function firstString(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) {
      return value.trim();
    }
  }
  return '';
}

/** 归一化错误码：转大写并统一分隔符，容忍前后缀/连字符变体 */
export function normalizeErrorCode(code: unknown): string {
  return String(code ?? '')
    .trim()
    .toUpperCase()
    .replaceAll(/[\s.-]+/g, '_');
}

/** 是否为稳定业务错误 PROJECT_TOKEN_RESERVE_INSUFFICIENT（大小写/前后缀不敏感） */
export function isProjectTokenReserveInsufficient(code: unknown): boolean {
  return normalizeErrorCode(code).includes('TOKEN_RESERVE_INSUFFICIENT');
}

function pickAmount(
  sources: Array<Record<string, any>>,
  keys: string[],
): number | string | undefined {
  for (const source of sources) {
    for (const key of keys) {
      const value = source[key];
      if (value !== undefined && value !== null && value !== '') {
        return value as number | string;
      }
    }
  }
  return undefined;
}

/**
 * 从错误响应中提取 current / minimum / shortage 三个金额。
 * 仅解析结构化响应（data 对象 / 响应体根对象）；
 * 不从 message 文本正则匹配金额，避免误判与暴露底层细节。
 */
export function extractTokenReserveAmounts(
  responseBody: unknown,
): null | TokenReserveAmounts {
  const body = asRecord(responseBody);
  if (!body) {
    return null;
  }
  const data = asRecord(body.data);
  const sources = [data, body].filter(Boolean) as Array<Record<string, any>>;

  const current = pickAmount(sources, [
    'current',
    'current_reserve',
    'cur_reserve',
    'balance',
    'reserve_balance',
  ]);
  const minimum = pickAmount(sources, [
    'minimum',
    'min',
    'min_reserve',
    'minimum_reserve',
    'required',
    'required_reserve',
  ]);
  const shortage = pickAmount(sources, [
    'shortfall',
    'shortage',
    'deficit',
    'missing',
    'insufficient',
    'insufficient_amount',
    'gap',
  ]);

  if (
    current === undefined &&
    minimum === undefined &&
    shortage === undefined
  ) {
    return null;
  }
  const decimalsValue = pickAmount(sources, ['decimals']);
  const decimals = Number(decimalsValue);
  const symbol = firstString(data?.symbol, body.symbol);
  const result: TokenReserveAmounts = { current, minimum, shortage };
  if (Number.isInteger(decimals) && decimals >= 0 && decimals <= 255) {
    result.decimals = decimals;
  }
  if (symbol) result.symbol = symbol;
  return result;
}

/** 判断文案是否包含不应暴露的底层实现特征（durable outbox / estimateGas / VM Exception） */
export function containsForbiddenInternal(text: unknown): boolean {
  const value = String(text ?? '');
  return FORBIDDEN_INTERNAL_MARKERS.some((marker) => marker.test(value));
}

function formatRawTokenAmount(
  value: number | string | undefined,
  decimals?: number,
  symbol?: string,
): string | undefined {
  if (value === undefined) return undefined;
  const raw = String(value);
  if (decimals === undefined || !/^\d+$/.test(raw)) return raw;
  const padded = raw.padStart(decimals + 1, '0');
  const integerRaw = decimals === 0 ? padded : padded.slice(0, -decimals);
  const fractionRaw =
    decimals === 0 ? '' : padded.slice(-decimals).replace(/0+$/, '');
  const integer = integerRaw.replaceAll(/\B(?=(\d{3})+(?!\d))/g, ',');
  const display = fractionRaw ? `${integer}.${fractionRaw}` : integer;
  return symbol ? `${display} ${symbol}` : display;
}

/** 组装「储备不足」用户文案：有金额展示金额，无金额给通用提示 */
export function formatTokenReserveInsufficientMessage(
  amounts: null | TokenReserveAmounts,
  translate: TranslateFn = defaultTranslate,
): string {
  const labels: Array<[string | undefined, string]> = [
    [
      formatRawTokenAmount(
        amounts?.current,
        amounts?.decimals,
        amounts?.symbol,
      ),
      'projects.errors.currentLabel',
    ],
    [
      formatRawTokenAmount(
        amounts?.minimum,
        amounts?.decimals,
        amounts?.symbol,
      ),
      'projects.errors.minimumLabel',
    ],
    [
      formatRawTokenAmount(
        amounts?.shortage,
        amounts?.decimals,
        amounts?.symbol,
      ),
      'projects.errors.shortageLabel',
    ],
  ];
  const parts = labels
    .filter(([value]) => value !== undefined)
    .map(([value, key]) => `${translate(key)} ${value}`);
  if (parts.length === 0) {
    return translate('projects.errors.reserveInsufficientGeneric');
  }
  return `${translate('projects.errors.reserveInsufficientPrefix')}：${parts.join(
    '，',
  )}。${translate('projects.errors.reserveInsufficientSuffix')}`;
}

/**
 * 将后端错误响应体格式化为用户可读文案。
 * - 命中稳定业务错误（如 PROJECT_TOKEN_RESERVE_INSUFFICIENT）：返回可读文案（含金额），绝不回退到原始 message；
 * - 其它错误：返回 null，由调用方按原逻辑处理。
 */
export function formatProjectError(
  responseBody: unknown,
  translate: TranslateFn = defaultTranslate,
): null | string {
  const v3 = formatV3Diagnostic(responseBody);
  if (v3) return v3;
  const body = asRecord(responseBody);
  if (!body) {
    return null;
  }
  const code = firstString(body.code, body.error_code, body.err_code);
  if (isProjectTokenReserveInsufficient(code)) {
    return `${formatTokenReserveInsufficientMessage(
      extractTokenReserveAmounts(body),
      translate,
    )} 错误码：${PROJECT_TOKEN_RESERVE_INSUFFICIENT_CODE}`;
  }
  return formatIdoCode(body);
}

export function projectErrorMessage(error: unknown, fallback?: string): string {
  return formatProjectError(v3ErrorBody(error)) ?? idoErrorMessage(error, fallback);
}
