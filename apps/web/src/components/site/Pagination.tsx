import Link from "next/link";
import type { PageInfo } from "@/lib/types";

/** Link-based pagination (works without JavaScript). */
export function Pagination({ page, basePath, params = {} }: { page: PageInfo; basePath: string; params?: Record<string, string | undefined> }) {
  const totalPages = Math.max(1, Math.ceil(page.total / Math.max(1, page.limit)));
  if (totalPages <= 1 && !page.has_more) return null;

  const href = (p: number) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v) qs.set(k, v);
    if (p > 1) qs.set("page", String(p));
    const s = qs.toString();
    return s ? `${basePath}?${s}` : basePath;
  };

  return (
    <nav aria-label="Pagination" className="mt-8 flex items-center justify-between gap-4 text-sm">
      {page.page > 1 ? (
        <Link href={href(page.page - 1)} className="btn-secondary" rel="prev">
          Previous
        </Link>
      ) : (
        <span />
      )}
      <span className="text-slate-700">
        Page {page.page} of {totalPages} · {page.total} result{page.total === 1 ? "" : "s"}
      </span>
      {page.has_more ? (
        <Link href={href(page.page + 1)} className="btn-secondary" rel="next">
          Next
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}

export function parsePage(value: string | string[] | undefined): number {
  const raw = Array.isArray(value) ? value[0] : value;
  const n = Number.parseInt(raw ?? "1", 10);
  return Number.isFinite(n) && n >= 1 && n <= 10_000 ? n : 1;
}

export function firstParam(value: string | string[] | undefined): string | undefined {
  const v = Array.isArray(value) ? value[0] : value;
  return v && v.trim() ? v.trim().slice(0, 100) : undefined;
}
