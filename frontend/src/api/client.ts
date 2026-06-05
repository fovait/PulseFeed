import { readToken, saveSession } from "./storage";

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
};

function normalizeBaseUrl(value: string) {
  return value.replace(/\/+$/, "");
}

export const API_BASE_URL = normalizeBaseUrl(import.meta.env.VITE_API_BASE_URL || "/api");

export class ApiClient {
  baseUrl: string;

  constructor(baseUrl = API_BASE_URL) {
    this.baseUrl = normalizeBaseUrl(baseUrl);
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
