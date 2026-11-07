import type { MetadataRoute } from "next";
import { absoluteUrl } from "@/lib/env";
import { getSitemapData } from "@/lib/server-api";

export const dynamic = "force-dynamic";

const STATIC_PATHS = ["/", "/industries", "/locations", "/register-supplier", "/about", "/contact", "/privacy", "/terms", "/data-policy"];

function lastModified(value: string | undefined): Date | undefined {
  if (!value) return undefined;
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? undefined : d;
}

/** Built only from GET /api/v1/public/sitemap (indexable routes) plus static public pages. */
export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const entries: MetadataRoute.Sitemap = STATIC_PATHS.map((p) => ({ url: absoluteUrl(p) }));
  const res = await getSitemapData();
  if (!res.ok) return entries;
  const data = res.data;
  for (const c of data.categories ?? []) entries.push({ url: absoluteUrl(`/industries/${c.slug}`), lastModified: lastModified(c.updated_at) });
  for (const s of data.states ?? []) entries.push({ url: absoluteUrl(`/locations/${s.slug}`) });
  for (const c of data.companies ?? []) entries.push({ url: absoluteUrl(`/suppliers/${c.slug}`), lastModified: lastModified(c.updated_at) });
  return entries;
}
