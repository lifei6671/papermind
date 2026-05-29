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
  username: string;
  password: string;
};

export type TenantSpaceSelectInput = {
  tenantID: number;
  spaceID?: number;
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

export type ProfileSpaceMembership = {
  id: number;
  tenantID: number;
  tenantName?: string;
  spaceID: number;
  spaceName?: string;
  role: "tenant_admin" | "space_admin" | "teacher" | "student";
  status: "enabled" | "disabled";
};

export type ProfileSpaceMembershipList = {
  items: ProfileSpaceMembership[];
};

export type AuthAPI = {
  platformLogin(input: PlatformLoginInput): Promise<AuthSession>;
  tenantLogin(input: TenantLoginInput): Promise<AuthSession>;
  selectTenantSpace(input: TenantSpaceSelectInput): Promise<AuthSession>;
  tenantRegister(input: TenantRegisterInput): Promise<TenantRegisterResult>;
  listProfileSpaces(): Promise<ProfileSpaceMembershipList>;
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

type ProfileSpaceMembershipAPIResponse = {
  id: number;
  tenant_id: number;
  tenant_name?: string;
  space_id: number;
  space_name?: string;
  role: ProfileSpaceMembership["role"];
  status: ProfileSpaceMembership["status"];
};

type ProfileSpaceMembershipListAPIResponse = {
  items: ProfileSpaceMembershipAPIResponse[];
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
        username: input.username,
        password: input.password,
      });
      return mapAuthSessionResponse(data);
    },
    async selectTenantSpace(input) {
      const data = await apiClient.post<AuthSessionAPIResponse>("/api/v1/auth/tenant/select-space", {
        tenant_id: input.tenantID,
        space_id: input.spaceID,
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
    async listProfileSpaces() {
      const data = await apiClient.get<ProfileSpaceMembershipListAPIResponse>("/api/v1/tenant/profile/spaces");
      return mapProfileSpaceMembershipList(data);
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
    case "tenant_user":
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

function mapProfileSpaceMembershipList(response: ProfileSpaceMembershipListAPIResponse): ProfileSpaceMembershipList {
  return {
    // 这些条目表示当前用户在 space_members 中的启用授权，不能写回 session.role。
    items: response.items.map((item) => ({
      id: item.id,
      tenantID: item.tenant_id,
      tenantName: item.tenant_name,
      spaceID: item.space_id,
      spaceName: item.space_name,
      role: item.role,
      status: item.status,
    })),
  };
}
