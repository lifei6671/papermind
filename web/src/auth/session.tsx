import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { API_UNAUTHORIZED_EVENT } from "../api/client";
import { SessionContext, SESSION_STORAGE_KEY } from "./session-context";
import type { AuthSession } from "./session-context";

type SessionProviderProps = {
  children: ReactNode;
};

export function SessionProvider({ children }: SessionProviderProps) {
  const [session, setSession] = useState<AuthSession | null>(() => readStoredSession());

  const signIn = useCallback((nextSession: AuthSession) => {
    const normalized = normalizeSession(nextSession);
    if (!normalized) {
      setSession(null);
      window.localStorage.removeItem(SESSION_STORAGE_KEY);
      return;
    }
    setSession(normalized);
    window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(normalized));
  }, []);

  const signOut = useCallback(() => {
    setSession(null);
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
  }, []);

  useEffect(() => {
    window.addEventListener(API_UNAUTHORIZED_EVENT, signOut);
    return () => window.removeEventListener(API_UNAUTHORIZED_EVENT, signOut);
  }, [signOut]);

  const value = useMemo(
    () => ({
      session,
      signIn,
      signOut,
    }),
    [session, signIn, signOut],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

function readStoredSession() {
  const rawSession = window.localStorage.getItem(SESSION_STORAGE_KEY);
  if (!rawSession) {
    return null;
  }

  try {
    const session = normalizeSession(JSON.parse(rawSession) as AuthSession);
    if (!session) {
      window.localStorage.removeItem(SESSION_STORAGE_KEY);
    }
    return session;
  } catch {
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
    return null;
  }
}

function normalizeSession(session: AuthSession | null): AuthSession | null {
  if (!session || !isSessionRole(session.user.role)) {
    return null;
  }
  return session;
}

function isSessionRole(role: string) {
  return role === "platform_admin" || role === "tenant_user" || role === "tenant_admin" || role === "teacher" || role === "student";
}
