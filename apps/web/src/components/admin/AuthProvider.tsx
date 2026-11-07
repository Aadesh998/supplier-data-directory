"use client";

import { usePathname, useRouter } from "next/navigation";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { ApiError } from "@/lib/api-error";
import { createAdminClient, SessionTokenStore, type AdminClient } from "@/lib/admin/client";
import type { AdminUser, Session } from "@/lib/types";

type AuthState = {
  user: AdminUser | null;
  csrfToken: string | null;
  api: AdminClient;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

type Props = {
  children: React.ReactNode;
  /** For tests: skip the /me check and use this session. */
  initialSession?: Session;
};

export function AuthProvider({ children, initialSession }: Props) {
  const router = useRouter();
  const pathname = usePathname();
  const [session, setSession] = useState<Session | null>(initialSession ?? null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">(initialSession ? "ready" : "loading");
  // Mutable, in-memory holder for the CSRF token (never persisted to storage).
  const [store] = useState(() => new SessionTokenStore(initialSession?.csrf_token ?? null));

  const api = useMemo(
    () =>
      createAdminClient(
        () => store.get(),
        () => {
          if (!store.beginRedirect()) return;
          const next = pathname && pathname.startsWith("/admin") ? `?next=${encodeURIComponent(pathname)}` : "";
          router.replace(`/login${next}`);
        },
      ),
    [store, pathname, router],
  );

  useEffect(() => {
    if (initialSession) return;
    let cancelled = false;
    api
      .get<Session>("/admin/me")
      .then((s) => {
        if (cancelled) return;
        store.set(s.csrf_token);
        setSession(s);
        setStatus("ready");
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) return; // redirect already triggered
        setStatus("error");
      });
    return () => {
      cancelled = true;
    };
  }, [api, initialSession, store]);

  const logout = useCallback(async () => {
    try {
      await api.post("/admin/logout");
    } catch {
      // ignore; we clear local state regardless
    }
    store.set(null);
    setSession(null);
    router.replace("/login");
  }, [api, router, store]);

  const value = useMemo<AuthState>(
    () => ({ user: session?.user ?? null, csrfToken: session?.csrf_token ?? null, api, logout }),
    [session, api, logout],
  );

  if (status === "loading") {
    return (
      <div className="grid min-h-screen place-items-center text-sm text-slate-700" role="status">
        Checking your session…
      </div>
    );
  }
  if (status === "error" || !session) {
    return (
      <div className="grid min-h-screen place-items-center px-4 text-center" role="alert">
        <div className="space-y-3">
          <p className="text-slate-800">We couldn&apos;t reach the admin API. Please try again.</p>
          <button type="button" className="btn-secondary" onClick={() => window.location.reload()}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside AuthProvider");
  return ctx;
}

export function useApi(): AdminClient {
  return useAuth().api;
}
