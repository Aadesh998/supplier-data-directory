import type { NextConfig } from "next";

const noindexHeaders = [{ key: "X-Robots-Tag", value: "noindex, nofollow" }];

const nextConfig: NextConfig = {
  output: "standalone",
  poweredByHeader: false,
  reactStrictMode: true,
  // The API proxy is implemented as a runtime route handler
  // (src/app/api/[...path]/route.ts and src/app/healthz/route.ts) rather than a
  // next.config rewrite, because rewrites are frozen at build time in standalone
  // output while API_INTERNAL_URL must be configurable at container runtime.
  async headers() {
    return [
      { source: "/admin", headers: noindexHeaders },
      { source: "/admin/:path*", headers: noindexHeaders },
      { source: "/login", headers: noindexHeaders },
      { source: "/search", headers: noindexHeaders },
      { source: "/request-correction", headers: noindexHeaders },
      { source: "/register-supplier/submitted", headers: noindexHeaders },
      {
        source: "/:path*",
        headers: [
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
        ],
      },
    ];
  },
};

export default nextConfig;
