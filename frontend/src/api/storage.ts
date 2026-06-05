import type { Session } from "../types/api";

export const TOKEN_STORAGE_KEY = "pulsefeed.token";
export const SESSION_STORAGE_KEY = "pulsefeed.session";

export function readSession(): Session | null {
  try {
    const raw = localStorage.getItem(SESSION_STORAGE_KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

export function saveSession(session: Session | null) {
  if (session?.token) {
    localStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
    localStorage.setItem(TOKEN_STORAGE_KEY, session.token);
    return;
  }
  localStorage.removeItem(SESSION_STORAGE_KEY);
  localStorage.removeItem(TOKEN_STORAGE_KEY);
}

export function readToken() {
  return localStorage.getItem(TOKEN_STORAGE_KEY) || readSession()?.token || "";
}

export function readRefreshToken(): string {
  return readSession()?.refresh_token || "";
}

/** refresh 成功后用新 access token 更新本地 session，其余字段不变 */
export function updateStoredToken(token: string, accountID?: number, username?: string) {
  const session = readSession();
  if (!session) return;
  saveSession({
    ...session,
    token,
    ...(accountID ? { account_id: accountID } : {}),
    ...(username ? { username } : {}),
  });
}
