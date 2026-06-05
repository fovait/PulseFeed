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
