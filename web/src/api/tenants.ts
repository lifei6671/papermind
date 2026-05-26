import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

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
};

export type TenantListResult = {
  items: TenantRow[];
};

export type TenantManagementAPI = {
  listTenants(): Promise<TenantListResult>;
  createTenant(input: CreateTenantInput): Promise<TenantRow>;
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
});

export const tenantApi = createTenantAPI(defaultApiClient);

export function createTenantAPI(apiClient: ApiClient): TenantManagementAPI {
  return {
    async listTenants() {
      const data = await apiClient.get<PageData<TenantAPIResponse>>("/api/v1/tenants");
      return { items: data.items.map(mapTenantResponse) };
    },
    async createTenant(input) {
      const data = await apiClient.post<TenantAPIResponse>("/api/v1/tenants", {
        name: input.name,
        description: input.description,
        logo_url: input.logoFileName,
      });
      return mapTenantResponse(data);
    },
  };
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
