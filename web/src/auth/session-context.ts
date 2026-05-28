import { createContext, useContext } from "react";

export const SESSION_STORAGE_KEY = "papermind.session.v1";

export type SessionRole = "platform_admin" | "tenant_admin" | "teacher" | "student";

export type SessionUser = {
  userID: number;
  displayName: string;
  role: SessionRole;
  tenantID?: number;
};

export type AuthSession = {
  accessToken: string;
  refreshToken: string;
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
