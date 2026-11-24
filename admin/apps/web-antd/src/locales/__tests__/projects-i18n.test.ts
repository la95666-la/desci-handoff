import { describe, expect, it } from 'vitest';

import enPage from '../langs/en-US/page.json';
import zhPage from '../langs/zh-CN/page.json';

type NestedMessages = Record<string, any>;

function flattenKeys(obj: NestedMessages, prefix = ''): string[] {
  const keys: string[] = [];
  for (const [key, value] of Object.entries(obj)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      keys.push(...flattenKeys(value, fullKey));
    } else {
      keys.push(fullKey);
    }
  }
  return keys;
}

function findValue(obj: NestedMessages, dottedKey: string): unknown {
  let current: any = obj;
  for (const part of dottedKey.split('.')) {
    current = current?.[part];
  }
  return current;
}

describe('projects i18n 语言包一致性', () => {
  const zhProjects = (zhPage as NestedMessages).projects ?? {};
  const enProjects = (enPage as NestedMessages).projects ?? {};

  it('en-US 与 zh-CN 的 projects 命名空间键完全一致', () => {
    const zhKeys = flattenKeys(zhProjects).toSorted();
    const enKeys = flattenKeys(enProjects).toSorted();
    expect(enKeys).toEqual(zhKeys);
  });

  it('projects 下每个键在两种语言下均为非空文案', () => {
    const zhKeys = flattenKeys(zhProjects);
    for (const key of zhKeys) {
      const zhValue = findValue(zhProjects, key);
      const enValue = findValue(enProjects, key);
      expect(typeof zhValue).toBe('string');
      expect(typeof enValue).toBe('string');
      expect(String(zhValue).trim()).not.toBe('');
      expect(String(enValue).trim()).not.toBe('');
    }
  });

  it('list 操作按钮文案齐全（创建/激活/启用恢复/禁用/删除）', () => {
    const zhList = (zhProjects as any).list ?? {};
    for (const key of [
      'createOnchain',
      'activateOnchain',
      'restore',
      'disable',
      'delete',
    ]) {
      expect(findValue(zhList, key)).toBeTruthy();
      expect(findValue(enProjects as any, `list.${key}`)).toBeTruthy();
    }
  });

  it('zh 恢复按钮按需求显示为「启用/恢复」', () => {
    expect(findValue(zhProjects, 'list.restore')).toBe('启用/恢复');
  });
});
