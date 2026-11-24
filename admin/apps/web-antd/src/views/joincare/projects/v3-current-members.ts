import type {
  V3PolicyPreview,
  V3PolicyReviewRow,
  V3PolicyStatus,
} from '#/api/joincare/project_content';

export interface V3CurrentMembers {
  members: string[];
  reviewId: string;
  root: string;
  version: string;
}

const same = (a: string, b: string) => a.toLowerCase() === b.toLowerCase();

export function sameV3Policy(a: V3PolicyStatus, b: V3PolicyStatus) {
  return (
    a.chain_id === b.chain_id &&
    a.project_id === b.project_id &&
    same(a.proxy, b.proxy) &&
    a.version === b.version &&
    a.enabled === b.enabled &&
    same(a.root, b.root)
  );
}

// Review detail is integrity checked by Backend (members/leaves/tree/digest).
// Only an observed execution matching the independently read live commitment
// can label its complete members as current. This is NOT proof eligibility.
export async function loadV3CurrentMembers(
  status: V3PolicyStatus,
  reviews: V3PolicyReviewRow[],
  read: (id: string) => Promise<V3PolicyPreview>,
  freshStatus: () => Promise<V3PolicyStatus>,
): Promise<V3CurrentMembers> {
  if (!status.enabled) throw new Error('policy_disabled');
  const candidates = reviews.filter(
    (r) =>
      r.project_id === status.project_id &&
      r.status === 'observed_not_proof' &&
      r.enabled &&
      r.target_version === status.version,
  );
  // Multiple observed reviews for one version are ambiguous; never guess.
  if (candidates.length !== 1) throw new Error('current_members_unavailable');
  const row = candidates[0];
  if (!row) throw new Error('current_members_unavailable');
  const p = await read(row.review_id);
  if (
    p.chain_id !== status.chain_id ||
    p.project_id !== status.project_id ||
    !same(p.proxy, status.proxy) ||
    p.target_version !== status.version ||
    !p.enabled ||
    !same(p.root, status.root) ||
    p.preview_digest !== row.preview_digest ||
    p.member_count !== row.member_count ||
    p.member_count < 1 ||
    p.member_count > 512 ||
    p.canonical_members.length !== p.member_count ||
    p.canonical_members.some(
      (w) => !/^0x[\da-f]{40}$/.test(w) || /^0x0{40}$/.test(w),
    ) ||
    new Set(p.canonical_members).size !== p.member_count
  ) {
    throw new Error('current_members_mismatch');
  }
  if (!sameV3Policy(status, await freshStatus()))
    throw new Error('policy_changed');
  return {
    members: [...p.canonical_members],
    reviewId: row.review_id,
    root: p.root,
    version: p.target_version,
  };
}
