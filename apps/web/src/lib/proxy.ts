import "server-only";

import { apiInternalUrl } from "./env";

// Request headers forwarded to the Go API. Everything else (host, connection,
// accept-encoding, ...) is dropped so fetch can manage transport itself.
const FORWARD_REQUEST_HEADERS = [
  "accept",
  "accept-language",
  "content-type",
  "cookie",
  "idempotency-key",
  "user-agent",
  "x-csrf-token",
  "x-request-id",
];

// Response headers that must not be copied verbatim: fetch has already decoded
// the body, and hop-by-hop headers are connection specific.
const DROP_RESPONSE_HEADERS = new Set([
  "connection",
  "content-encoding",
  "content-length",
  "keep-alive",
  "transfer-encoding",
  "set-cookie",
]);

function clientIp(req: Request): string | undefined {
  // On Cloudflare Workers the edge sets CF-Connecting-IP to the visitor's address; clients can't override it.
  const cf = req.headers.get("cf-connecting-ip");
  if (cf) return cf;
  const xff = req.headers.get("x-forwarded-for");
  if (xff) return xff;
  return req.headers.get("x-real-ip") ?? undefined;
}

/**
 * Forward a request to the API at runtime. Method, relevant headers and body
 * are passed through; status, Location and Set-Cookie come back untouched.
 */
export async function forwardToApi(req: Request, upstreamPath: string): Promise<Response> {
  const incoming = new URL(req.url);
  const target = `${apiInternalUrl()}${upstreamPath}${incoming.search}`;

  const headers = new Headers();
  for (const name of FORWARD_REQUEST_HEADERS) {
    const value = req.headers.get(name);
    if (value !== null) headers.set(name, value);
  }
  const ip = clientIp(req);
  if (ip) headers.set("x-forwarded-for", ip);
  headers.set("x-forwarded-host", incoming.host);
  headers.set("x-forwarded-proto", req.headers.get("x-forwarded-proto") ?? incoming.protocol.replace(":", ""));

  const method = req.method.toUpperCase();
  const hasBody = method !== "GET" && method !== "HEAD";

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method,
      headers,
      body: hasBody ? await req.arrayBuffer() : undefined,
      redirect: "manual",
      cache: "no-store",
      signal: AbortSignal.timeout(60_000),
    });
  } catch {
    return Response.json(
      { error: { code: "upstream_unavailable", message: "The directory API is not reachable right now.", request_id: "" } },
      { status: 502 },
    );
  }

  const outHeaders = new Headers();
  upstream.headers.forEach((value, key) => {
    if (!DROP_RESPONSE_HEADERS.has(key.toLowerCase())) outHeaders.set(key, value);
  });
  for (const cookie of upstream.headers.getSetCookie()) outHeaders.append("set-cookie", cookie);

  const nullBody = method === "HEAD" || upstream.status === 204 || upstream.status === 304;
  return new Response(nullBody ? null : upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: outHeaders,
  });
}
