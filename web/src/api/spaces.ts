import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type MemberRole = "space_admin" | "teacher" | "student";

export type SpaceMember = {
  id: number;
  userID: number;
  name: string;
  role: MemberRole;
  status: "enabled" | "disabled";
};

export type SpaceRow = {
  id: number;
  tenantID: number;
  name: string;
  description: string;
  logoFileName: string;
  members: SpaceMember[];
};

export type CreateSpaceInput = {
  tenantID: number;
  name: string;
  description: string;
  logoFileName: string;
  adminUserID: number;
};

export type SpaceListResult = {
  items: SpaceRow[];
};

export type SpaceMemberListInput = {
  tenantID: number;
  spaceID: number;
};

export type SpaceMemberCreateInput = SpaceMemberListInput & {
  userID: number;
  role: MemberRole;
};

export type SpaceMemberUpdateInput = SpaceMemberListInput & {
  userID: number;
  role?: MemberRole;
  status?: SpaceMember["status"];
};

export type SpaceMemberRemoveInput = SpaceMemberListInput & {
  userID: number;
};

export type SpaceMemberListResult = {
  items: SpaceMember[];
};

export type SpaceManagementAPI = {
  listSpaces(tenantID: number): Promise<SpaceListResult>;
  createSpace(input: CreateSpaceInput): Promise<SpaceRow>;
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
  members: SpaceMemberAPIResponse[];
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const spaceApi = createSpaceAPI(defaultApiClient);

export function createSpaceAPI(apiClient: ApiClient): SpaceManagementAPI & SpaceMemberAPI {
  return {
    async listSpaces(tenantID) {
      const data = await apiClient.get<PageData<SpaceAPIResponse>>(`/api/v1/tenant/spaces?tenant_id=${tenantID}`);
      return { items: data.items.map((row) => mapSpaceResponse(row)) };
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
    async listSpaceMembers(input) {
      const data = await apiClient.get<SpaceMemberListAPIResponse>(
        `/api/v1/tenant/spaces/${input.spaceID}/members?tenant_id=${input.tenantID}`,
      );
      return { items: mapSpaceMemberListResponse(data) };
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
    role: member.role,
    status: member.status,
  };
}
