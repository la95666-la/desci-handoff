// A lost HTTP response is not evidence that a durable chain submission failed.
// Only these transport failures are ambiguous; business errors keep their
// normal handling. Never retry create-onchain automatically.
export function isAmbiguousOnchainCreateError(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false;
  const { code, response } = error as {
    code?: string;
    response?: {
      data?: { next_action?: string; submission_outcome?: string };
      status?: number;
    };
  };
  if (
    response?.data?.submission_outcome === 'signed_or_submitted' ||
    response?.data?.next_action === 'reconcile_original_do_not_resend'
  )
    return true;
  return (
    code === 'ECONNABORTED' ||
    code === 'ETIMEDOUT' ||
    code === 'ERR_NETWORK' ||
    response?.status === 502 ||
    response?.status === 503 ||
    response?.status === 504
  );
}
