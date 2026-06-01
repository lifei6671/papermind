import { createApiClient } from "./client";
import type { ApiClient } from "./client";

export type Profile = {
  userID: number;
  tenantID?: number;
  displayName: string;
  avatarURL: string;
  phone: string;
  email: string;
  role: string;
  subjectType: string;
  forcePasswordChange: boolean;
};

export type UpdateProfileInput = {
  displayName: string;
  avatarURL: string;
  phone: string;
  email: string;
};

export type ChangePasswordInput = {
  currentPassword: string;
  newPassword: string;
};

export type ProfileAPI = {
  getProfile(): Promise<Profile>;
  updateProfile(input: UpdateProfileInput): Promise<Profile>;
  changePassword(input: ChangePasswordInput): Promise<Profile>;
};

type ProfileAPIResponse = {
  user_id: number;
  tenant_id?: number;
  display_name: string;
  avatar_url: string;
  phone: string;
  email: string;
  role: string;
  subject_type: string;
  force_password_change?: boolean;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const profileApi = createProfileAPI(defaultApiClient);

export function createProfileAPI(apiClient: ApiClient): ProfileAPI {
  return {
    async getProfile() {
      const data = await apiClient.get<ProfileAPIResponse>("/api/v1/profile");
      return mapProfile(data);
    },
    async updateProfile(input) {
      const data = await apiClient.post<ProfileAPIResponse>("/api/v1/profile", {
        display_name: input.displayName,
        avatar_url: input.avatarURL,
        phone: input.phone,
        email: input.email,
      });
      return mapProfile(data);
    },
    async changePassword(input) {
      const data = await apiClient.post<ProfileAPIResponse>("/api/v1/profile/password", {
        current_password: input.currentPassword,
        new_password: input.newPassword,
      });
      return mapProfile(data);
    },
  };
}

function mapProfile(response: ProfileAPIResponse): Profile {
  return {
    userID: response.user_id,
    tenantID: response.tenant_id,
    displayName: response.display_name,
    avatarURL: response.avatar_url,
    phone: response.phone,
    email: response.email,
    role: response.role,
    subjectType: response.subject_type,
    forcePasswordChange: response.force_password_change ?? false,
  };
}
