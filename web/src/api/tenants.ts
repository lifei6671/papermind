import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import { readStoredAccessToken } from "./session-token";

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

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
  getAccessToken: readStoredAccessToken,
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
