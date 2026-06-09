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
  forcePasswordChange?: boolean;
  phone?: string;
  email?: string;
  lastLoginIP?: string;
  lastLoginAt?: number;
  createdAt?: number;
  updatedAt?: number;
};

export type CreateTenantUserInput = {
  tenantID: number;
  name: string;
  username: string;
  password: string;
  role: UserRole;
  avatarFileName: string;
  forcePasswordChange: boolean;
};

export type UpdateTenantUserProfileInput = {
  tenantID: number;
  userID: number;
  name: string;
  phone: string;
  email: string;
};

export type UpdateTenantUserStatusInput = {
  tenantID: number;
  actorID: number;
  userID: number;
};

export type TenantUserListResult = {
  items: TenantUserRow[];
  page?: number;
  pageSize?: number;
  total?: number;
};

export type ListTenantUsersFilters = {
  role?: UserRole;
  status?: TenantUserRow["status"];
  excludeSpaceID?: number;
};

export type ListTenantUsersInput = {
  tenantID: number;
  page?: number;
  pageSize?: number;
  search?: string;
  filters?: ListTenantUsersFilters;
};

export type UserManagementAPI = {
  listUsers(input: ListTenantUsersInput): Promise<TenantUserListResult>;
  createUser(input: CreateTenantUserInput): Promise<TenantUserRow>;
  updateUser(input: UpdateTenantUserProfileInput): Promise<TenantUserRow>;
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
  force_password_change?: boolean;
  phone?: string;
  email?: string;
  last_login_ip?: string;
  last_login_at?: number;
  created_at?: number;
  updated_at?: number;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const userApi = createUserAPI(defaultApiClient);

export function createUserAPI(apiClient: ApiClient): UserManagementAPI {
  return {
    async listUsers(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      if (input.page !== undefined) {
        params.set("page", String(input.page));
      }
      if (input.pageSize !== undefined) {
        params.set("page_size", String(input.pageSize));
      }
      const keyword = input.search?.trim();
      if (keyword) {
        params.set("search", keyword);
      }
      if (input.filters?.role) {
        params.set("role", input.filters.role);
      }
      if (input.filters?.status) {
        params.set("status", input.filters.status);
      }
      if (input.filters?.excludeSpaceID !== undefined) {
        params.set("exclude_space_id", String(input.filters.excludeSpaceID));
      }
      const data = await apiClient.get<PageData<TenantUserAPIResponse>>(`/api/v1/tenant/users?${params.toString()}`);
      return {
        items: data.items.map(mapUserResponse),
        page: data.page,
        pageSize: data.page_size,
        total: data.total,
      };
    },
    async createUser(input) {
      const data = await apiClient.post<TenantUserAPIResponse>("/api/v1/tenant/users", {
        tenant_id: input.tenantID,
        username: input.username,
        real_name: input.name,
        avatar_url: input.avatarFileName,
        password: input.password,
        role: input.role,
        force_password_change: input.forcePasswordChange,
      });
      return mapUserResponse(data);
    },
    async updateUser(input) {
      const data = await apiClient.request<TenantUserAPIResponse>(`/api/v1/tenant/users/${input.userID}/profile`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          real_name: input.name,
          phone: input.phone,
          email: input.email,
        },
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
    forcePasswordChange: row.force_password_change ?? false,
    phone: row.phone,
    email: row.email,
    lastLoginIP: row.last_login_ip,
    lastLoginAt: row.last_login_at ?? 0,
    createdAt: row.created_at ?? 0,
    updatedAt: row.updated_at ?? 0,
  };
}
