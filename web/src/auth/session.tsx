import { useCallback, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { SessionContext, SESSION_STORAGE_KEY } from "./session-context";
import type { AuthSession } from "./session-context";

type SessionProviderProps = {
  children: ReactNode;
};

export function SessionProvider({ children }: SessionProviderProps) {
  const [session, setSession] = useState<AuthSession | null>(() => readStoredSession());

  const signIn = useCallback((nextSession: AuthSession) => {
    setSession(nextSession);
    window.localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(nextSession));
  }, []);

  const signOut = useCallback(() => {
    setSession(null);
    window.localStorage.removeItem(SESSION_STORAGE_KEY);
  }, []);

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

  return JSON.parse(rawSession) as AuthSession;
}
