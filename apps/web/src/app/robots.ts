import type { MetadataRoute } from "next";
import { absoluteUrl, indexingAllowed } from "@/lib/env";

export const dynamic = "force-dynamic";

export default function robots(): MetadataRoute.Robots {
  if (!indexingAllowed()) {
    // Staging / default: keep everything out of search engines.
    return { rules: [{ userAgent: "*", disallow: "/" }] };
  }
  return {
    rules: [{ userAgent: "*", allow: "/", disallow: ["/admin", "/login", "/api/", "/search"] }],
    sitemap: absoluteUrl("/sitemap.xml"),
  };
}
