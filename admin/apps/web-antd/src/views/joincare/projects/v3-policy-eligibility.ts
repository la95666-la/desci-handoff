import type { OnchainParamsCurrent } from '#/api/joincare/project_content';

/** Read-only policy inspection includes terminal projects; it grants no mutation. */
export function isV3PolicyReadableProject(
  current: OnchainParamsCurrent,
): boolean {
  return (
    current.contract_version === 'ido_v3_stablecoin' &&
    current.funding_mode === 'capped_presale' &&
    current.locked === true &&
    [
      'Active',
      'Canceled',
      'Cancelled',
      'Failed',
      'Pending',
      'Successful',
      'V3ReorgHalted',
    ].includes(current.status)
  );
}

/** UI precondition only. Backend independently verifies the original receipt, cursor and Safe. */
export function isV3PolicyPreviewProjectCreated(
  current: OnchainParamsCurrent,
): boolean {
  return (
    current.contract_version === 'ido_v3_stablecoin' &&
    current.funding_mode === 'capped_presale' &&
    current.locked === true &&
    (current.status === 'Pending' || current.status === 'Active')
  );
}
