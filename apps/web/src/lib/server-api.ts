import "server-only";

import { apiInternalUrl, internalApiHeaders } from "./env";
import type {
  CategoryDetail,
  CategoryNode,
  DataList,
  Paginated,
  PublicCompany,
  PublicCompanySummary,
  Role,
  SitemapData,
  State,
} from "./types";

// All public data is fetched with `cache: "no-store"` so that unpublished or
// removed profiles disappear immediately and nothing private is ever baked into
// a static build. Pages using these helpers render dynamically.

export type ApiResult<T> = { ok: true; data: T } | { ok: false; status: number };

const TIMEOUT_MS = 8000;

async function apiGet<T>(path: string): Promise<ApiResult<T>> {
  try {
    const res = await fetch(`${apiInternalUrl()}${path}`, {
      cache: "no-store",
      headers: { Accept: "application/json", ...internalApiHeaders() },
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
    if (!res.ok) return { ok: false, status: res.status };
    return { ok: true, data: (await res.json()) as T };
  } catch {
    // API unreachable (e.g. during `next build` or an outage).
    return { ok: false, status: 0 };
  }
}

export function getCategoryTree() {
  return apiGet<DataList<CategoryNode>>("/api/v1/categories/tree");
}

export function getCategory(slug: string) {
  return apiGet<CategoryDetail>(`/api/v1/categories/${encodeURIComponent(slug)}`);
}

export function getStates() {
  return apiGet<DataList<State>>("/api/v1/states");
}

export function getRoles() {
  return apiGet<DataList<Role>>("/api/v1/roles");
}

export function getSitemapData() {
  return apiGet<SitemapData>("/api/v1/public/sitemap");
}

export type CompanySearchParams = {
  q?: string;
  category?: string;
  state?: string;
  city?: string;
  role?: string;
  page?: number;
  limit?: number;
};

export function searchCompanies(params: CompanySearchParams) {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && String(value).trim() !== "") qs.set(key, String(value).trim());
  }
  const query = qs.toString();
  return apiGet<Paginated<PublicCompanySummary>>(`/api/v1/public/companies${query ? `?${query}` : ""}`);
}

export type CompanyLookup =
  | { kind: "ok"; company: PublicCompany }
  | { kind: "redirect"; slug: string }
  | { kind: "not_found" }
  | { kind: "gone" }
  | { kind: "error"; status: number };

export async function getPublicCompany(slug: string): Promise<CompanyLookup> {
  try {
    const res = await fetch(`${apiInternalUrl()}/api/v1/public/companies/${encodeURIComponent(slug)}`, {
      cache: "no-store",
      redirect: "manual",
      headers: { Accept: "application/json" },
      signal: AbortSignal.timeout(TIMEOUT_MS),
    });
    if (res.status === 301 || res.status === 308 || res.status === 302) {
      let target: string | undefined;
      try {
        const body = (await res.json()) as { redirect_slug?: string };
        target = body.redirect_slug;
      } catch {
        target = undefined;
      }
      if (!target) {
        const location = res.headers.get("location");
        target = location ? location.split("/").filter(Boolean).pop() : undefined;
      }
      return target ? { kind: "redirect", slug: target } : { kind: "not_found" };
    }
    if (res.status === 404) return { kind: "not_found" };
    if (res.status === 410) return { kind: "gone" };
    if (!res.ok) return { kind: "error", status: res.status };
    return { kind: "ok", company: (await res.json()) as PublicCompany };
  } catch {
    return { kind: "error", status: 0 };
  }
}
