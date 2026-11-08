// Pure request helpers (no Cloudflare runtime imports, so they are unit-testable).

/** Paths served by the Go API; everything else goes to Next.js. */
export function isApiPath(pathname: string): boolean {
  return pathname.startsWith("/api/") || pathname === "/healthz" || pathname === "/readyz";
}

/** Copy the request, replacing any client-supplied forwarding headers with Cloudflare's view. */
export function withClientIp(request: Request): Request {
  const headers = new Headers(request.headers);
  const ip = request.headers.get("CF-Connecting-IP");
  headers.delete("X-Forwarded-For");
  headers.delete("X-Real-IP");
  if (ip) headers.set("X-Forwarded-For", ip);
  headers.set("X-Forwarded-Proto", new URL(request.url).protocol.replace(":", ""));
  return new Request(request, { headers });
}
