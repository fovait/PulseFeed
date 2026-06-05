import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { pulsefeedApi } from "../api/pulsefeed";
import { readSession, saveSession } from "../api/storage";
import type { Session } from "../types/api";
import { useToast } from "./useToast";

type AuthMode = "login" | "register";

type AuthContextValue = {
  session: Session | null;
  authMode: AuthMode;
  authOpen: boolean;
  authReason: string;
  openAuth: (reason?: string, mode?: AuthMode) => void;
  closeAuth: () => void;
  requireAuth: (reason?: string) => boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string) => Promise<void>;
  logout: () => void;
  setAuthMode: (mode: AuthMode) => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(() => readSession());
  const [authOpen, setAuthOpen] = useState(false);
  const [authMode, setAuthMode] = useState<AuthMode>("login");
  const [authReason, setAuthReason] = useState("登录后可以继续操作");
  const { pushToast } = useToast();

  const persistSession = useCallback((next: Session | null) => {
    setSession(next);
    saveSession(next);
  }, []);

  const openAuth = useCallback((reason = "登录后可以继续操作", mode: AuthMode = "login") => {
    setAuthReason(reason);
    setAuthMode(mode);
    setAuthOpen(true);
  }, []);

  const closeAuth = useCallback(() => setAuthOpen(false), []);

  const requireAuth = useCallback(
    (reason?: string) => {
      if (session?.token) return true;
      openAuth(reason || "请先登录");
      return false;
    },
    [openAuth, session?.token],
  );

  const login = useCallback(
    async (username: string, password: string) => {
      const next = await pulsefeedApi.login(username, password);
      persistSession(next);
      setAuthOpen(false);
      pushToast("登录成功", "success");
    },
    [persistSession, pushToast],
  );

  const register = useCallback(
    async (username: string, password: string) => {
      await pulsefeedApi.register(username, password);
      const next = await pulsefeedApi.login(username, password);
      persistSession(next);
      setAuthOpen(false);
      pushToast("注册并登录成功", "success");
    },
    [persistSession, pushToast],
  );

  const logout = useCallback(() => {
    persistSession(null);
    pushToast("已退出登录");
  }, [persistSession, pushToast]);

  useEffect(() => {
    const onExpired = () => {
      persistSession(null);
      openAuth("登录已过期，请重新登录");
      pushToast("登录已过期", "error");
    };
    window.addEventListener("pulsefeed:auth-expired", onExpired);
    return () => window.removeEventListener("pulsefeed:auth-expired", onExpired);
  }, [openAuth, persistSession, pushToast]);

  const value = useMemo(
    () => ({
      session,
      authMode,
      authOpen,
      authReason,
      openAuth,
      closeAuth,
      requireAuth,
      login,
      register,
      logout,
      setAuthMode,
    }),
    [authMode, authOpen, authReason, closeAuth, login, logout, openAuth, register, requireAuth, session],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}
