"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { apiGet, apiSend, type Me, type User } from "@/lib/api";

type Session =
  | { status: "loading"; user: null }
  | { status: "anonymous"; user: null }
  | { status: "signed-in"; user: Me };

type SessionApi = {
  session: Session;
  refresh: () => Promise<void>;
  signedIn: (user: User) => void;
  signOut: () => Promise<void>;
};

const SessionContext = createContext<SessionApi | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session>({ status: "loading", user: null });

  const refresh = useCallback(async () => {
    try {
      const user = await apiGet<Me>("/v1/auth/me");
      setSession({ status: "signed-in", user });
    } catch {
      setSession({ status: "anonymous", user: null });
    }
  }, []);

  useEffect(() => {
    let alive = true;
    apiGet<Me>("/v1/auth/me")
      .then((user) => alive && setSession({ status: "signed-in", user }))
      .catch(() => alive && setSession({ status: "anonymous", user: null }));
    return () => {
      alive = false;
    };
  }, []);

  const api = useMemo<SessionApi>(
    () => ({
      session,
      refresh,
      signedIn: (user) => setSession({ status: "signed-in", user: { ...user, login_method: "password" } }),
      signOut: async () => {
        try {
          await apiSend("POST", "/v1/auth/logout");
        } finally {
          setSession({ status: "anonymous", user: null });
        }
      },
    }),
    [session, refresh],
  );

  return <SessionContext.Provider value={api}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionApi {
  const api = useContext(SessionContext);
  if (!api) throw new Error("useSession outside <SessionProvider>");
  return api;
}
