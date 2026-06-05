import { readRefreshToken, readToken, saveSession, updateStoredToken } from "./storage";

export class ApiError extends Error {
  status: number;
  payload: unknown;

  constructor(message: string, status: number, payload: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.payload = payload;
  }
}

type RequestOptions = {
  auth?: boolean;
  body?: unknown;
  method?: "GET" | "POST";
  signal?: AbortSignal;
  /** 内部标记：是否已经是重试请求，防止无限循环 */
  _retry?: boolean;
};

function normalizeBaseUrl(value: string) {
  return value.replace(/\/+$/, "");
}

export const API_BASE_URL = normalizeBaseUrl(import.meta.env.VITE_API_BASE_URL || "/api");

export class ApiClient {
  baseUrl: string;
  /** 同一时刻只发一个 refresh 请求，避免并发 401 时重复刷 */
  private refreshPromise: Promise<string | null> | null = null;

  constructor(baseUrl = API_BASE_URL) {
    this.baseUrl = normalizeBaseUrl(baseUrl);
  }

  private async tryRefresh(): Promise<string | null> {
    if (this.refreshPromise) return this.refreshPromise;
    this.refreshPromise = (async () => {
      try {
        const rt = readRefreshToken();
        if (!rt) return null;
        const res = await fetch(this.absoluteUrl("/account/refresh"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: rt }),
        });
        if (!res.ok) return null;
        const data = await res.json().catch(() => null);
        if (!data?.token) return null;
        updateStoredToken(data.token, data.account_id, data.username);
        return data.token as string;
      } catch {
        return null;
      } finally {
        this.refreshPromise = null;
      }
    })();
    return this.refreshPromise;
  }

  absoluteUrl(path: string) {
    if (/^https?:\/\//i.test(path)) return path;
    return `${this.baseUrl}${path.startsWith("/") ? path : `/${path}`}`;
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const method = options.method || "POST";
    const headers: Record<string, string> = {};
    const token = readToken();

    if (options.auth !== false && token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const init: RequestInit = {
      method,
      headers,
      signal: options.signal,
    };

    if (method !== "GET") {
      headers["Content-Type"] = "application/json";
      init.body = JSON.stringify(options.body ?? {});
    }

    const response = await fetch(this.absoluteUrl(path), init);
    const payload = await response.json().catch(() => ({}));

    if (response.status === 401) {
      // refresh 接口本身 401 或已是重试请求，直接踢出
      if (!options._retry && !path.includes("/account/refresh") && !path.includes("/account/logout")) {
        const newToken = await this.tryRefresh();
        if (newToken) {
          return this.request<T>(path, { ...options, _retry: true });
        }
      }
      saveSession(null);
      window.dispatchEvent(new CustomEvent("pulsefeed:auth-expired"));
    }

    if (!response.ok) {
      const message = typeof payload === "object" && payload && "error" in payload
        ? String((payload as { error: unknown }).error)
        : `Request failed with ${response.status}`;
      throw new ApiError(message, response.status, payload);
    }

    return payload as T;
  }

  async upload<T>(path: string, formData: FormData): Promise<T> {
    const headers: Record<string, string> = {};
    const token = readToken();
    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await fetch(this.absoluteUrl(path), {
      method: "POST",
      headers,
      body: formData,
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      const message = typeof payload === "object" && payload && "error" in payload
        ? String((payload as { error: unknown }).error)
        : `Upload failed with ${response.status}`;
      throw new ApiError(message, response.status, payload);
    }
    return payload as T;
  }
}

export const apiClient = new ApiClient();
