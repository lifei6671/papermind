import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import type { MemberRole, SpaceListResult } from "./spaces";
import type { TenantUserListResult, UserRole } from "./users";

export type TenantRow = {
  id: number;
  name: string;
  description: string;
  code: string;
  logoFileName: string;
  allowRegister: boolean;
  registerClosedLabel?: string;
};

export type CreateTenantInput = {
  name: string;
  description: string;
  logoFileName: string;
  allowRegister: boolean;
  adminUsername: string;
  adminRealName: string;
  adminPhone: string;
  adminEmail: string;
  adminPassword: string;
};

export type UpdateTenantProfileInput = {
  tenantID: number;
  name: string;
  description: string;
  logoFileName: string;
};

export type TenantListResult = {
  items: TenantRow[];
};

export type ListTenantsInput = {
  keyword?: string;
};

export type TenantManagementAPI = {
  listTenants(input?: ListTenantsInput): Promise<TenantListResult>;
  createTenant(input: CreateTenantInput): Promise<TenantRow>;
  updateTenantProfile(input: UpdateTenantProfileInput): Promise<TenantRow>;
  resetTenantCode(tenantID: number): Promise<TenantRow>;
  disableTenantRegistration(tenantID: number): Promise<TenantRow>;
  enableTenantRegistration(tenantID: number): Promise<TenantRow>;
  listTenantSpaces(tenantID: number): Promise<SpaceListResult>;
  listTenantUsers(tenantID: number): Promise<TenantUserListResult>;
};

type TenantAPIResponse = {
  id: number;
  name: string;
  logo_url: string;
  description: string;
  tenant_code: string;
  allow_register: boolean;
  status: "enabled" | "disabled";
};

type PlatformTenantSpaceMemberAPIResponse = {
  id: number;
  user_id: number;
  name: string;
  role: MemberRole;
  status: "enabled" | "disabled";
};

type PlatformTenantSpaceAPIResponse = {
  id: number;
  tenant_id: number;
  name: string;
  logo_url: string;
  description: string;
  status: "enabled" | "disabled";
  members: PlatformTenantSpaceMemberAPIResponse[];
};

type PlatformTenantUserAPIResponse = {
  id: number;
  tenant_id: number;
  username: string;
  real_name: string;
  avatar_url: string;
  role: UserRole;
  status: "enabled" | "disabled";
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const tenantApi = createTenantAPI(defaultApiClient);

export function createTenantAPI(apiClient: ApiClient): TenantManagementAPI {
  return {
    async listTenants(input) {
      const keyword = input?.keyword?.trim();
      const query = keyword ? `?keyword=${encodeURIComponent(keyword)}` : "";
      const data = await apiClient.get<PageData<TenantAPIResponse>>(`/api/v1/tenants${query}`);
      return { items: data.items.map(mapTenantResponse) };
    },
    async createTenant(input) {
      const data = await apiClient.post<TenantAPIResponse>("/api/v1/tenants", {
        name: input.name,
        description: input.description,
        logo_url: input.logoFileName,
        allow_register: input.allowRegister,
        admin_username: input.adminUsername,
        admin_real_name: input.adminRealName,
        admin_phone: input.adminPhone,
        admin_email: input.adminEmail,
        admin_password: input.adminPassword,
      });
      return mapTenantResponse(data);
    },
    async updateTenantProfile(input) {
      const data = await apiClient.post<TenantAPIResponse>(`/api/v1/tenants/${input.tenantID}/profile`, {
        name: input.name,
        description: input.description,
        logo_url: input.logoFileName === "未上传" ? "" : input.logoFileName,
      });
      return mapTenantResponse(data);
    },
    async resetTenantCode(tenantID) {
      const data = await apiClient.post<TenantAPIResponse>(`/api/v1/tenants/${tenantID}/reset-code`, {});
      return mapTenantResponse(data);
    },
    async disableTenantRegistration(tenantID) {
      return updateTenantRegisterSetting(apiClient, tenantID, false);
    },
    async enableTenantRegistration(tenantID) {
      return updateTenantRegisterSetting(apiClient, tenantID, true);
    },
    async listTenantSpaces(tenantID) {
      const data = await apiClient.get<PageData<PlatformTenantSpaceAPIResponse>>(`/api/v1/tenants/${tenantID}/spaces`);
      return {
        items: data.items.map((row) => ({
          id: row.id,
          tenantID: row.tenant_id,
          name: row.name,
          description: row.description,
          logoFileName: row.logo_url || "未上传",
          status: row.status,
          members: row.members.map((member) => ({
            id: member.id,
            userID: member.user_id,
            name: member.name,
            role: member.role,
            status: member.status,
          })),
        })),
      };
    },
    async listTenantUsers(tenantID) {
      const data = await apiClient.get<PageData<PlatformTenantUserAPIResponse>>(`/api/v1/tenants/${tenantID}/users`);
      return {
        items: data.items.map((row) => ({
          id: row.id,
          tenantID: row.tenant_id,
          name: row.real_name,
          username: row.username,
          role: row.role,
          avatarFileName: row.avatar_url || "未上传",
          status: row.status,
        })),
      };
    },
  };
}

async function updateTenantRegisterSetting(apiClient: ApiClient, tenantID: number, allowRegister: boolean) {
  const data = await apiClient.post<TenantAPIResponse>(`/api/v1/tenants/${tenantID}/register-setting`, {
    allow_register: allowRegister,
  });
  return mapTenantResponse(data);
}

function mapTenantResponse(row: TenantAPIResponse): TenantRow {
  return {
    id: row.id,
    name: row.name,
    description: row.description,
    code: row.tenant_code,
    logoFileName: row.logo_url || "未上传",
    allowRegister: row.allow_register,
    registerClosedLabel: row.allow_register ? undefined : "暂停注册",
  };
}
