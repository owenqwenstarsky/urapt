import { ApiError, NETWORK_ERROR, toApiError } from "./errors";

const DEFAULT_BASE = "/api/v1";

export function getApiBase(): string {
  const fromEnv = import.meta.env.VITE_API_BASE_URL as string | undefined;
  return (fromEnv && fromEnv.trim()) || DEFAULT_BASE;
}

type TokenGetter = () => string | null;
type UnauthorizedHandler = () => void;

let tokenGetter: TokenGetter = () => null;
let onUnauthorized: UnauthorizedHandler = () => {};

/** Wire the auth store's token into the client (called once at boot). */
export function configureAuth(getter: TokenGetter, handler: UnauthorizedHandler) {
  tokenGetter = getter;
  onUnauthorized = handler;
}

function authHeaders(): Record<string, string> {
  const token = tokenGetter();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

function buildUrl(path: string, params?: Record<string, unknown>): string {
  const base = getApiBase().replace(/\/$/, "");
  const url = `${base}${path.startsWith("/") ? path : `/${path}`}`;
  if (!params) return url;
  const search = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === "") continue;
    search.append(k, String(v));
  }
  const qs = search.toString();
  return qs ? `${url}?${qs}` : url;
}

async function handle<T>(res: Response): Promise<T> {
  if (res.status === 204) return undefined as T;
  if (res.status === 401) {
    // Defer to avoid throwing during render of a reactively-cleared store.
    try {
      onUnauthorized();
    } catch {
      // ignore handler errors
    }
  }
  if (!res.ok) throw await toApiError(res);
  if (res.status === 204 || res.status === 205) return undefined as T;
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

export const http = {
  async get<T>(path: string, params?: Record<string, unknown>): Promise<T> {
    let res: Response;
    try {
      res = await fetch(buildUrl(path, params), {
        headers: { Accept: "application/json", ...authHeaders() },
      });
    } catch {
      throw NETWORK_ERROR;
    }
    return handle<T>(res);
  },

  async sendJson<T>(
    method: "POST" | "PATCH" | "PUT" | "DELETE",
    path: string,
    body?: unknown,
  ): Promise<T> {
    let res: Response;
    try {
      res = await fetch(buildUrl(path), {
        method,
        headers: {
          Accept: "application/json",
          ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
          ...authHeaders(),
        },
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    } catch {
      throw NETWORK_ERROR;
    }
    return handle<T>(res);
  },

  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.sendJson<T>("POST", path, body);
  },

  async patch<T>(path: string, body?: unknown): Promise<T> {
    return this.sendJson<T>("PATCH", path, body);
  },

  async put<T>(path: string, body?: unknown): Promise<T> {
    return this.sendJson<T>("PUT", path, body);
  },

  async del<T>(path: string): Promise<T> {
    return this.sendJson<T>("DELETE", path);
  },

  /** Multipart upload (file + optional form fields). Returns parsed JSON. */
  async postForm<T>(
    path: string,
    fields: Record<string, string>,
    file: File,
    fileField = "file",
  ): Promise<T> {
    const form = new FormData();
    for (const [k, v] of Object.entries(fields)) form.append(k, v);
    form.append(fileField, file);
    let res: Response;
    try {
      res = await fetch(buildUrl(path), {
        method: "POST",
        headers: { Accept: "application/json", ...authHeaders() },
        body: form,
      });
    } catch {
      throw NETWORK_ERROR;
    }
    return handle<T>(res);
  },

  /** Fetch a non-JSON resource (text, blob) with auth headers attached. */
  async raw(path: string, init?: RequestInit): Promise<Response> {
    let res: Response;
    try {
      res = await fetch(buildUrl(path), {
        ...init,
        headers: { ...authHeaders(), ...(init?.headers ?? {}) },
      });
    } catch {
      throw NETWORK_ERROR;
    }
    if (!res.ok) throw await toApiError(res);
    return res;
  },
};

export { ApiError };
