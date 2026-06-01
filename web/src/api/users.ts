import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type UserRole = "tenant_admin" | "teacher" | "student";

export type TenantUserRow = {
  id: number;
  tenantID: number;
  name: string;
  username: string;
  role: UserRole;
  avatarFileName: string;
  status: "enabled" | "disabled";
};

export type CreateTenantUserInput = {
  tenantID: number;
  name: string;
  username: string;
  password: string;
  role: UserRole;
  avatarFileName: string;
};

export type UpdateTenantUserStatusInput = {
  tenantID: number;
  actorID: number;
  userID: number;
};

export type TenantUserListResult = {
  items: TenantUserRow[];
};

export type UserManagementAPI = {
  listUsers(tenantID: number): Promise<TenantUserListResult>;
  createUser(input: CreateTenantUserInput): Promise<TenantUserRow>;
  disableUser(input: UpdateTenantUserStatusInput): Promise<TenantUserRow>;
  enableUser(input: UpdateTenantUserStatusInput): Promise<TenantUserRow>;
};

type TenantUserAPIResponse = {
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

export const userApi = createUserAPI(defaultApiClient);

export function createUserAPI(apiClient: ApiClient): UserManagementAPI {
  return {
    async listUsers(tenantID) {
      const data = await apiClient.get<PageData<TenantUserAPIResponse>>(`/api/v1/tenant/users?tenant_id=${tenantID}`);
      return { items: data.items.map(mapUserResponse) };
    },
    async createUser(input) {
      const data = await apiClient.post<TenantUserAPIResponse>("/api/v1/tenant/users", {
        tenant_id: input.tenantID,
        username: input.username,
        real_name: input.name,
        avatar_url: input.avatarFileName,
        password: input.password,
        role: input.role,
      });
      return mapUserResponse(data);
    },
    async disableUser(input) {
      const data = await apiClient.post<TenantUserAPIResponse>(`/api/v1/tenant/users/${input.userID}/disable`, {
        tenant_id: input.tenantID,
        actor_id: input.actorID,
      });
      return mapUserResponse(data);
    },
    async enableUser(input) {
      const data = await apiClient.post<TenantUserAPIResponse>(`/api/v1/tenant/users/${input.userID}/enable`, {
        tenant_id: input.tenantID,
        actor_id: input.actorID,
      });
      return mapUserResponse(data);
    },
  };
}

function mapUserResponse(row: TenantUserAPIResponse): TenantUserRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    name: row.real_name,
    username: row.username,
    role: row.role,
    avatarFileName: row.avatar_url || "未上传",
    status: row.status,
  };
}
