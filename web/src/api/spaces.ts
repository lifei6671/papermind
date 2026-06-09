import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type MemberRole = "space_admin" | "teacher" | "student";

export type SpaceMember = {
  id: number;
  userID: number;
  name: string;
  username?: string;
  phone?: string;
  email?: string;
  registeredAt?: number;
  registerMethod?: string;
  role: MemberRole;
  status: "enabled" | "disabled";
};

export type SpaceRow = {
  id: number;
  tenantID: number;
  name: string;
  description: string;
  logoFileName: string;
  status: "enabled" | "disabled";
  members: SpaceMember[];
};

export type CreateSpaceInput = {
  tenantID: number;
  name: string;
  description: string;
  logoFileName: string;
  adminUserID: number;
};

export type UpdateSpaceInput = {
  tenantID: number;
  spaceID: number;
  name: string;
  description: string;
  logoFileName: string;
};

export type DisableSpaceInput = {
  tenantID: number;
  spaceID: number;
};

export type SpaceListResult = {
  items: SpaceRow[];
  page?: number;
  pageSize?: number;
  total?: number;
};

export type ListSpacesFilters = {
  status?: SpaceRow["status"];
};

export type ListSpacesInput = {
  tenantID: number;
  page?: number;
  pageSize?: number;
  search?: string;
  filters?: ListSpacesFilters;
};

export type SpaceMemberListInput = {
  tenantID: number;
  spaceID: number;
  page: number;
  pageSize: number;
  search?: string;
  role?: MemberRole;
  status?: SpaceMember["status"];
};

type SpaceMemberMutationInput = {
  tenantID: number;
  spaceID: number;
};

export type SpaceMemberCreateInput = SpaceMemberMutationInput & {
  userID: number;
  role: MemberRole;
};

export type SpaceMemberUpdateInput = SpaceMemberMutationInput & {
  userID: number;
  role?: MemberRole;
  status?: SpaceMember["status"];
};

export type SpaceMemberRemoveInput = SpaceMemberMutationInput & {
  userID: number;
};

export type SpaceMemberListResult = {
  items: SpaceMember[];
  page?: number;
  pageSize?: number;
  total?: number;
};

export type SpaceManagementAPI = {
  listSpaces(input: ListSpacesInput): Promise<SpaceListResult>;
  createSpace(input: CreateSpaceInput): Promise<SpaceRow>;
  updateSpace(input: UpdateSpaceInput): Promise<SpaceRow>;
  disableSpace(input: DisableSpaceInput): Promise<SpaceRow>;
};

export type SpaceMemberAPI = {
  listSpaceMembers(input: SpaceMemberListInput): Promise<SpaceMemberListResult>;
  createSpaceMember(input: SpaceMemberCreateInput): Promise<SpaceMember>;
  updateSpaceMember(input: SpaceMemberUpdateInput): Promise<SpaceMember>;
  removeSpaceMember(input: SpaceMemberRemoveInput): Promise<void>;
};

type SpaceMemberAPIResponse = {
  id: number;
  user_id: number;
  name: string;
  username?: string;
  phone?: string;
  email?: string;
  created_at?: number;
  register_method?: string;
  role: MemberRole;
  status: "enabled" | "disabled";
};

type SpaceMemberListAPIResponse = PageData<SpaceMemberAPIResponse> | SpaceMemberAPIResponse[];

type SpaceAPIResponse = {
  id: number;
  tenant_id: number;
  name: string;
  logo_url: string;
  description: string;
  status?: "enabled" | "disabled";
  members: SpaceMemberAPIResponse[];
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const spaceApi = createSpaceAPI(defaultApiClient);

export function createSpaceAPI(apiClient: ApiClient): SpaceManagementAPI & SpaceMemberAPI {
  return {
    async listSpaces(input) {
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
      if (input.filters?.status) {
        params.set("status", input.filters.status);
      }
      const data = await apiClient.get<PageData<SpaceAPIResponse>>(`/api/v1/tenant/spaces?${params.toString()}`);
      return {
        items: data.items.map((row) => mapSpaceResponse(row)),
        page: data.page,
        pageSize: data.page_size,
        total: data.total,
      };
    },
    async createSpace(input) {
      const data = await apiClient.post<SpaceAPIResponse>("/api/v1/tenant/spaces", {
        tenant_id: input.tenantID,
        name: input.name,
        description: input.description,
        logo_url: input.logoFileName,
        type: "class",
        admin_user_ids: [input.adminUserID],
      });
      return mapSpaceResponse(data);
    },
    async updateSpace(input) {
      const data = await apiClient.request<SpaceAPIResponse>(`/api/v1/tenant/spaces/${input.spaceID}`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          name: input.name,
          description: input.description,
          logo_url: input.logoFileName,
          type: "class",
        },
      });
      return mapSpaceResponse(data);
    },
    async disableSpace(input) {
      const data = await apiClient.post<SpaceAPIResponse>(`/api/v1/tenant/spaces/${input.spaceID}/disable`, {
        tenant_id: input.tenantID,
      });
      return mapSpaceResponse(data);
    },
    async listSpaceMembers(input) {
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
      if (input.role) {
        params.set("role", input.role);
      }
      if (input.status) {
        params.set("status", input.status);
      }
      const data = await apiClient.get<SpaceMemberListAPIResponse>(
        `/api/v1/tenant/spaces/${input.spaceID}/members?${params.toString()}`,
      );
      return {
        items: mapSpaceMemberListResponse(data),
        ...(Array.isArray(data) ? {} : { page: data.page, pageSize: data.page_size, total: data.total }),
      };
    },
    async createSpaceMember(input) {
      const data = await apiClient.post<SpaceMemberAPIResponse>(`/api/v1/tenant/spaces/${input.spaceID}/members`, {
        tenant_id: input.tenantID,
        user_id: input.userID,
        role: input.role,
      });
      return mapSpaceMemberResponse(data);
    },
    async updateSpaceMember(input) {
      const data = await apiClient.request<SpaceMemberAPIResponse>(
        `/api/v1/tenant/spaces/${input.spaceID}/members/${input.userID}`,
        {
          method: "PUT",
          body: {
            tenant_id: input.tenantID,
            role: input.role,
            status: input.status,
          },
        },
      );
      return mapSpaceMemberResponse(data);
    },
    async removeSpaceMember(input) {
      await apiClient.request(`/api/v1/tenant/spaces/${input.spaceID}/members/${input.userID}?tenant_id=${input.tenantID}`, {
        method: "DELETE",
      });
    },
  };
}

function mapSpaceResponse(row: SpaceAPIResponse): SpaceRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    name: row.name,
    description: row.description,
    logoFileName: row.logo_url || "未上传",
    status: row.status ?? "enabled",
    members: row.members.map((member) => mapSpaceMemberResponse(member)),
  };
}

function mapSpaceMemberListResponse(data: SpaceMemberListAPIResponse) {
  const items = Array.isArray(data) ? data : data.items;
  return items.map((member) => mapSpaceMemberResponse(member));
}

function mapSpaceMemberResponse(member: SpaceMemberAPIResponse): SpaceMember {
  return {
    id: member.id,
    userID: member.user_id,
    name: member.name,
    username: member.username,
    phone: member.phone,
    email: member.email,
    registeredAt: member.created_at,
    registerMethod: member.register_method,
    role: member.role,
    status: member.status,
  };
}
