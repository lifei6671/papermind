import { createApiClient } from "./client";
import type { ApiClient } from "./client";
import type { AuthSession } from "../auth/session-context";

export type PlatformLoginInput = {
  username: string;
  password: string;
};

export type AuthAPI = {
  platformLogin(input: PlatformLoginInput): Promise<AuthSession>;
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
  };
}

function mapAuthSessionResponse(response: AuthSessionAPIResponse): AuthSession {
  return {
    accessToken: response.access_token,
    refreshToken: response.refresh_token,
    user: {
      userID: response.user.user_id,
      displayName: response.user.display_name,
      role: response.user.role,
      tenantID: response.user.tenant_id,
    },
  };
}
