import { createContext, useContext } from "react";

export const SESSION_STORAGE_KEY = "papermind.session.v1";

export type SessionRole = "platform_admin" | "tenant_user" | "tenant_admin" | "teacher" | "student";

export type ProfileSpaceAuthorization = {
  id: number;
  tenantID: number;
  tenantName?: string;
  spaceID: number;
  spaceName?: string;
  role: "tenant_admin" | "space_admin" | "teacher" | "student";
  status: "enabled" | "disabled";
};

export type SessionUser = {
  userID: number;
  displayName: string;
  role: SessionRole;
  tenantID?: number;
};

export type AuthSession = {
  accessToken: string;
  refreshToken: string;
  // 空间授权来自 profile 接口，只用于菜单和空间内操作判断，不能覆盖租户级 session role。
  profileSpaces?: ProfileSpaceAuthorization[];
  user: SessionUser;
};

export type SessionContextValue = {
  session: AuthSession | null;
  signIn: (session: AuthSession) => void;
  signOut: () => void;
};

export const SessionContext = createContext<SessionContextValue | null>(null);

export function useSession() {
  const value = useContext(SessionContext);
  if (!value) {
    throw new Error("useSession must be used within SessionProvider");
  }

  return value;
}
