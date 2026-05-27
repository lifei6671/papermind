import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import { readStoredAccessToken } from "./session-token";

export type MemberRole = "space_admin" | "teacher" | "student";

export type SpaceMember = {
  id: number;
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
  adminName: string;
};

export type SpaceListResult = {
  items: SpaceRow[];
};

export type SpaceManagementAPI = {
  listSpaces(tenantID: number): Promise<SpaceListResult>;
  createSpace(input: CreateSpaceInput): Promise<SpaceRow>;
};

type SpaceMemberAPIResponse = {
  id: number;
  user_id: number;
  name: string;
  role: MemberRole;
  status: "enabled" | "disabled";
};

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
  getAccessToken: readStoredAccessToken,
});

export const spaceApi = createSpaceAPI(defaultApiClient);

export function createSpaceAPI(apiClient: ApiClient): SpaceManagementAPI {
  return {
    async listSpaces(tenantID) {
      const data = await apiClient.get<PageData<SpaceAPIResponse>>(`/api/v1/spaces?tenant_id=${tenantID}`);
      return { items: data.items.map((row) => mapSpaceResponse(row)) };
    },
    async createSpace(input) {
      const adminUserID = adminUserIDByName(input.adminName);
      const data = await apiClient.post<SpaceAPIResponse>("/api/v1/spaces", {
        tenant_id: input.tenantID,
        name: input.name,
        description: input.description,
        logo_url: input.logoFileName,
        type: "class",
        admin_user_ids: [adminUserID],
      });
      return mapSpaceResponse(data, input.adminName);
    },
  };
}

function mapSpaceResponse(row: SpaceAPIResponse, createdAdminName?: string): SpaceRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    name: row.name,
    description: row.description,
    logoFileName: row.logo_url || "未上传",
    members: row.members.map((member) => ({
      id: member.id,
      name: displayMemberName(member.name, createdAdminName),
      role: member.role,
      status: member.status,
    })),
  };
}

function displayMemberName(name: string, createdAdminName: string | undefined) {
  if (createdAdminName && name.startsWith("用户 ")) {
    return createdAdminName;
  }
  return name;
}

function adminUserIDByName(name: string) {
  const knownUsers: Record<string, number> = {
    李老师: 20,
    周老师: 21,
    赵老师: 22,
  };

  return knownUsers[name] ?? 20;
}
