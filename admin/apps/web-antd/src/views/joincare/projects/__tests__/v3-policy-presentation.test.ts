import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

const source = readFileSync(
  resolve(
    process.cwd(),
    'apps/web-antd/src/views/joincare/projects/v3_policy_preview.vue',
  ),
  'utf8',
);

describe('v3 policy governance presentation', () => {
  it('uses live Ant Design semantic tokens for surfaces, code and member diffs', () => {
    expect(source).toContain('theme.useToken()');
    expect(source).toContain(':style="policyTheme"');
    for (const token of [
      'colorBgContainer',
      'colorFillAlter',
      'colorText',
      'colorTextSecondary',
      'colorSuccessText',
      'colorSuccessBg',
      'colorErrorText',
      'colorErrorBg',
    ]) {
      expect(source).toContain(`themeToken.value.${token}`);
    }
    expect(source).toContain('background: var(--policy-surface)');
    expect(source).toContain('color: var(--policy-text)');
    expect(source).not.toMatch(
      /bg-green-50|bg-red-50|rgb\(245 247 250\)|rgb\(255 255 255/,
    );
    expect(source.match(/class="policy-diff policy-diff-added"/g)).toHaveLength(
      2,
    );
    expect(
      source.match(/class="policy-diff policy-diff-removed"/g),
    ).toHaveLength(2);
  });

  it('keeps safety-critical root and full members visible while folding technical detail', () => {
    expect(source).toContain('class="policy-root"');
    expect(source).toContain('result.canonical_members');
    expect(source).toContain('reviewed.canonical_members');
    expect(source).toContain('<details class="policy-details">');
    expect(source).toContain('函数 ABI 与精确 calldata');
    expect(source.match(/\{\{ v3PolicySafeAbi \}\}/g)).toHaveLength(1);
    expect(source.match(/\{\{ safeHandoff.calldata \}\}/g)).toHaveLength(1);
    expect(source).not.toContain(
      '完整名单替换，不是追加。Active 不可关闭白名单；审批不等于生效。',
    );
    expect(source).not.toContain('治理规则与当前链上身份');
  });

  it('provides responsive workflow navigation, scrollable review table and labelled inputs', () => {
    expect(source).toContain('aria-label="治理流程"');
    expect(source).toContain(':scroll="{ x: 920 }"');
    for (const id of [
      'policy-wallets',
      'policy-reason',
      'policy-tx-hash',
      'policy-safe-hash',
    ]) {
      expect(source).toContain(`for="${id}"`);
      expect(source).toContain(`id="${id}"`);
    }
    expect(source).toContain('@media (max-width: 600px)');
    expect(source).toContain('summary:focus-visible');
  });
});
