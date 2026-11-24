import { requestClient } from '#/api/request';

export interface TwitterVerificationSettings {
  verificationDisabled: boolean;
  updatedAt: number;
}

export function getTwitterVerificationSettings() {
  return requestClient.get<TwitterVerificationSettings>(
    '/v1/twitter-verification-settings',
  );
}

export function updateTwitterVerificationSettings(
  verificationDisabled: boolean,
) {
  return requestClient.put<TwitterVerificationSettings>(
    '/v1/twitter-verification-settings',
    { verificationDisabled },
  );
}
