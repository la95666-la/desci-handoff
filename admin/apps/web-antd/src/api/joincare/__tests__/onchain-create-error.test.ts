import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { isAmbiguousOnchainCreateError } from '../onchain-create-error';

describe('create-onchain lost response', () => {
  it('treats only transport failures as ambiguous, never business rejections', () => {
    for (const code of ['ECONNABORTED', 'ETIMEDOUT', 'ERR_NETWORK']) {
      expect(isAmbiguousOnchainCreateError({ code })).toBe(true);
    }
    for (const status of [502, 503, 504]) {
      expect(isAmbiguousOnchainCreateError({ response: { status } })).toBe(
        true,
      );
    }
    expect(isAmbiguousOnchainCreateError({ response: { status: 400 } })).toBe(
      false,
    );
    expect(isAmbiguousOnchainCreateError({ response: { status: 409 } })).toBe(
      false,
    );
    expect(isAmbiguousOnchainCreateError(null)).toBe(false);
  });

  it('allows the V3 backend to finish while keeping the client inside the proxy budget', () => {
    const api = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/api/joincare/project_content.ts',
      ),
      'utf8',
    );
    const create = api
      .split('export function createProjectOnchain(')[1]
      ?.split('export function activateProjectOnchain(')[0];
    expect(create).toContain('{ timeout: 160_000 }');
    const list = readFileSync(
      resolve(
        process.cwd(),
        'apps/web-antd/src/views/joincare/projects/list.vue',
      ),
      'utf8',
    );
    expect(list).toContain('idoErrorMessage(error');
    expect(list).toContain('切勿重复提交');
    expect(list).toContain('await load();');
  });
});
