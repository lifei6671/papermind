import { createApiClient } from "./client";
import type { ApiClient } from "./client";
import type { AuthSession, SessionRole } from "../auth/session-context";

export type PlatformLoginInput = {
  username: string;
  password: string;
};

export type TenantRegisterInput = {
  tenantCode: string;
  username: string;
  realName: string;
  password: string;
  phone?: string;
  email?: string;
};

export type TenantLoginInput = {
  tenantID: number;
  username: string;
  password: string;
};

export type TenantRegisterResult = {
  id: number;
  tenantID: number;
  username: string;
  realName: string;
  avatarURL: string;
  role: string;
  status: string;
};

export type AuthAPI = {
  platformLogin(input: PlatformLoginInput): Promise<AuthSession>;
  tenantLogin(input: TenantLoginInput): Promise<AuthSession>;
  tenantRegister(input: TenantRegisterInput): Promise<TenantRegisterResult>;
};

type AuthSessionAPIResponse = {
  access_token: string;
  refresh_token: string;
  user: {
    user_id: number;
    display_name: string;
    role: string;
    tenant_id?: number;
  };
};

type TenantRegisterAPIResponse = {
  id: number;
  tenant_id: number;
  username: string;
  real_name: string;
  avatar_url: string;
  role: string;
  status: string;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const authApi = createAuthAPI(defaultApiClient);

export function createAuthAPI(apiClient: ApiClient): AuthAPI {
  return {
    async platformLogin(input) {
      const data = await apiClient.post<AuthSessionAPIResponse>("/api/v1/auth/platform/login", {
        username: input.username,
        password: input.password,
      });
      return mapAuthSessionResponse(data);
    },
    async tenantLogin(input) {
      const data = await apiClient.post<AuthSessionAPIResponse>("/api/v1/auth/tenant/login", {
        tenant_id: input.tenantID,
        username: input.username,
        password: input.password,
      });
      return mapAuthSessionResponse(data);
    },
    async tenantRegister(input) {
      const data = await apiClient.post<TenantRegisterAPIResponse>("/api/v1/auth/tenant/register", {
        tenant_code: input.tenantCode,
        username: input.username,
        real_name: input.realName,
        password: input.password,
        phone: input.phone,
        email: input.email,
      });
      return mapTenantRegisterResponse(data);
    },
  };
}

function mapAuthSessionResponse(response: AuthSessionAPIResponse): AuthSession {
  return {
    accessToken: response.access_token,
    refreshToken: response.refresh_token,
    user: {
      userID: response.user.user_id,
      displayName: response.user.display_name,
      role: toSessionRole(response.user.role),
      tenantID: response.user.tenant_id,
    },
  };
}

function toSessionRole(role: string): SessionRole {
  switch (role) {
    case "platform_admin":
    case "tenant_admin":
    case "teacher":
    case "student":
      return role;
    default:
      throw new Error(`unsupported session role: ${role}`);
  }
}

function mapTenantRegisterResponse(response: TenantRegisterAPIResponse): TenantRegisterResult {
  return {
    id: response.id,
    tenantID: response.tenant_id,
    username: response.username,
    realName: response.real_name,
    avatarURL: response.avatar_url,
    role: response.role,
    status: response.status,
  };
}
