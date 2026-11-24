import { describe, expect, it, vi } from 'vitest';

import {
  activateProjectOnchain,
  createProjectOnchain,
  endProjectOnchain,
  getV3PolicyStatus,
  previewV3InitialPolicy,
  withdrawPlatformFee,
} from '../project_content';

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock('#/api/request', () => ({ requestClient: { get, post } }));

describe('project activation timeout', () => {
  it('allows the bounded V3 activation preflight below the dedicated proxy deadline', async () => {
    post.mockResolvedValueOnce({ submitted: false });
    await activateProjectOnchain('42950728196620289');
    expect(post).toHaveBeenCalledWith(
      '/v1/projects/42950728196620289/activate-onchain',
      undefined,
      { timeout: 160_000 },
    );
  });
  it('budgets create, settlement and fee withdrawal without retrying mutations', async () => {
    for (const [submit, route] of [
      [createProjectOnchain, 'create-onchain'],
      [endProjectOnchain, 'end'],
      [withdrawPlatformFee, 'withdraw-platform-fee'],
    ] as const) {
      post.mockReset();
      post.mockResolvedValueOnce({ submitted: true });
      await submit('42950728196620289');
      expect(post).toHaveBeenCalledTimes(1);
      expect(post).toHaveBeenCalledWith(
        `/v1/projects/42950728196620289/${route}`,
        undefined,
        { timeout: 160_000 },
      );
    }
  });
});

describe('v3 policy candidate preview timeout', () => {
  it('allows read-only historical chain verification without changing other requests', async () => {
    post.mockReset();
    post.mockResolvedValueOnce({ preview_digest: '0x1' });
    await previewV3InitialPolicy(
      '42950728196620289',
      ['0x1111111111111111111111111111111111111111'],
      true,
      'review',
    );
    expect(post).toHaveBeenCalledWith(
      '/v1/projects/42950728196620289/ido-v3/policy/preview',
      {
        wallets: ['0x1111111111111111111111111111111111111111'],
        enabled: true,
        reason: 'review',
      },
      { timeout: 170_000 },
    );
  });

  it('allows the read-only status request under the dedicated policy proxy budget', async () => {
    get.mockResolvedValueOnce({ project_state: 'pending' });
    await getV3PolicyStatus('42950728196620289');
    expect(get).toHaveBeenCalledWith(
      '/v1/projects/42950728196620289/ido-v3/policy/status',
      { timeout: 170_000 },
    );
  });
});
