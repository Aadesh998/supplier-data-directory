import { toApiError } from "../api-error";

export type QueryValue = string | number | boolean | undefined | null;

export type RequestOptions = {
  body?: unknown;
  query?: Record<string, QueryValue>;
  /** Return the raw Response instead of parsing JSON (e.g. CSV downloads). */
  raw?: boolean;
  signal?: AbortSignal;
};

export type AdminClient = {
  request<T>(method: string, path: string, opts?: RequestOptions): Promise<T>;
  get<T>(path: string, query?: Record<string, QueryValue>): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
  patch<T>(path: string, body?: unknown): Promise<T>;
  del<T>(path: string): Promise<T>;
  download(method: "GET" | "POST", path: string, body?: unknown): Promise<Blob>;
};

export function buildQuery(query?: Record<string, QueryValue>): string {
  if (!query) return "";
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === "") continue;
    qs.set(k, String(v));
  }
  const s = qs.toString();
  return s ? `?${s}` : "";
}

/**
 * Browser client for /api/v1/admin/*. Same-origin, so the HttpOnly bm_session
 * cookie flows automatically. The CSRF token (kept in memory only) is sent on
 * every non-GET request. A 401 triggers `onUnauthorized` (redirect to login).
 */
export function createAdminClient(getCsrf: () => string | null, onUnauthorized: () => void): AdminClient {
  async function request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<T> {
    const upper = method.toUpperCase();
    const headers: Record<string, string> = { Accept: "application/json" };
    let body: BodyInit | undefined;
    if (opts.body instanceof FormData) {
      body = opts.body;
    } else if (opts.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(opts.body);
    }
    if (upper !== "GET" && upper !== "HEAD") {
      const csrf = getCsrf();
      if (csrf) headers["X-CSRF-Token"] = csrf;
    }
    const res = await fetch(`/api/v1${path}${buildQuery(opts.query)}`, {
      method: upper,
      headers,
      body,
      credentials: "same-origin",
      cache: "no-store",
      signal: opts.signal,
    });
    if (res.status === 401) {
      onUnauthorized();
      throw await toApiError(res);
    }
    if (!res.ok) throw await toApiError(res);
    if (opts.raw) return res as unknown as T;
    if (res.status === 204) return undefined as T;
    const text = await res.text();
    return (text ? JSON.parse(text) : undefined) as T;
  }

  return {
    request,
    get: (path, query) => request("GET", path, { query }),
    post: (path, body) => request("POST", path, { body }),
    patch: (path, body) => request("PATCH", path, { body }),
    del: (path) => request("DELETE", path),
    async download(method, path, body) {
      const res = await request<Response>(method, path, { body, raw: true });
      return res.blob();
    },
  };
}

/** Trigger a browser download for a Blob. */
export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** In-memory holder for the session CSRF token. Never persisted to any storage. */
export class SessionTokenStore {
  private token: string | null;
  private redirecting = false;

  constructor(token: string | null) {
    this.token = token;
  }

  get(): string | null {
    return this.token;
  }

  set(token: string | null) {
    this.token = token;
  }

  /** Returns true only the first time it is called, to avoid redirect loops. */
  beginRedirect(): boolean {
    if (this.redirecting) return false;
    this.redirecting = true;
    return true;
  }
}
