import { requestClient } from '#/api/request';

export interface Role {
  id: number;
  code: string;
  name: string;
  description: string;
  is_built_in: number;
  sort: number;
}

export interface Permission {
  id: number;
  role_code: string;
  module: string;
  action: string;
  scope: string;
}

export function listRoles() {
  return requestClient.get<{ items: Role[] }>('/v1/roles');
}

export function getRolePermissions(code: string) {
  return requestClient.get<{ items: Permission[]; role_code: string }>(
    `/v1/roles/${code}/permissions`,
  );
}

export function setRolePermissions(
  code: string,
  permissions: Array<{ action: string; module: string; scope?: string }>,
) {
  return requestClient.put<{ count: number; role_code: string }>(
    `/v1/roles/${code}/permissions`,
    { permissions },
  );
}

export function myPermissions() {
  try {
    const baseUrl = new URL(
      String(import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_URL || ''),
    );
    const runId = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_RUN_ID || '',
    );
    const token = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_TOKEN || '',
    );
    const expiresAt = String(
      import.meta.env.VITE_FAIR_LAUNCH_LOCAL_API_EXPIRES_AT || '',
    );
    if (
      !import.meta.env.PROD &&
      import.meta.env.VITE_ENVIRONMENT === 'local' &&
      baseUrl.protocol === 'http:' &&
      baseUrl.hostname === '127.0.0.1' &&
      /^\d+$/.test(baseUrl.port) &&
      /^[0-9a-f-]{36}$/.test(runId) &&
      /^[0-9a-f]{64}$/.test(token) &&
      Date.now() < Date.parse(expiresAt)
    ) {
      return fetch(new URL('/admin/v1/my/permissions', baseUrl.origin), {
        headers: {
          'X-JC-FL-Run-ID': runId,
          'X-JC-FL-Capability': token,
          'X-JC-FL-Origin': String(
            import.meta.env.VITE_FAIR_LAUNCH_LOCAL_ADMIN_ORIGIN || '',
          ),
        },
      }).then(async (response) => {
        const result = await response.json();
        if (!response.ok || result?.status !== true)
          throw new Error(result?.message || 'materialization_unavailable');
        return result.data as {
          is_super: boolean;
          permissions: string[];
          roles: string[];
        };
      });
    }
  } catch {
    /* fall through to the ordinary Admin API */
  }
  return requestClient.get<{
    is_super: boolean;
    permissions: string[];
    roles: string[];
  }>('/v1/my/permissions');
}
