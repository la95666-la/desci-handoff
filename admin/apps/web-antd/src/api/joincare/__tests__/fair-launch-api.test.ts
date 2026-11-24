import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  fairLaunchApiPaths,
  isFairLaunchCapabilityUnavailable,
} from '../fair-launch-contract';
import {
  executeFairLaunchAction,
  getFairLaunchCapabilities,
  getFairLaunchSet,
  previewFairLaunchAction,
  submitFairLaunchAction,
} from '../fair_launch';

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock('#/api/request', () => ({ requestClient: { get, post } }));

describe('fair Launch Admin API seam', () => {
  beforeEach(() => {
    get.mockReset();
    post.mockReset();
  });

  it('keeps every endpoint below the trusted project fair-launch namespace', () => {
    const paths = [
      fairLaunchApiPaths.capability(9),
      fairLaunchApiPaths.currentSet(9),
      fairLaunchApiPaths.lifecycle(9, 'release'),
      fairLaunchApiPaths.pause(9, 'pause_claims'),
      fairLaunchApiPaths.governance(9, 'set_operator'),
    ];
    for (const path of paths) {
      expect(path).toMatch(/^\/v1\/projects\/9\/fair-launch\//);
      expect(path).not.toMatch(/0x[a-f\d]{40}/i);
    }
  });

  it('does not export Fair Launch Set mutation API paths', () => {
    expect(fairLaunchApiPaths).not.toHaveProperty('previewSet');
    expect(fairLaunchApiPaths).not.toHaveProperty('submitSet');
  });

  it('classifies absent backend capabilities as fail-closed', () => {
    expect(
      isFairLaunchCapabilityUnavailable({ response: { status: 404 } }),
    ).toBe(true);
    expect(
      isFairLaunchCapabilityUnavailable({ response: { status: 405 } }),
    ).toBe(true);
    expect(
      isFairLaunchCapabilityUnavailable({ response: { status: 501 } }),
    ).toBe(true);
    expect(
      isFairLaunchCapabilityUnavailable({ response: { status: 500 } }),
    ).toBe(false);
  });

  it('uses requestClient data-return contract for read and audited action seams', async () => {
    get
      .mockResolvedValueOnce({ available: true })
      .mockResolvedValueOnce({ configured: false });
    post.mockResolvedValue({ canonical_digest: '0xabc' });

    await getFairLaunchCapabilities(9);
    await getFairLaunchSet(9);
    await previewFairLaunchAction(9, 'end', {});
    await submitFairLaunchAction(9, 'end', {
      canonical_digest: '0xabc',
      evidence: {},
      mfa_assertion_token: 'mfa-token-32-bytes-long-value-123',
      preview_token: 'preview-token-32-bytes-long-value',
    });
    await executeFairLaunchAction(9, 11, {
      mfa_assertion_token: 'mfa-token-32-bytes-long-value-456',
    });

    expect(get).toHaveBeenCalledTimes(2);
    expect(post).toHaveBeenCalledTimes(3);
    expect(post.mock.calls.at(-1)?.[1]).not.toHaveProperty(
      'runtime_generation',
    );
    expect(post.mock.calls.at(-1)?.[1]).not.toHaveProperty('config_generation');
  });
});
