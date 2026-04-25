import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { api } from "@/api";
import { ACCESS_TOKEN_KEY, REFRESH_TOKEN_KEY } from "@/api/config";
import { DEFAULT_MANAGER_TELEGRAM } from "@/lib/constants";

/** Minimal user shape consumed by the rest of the UI. */
export interface AuthUser {
  id: string;
  email: string;
  full_name?: string;
}

interface AuthContextType {
  user: AuthUser | null;
  loading: boolean;
  signUp: (email: string, password: string, fullName?: string) => Promise<{ error: string | null }>;
  signIn: (email: string, password: string) => Promise<{ error: string | null }>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

function storeTokens(access: string, refresh: string) {
  localStorage.setItem(ACCESS_TOKEN_KEY, access);
  localStorage.setItem(REFRESH_TOKEN_KEY, refresh);
}
function clearTokens() {
  localStorage.removeItem(ACCESS_TOKEN_KEY);
  localStorage.removeItem(REFRESH_TOKEN_KEY);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loading, setLoading] = useState(true);

  // Hydrate session on boot.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const session = await api.getSession();
        if (!cancelled) {
          setUser(session ? { id: session.user_id, email: session.email, full_name: session.full_name } : null);
        }
      } catch {
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const signUp = async (email: string, password: string, fullName?: string) => {
    try {
      const res = await api.signup({ email, password, full_name: fullName, manager_telegram: DEFAULT_MANAGER_TELEGRAM });
      storeTokens(res.access_token, res.refresh_token);
      setUser({ id: "mock-user", email: res.user.mail, full_name: res.user.name });
      return { error: null };
    } catch (e: any) {
      return { error: e?.message || "Sign up failed" };
    }
  };

  const signIn = async (email: string, password: string) => {
    try {
      const res = await api.login({ email, password });
      storeTokens(res.access_token, res.refresh_token);
      setUser({ id: "mock-user", email: res.user.mail, full_name: res.user.name });
      return { error: null };
    } catch (e: any) {
      return { error: e?.message || "Sign in failed" };
    }
  };

  const signOut = async () => {
    try { await api.logout(); } catch { /* ignore */ }
    clearTokens();
    setUser(null);
  };

  return (
    <AuthContext.Provider value={{ user, loading, signUp, signIn, signOut }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
