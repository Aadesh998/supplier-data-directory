import "server-only";

import type { Metadata } from "next";
import { absoluteUrl, indexingAllowed } from "./env";
import { SITE_NAME } from "./constants";

type PageMetaInput = {
  title: string;
  description: string;
  path: string;
  /** Force noindex regardless of environment. */
  noindex?: boolean;
  ogType?: "website" | "profile";
};

/** Standard metadata: canonical URL, Open Graph basics and robots rules. */
export function pageMetadata({ title, description, path, noindex, ogType = "website" }: PageMetaInput): Metadata {
  const url = absoluteUrl(path);
  const index = indexingAllowed() && !noindex;
  return {
    title,
    description,
    alternates: { canonical: url },
    openGraph: { title, description, url, siteName: SITE_NAME, type: ogType === "profile" ? "profile" : "website", locale: "en_IN" },
    twitter: { card: "summary_large_image", title, description },
    // Thin or synthetic pages: keep out of the index but let crawlers follow links (production only).
    robots: index ? { index: true, follow: true } : { index: false, follow: indexingAllowed() },
  };
}

export function noindexMetadata(title: string, description = ""): Metadata {
  return { title, description, robots: { index: false, follow: false } };
}

export type Crumb = { name: string; path: string };

export function breadcrumbJsonLd(crumbs: Crumb[]) {
  return {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: crumbs.map((c, i) => ({
      "@type": "ListItem",
      position: i + 1,
      name: c.name,
      item: absoluteUrl(c.path),
    })),
  };
}

/** Path with ?page=N for N>1 so paginated pages are canonical to themselves. */
export function pagedPath(path: string, page: number): string {
  return page > 1 ? `${path}?page=${page}` : path;
}
