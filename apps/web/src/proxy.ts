import { NextResponse, type NextRequest } from "next/server";

// Next.js 16 "proxy" (formerly middleware).
//
// Purpose: give removed supplier profiles a real HTTP 410 status. App Router
// pages cannot set a 410 status themselves, so for /suppliers/<slug> we ask
// the API for the profile status first and, if it answers 410, respond here
// with a small self-contained "profile removed" page. All other statuses fall
// through to the normal page render (which also handles 410 as a fallback,
// rendering the same message with noindex).

function internalHeaders(): Record<string, string> {
  const token = (process.env as Record<string, string | undefined>)["INTERNAL_API_TOKEN"];
  return token ? { "X-Internal-Token": token } : {};
}

function apiBase(): string {
  const env: Record<string, string | undefined> = process.env;
  return (env["API_INTERNAL_URL"] || "http://localhost:8080").replace(/\/+$/, "");
}

const REMOVED_HTML = `<!doctype html>
<html lang="en-IN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Profile removed | Peoplvance</title>
<style>
body{margin:0;font-family:system-ui,-apple-system,"Segoe UI",Roboto,sans-serif;background:#fff;color:#09090b;line-height:1.6}
main{max-width:40rem;margin:0 auto;padding:4rem 1rem}
h1{font-size:1.875rem;margin:0 0 1rem}
a{color:#1d4ed8}
.btn{display:inline-block;margin-top:1rem;padding:.5rem 1rem;border:1px solid #e4e4e7;border-radius:.25rem;background:#fff;text-decoration:none;color:#09090b}
</style>
</head>
<body>
<main>
<p><a href="/">Peoplvance</a></p>
<h1>This profile has been removed</h1>
<p>The business profile that used to be at this address is no longer published. This can happen when a business withdraws permission, asks for removal, or the listing no longer meets our publication rules.</p>
<p>If you represent the business and believe this is a mistake, please <a href="/request-correction">contact us through the correction form</a>.</p>
<a class="btn" href="/search">Search other suppliers</a>
</main>
</body>
</html>`;

export async function proxy(request: NextRequest) {
  const match = /^\/suppliers\/([^/]+)\/?$/.exec(request.nextUrl.pathname);
  if (!match || request.method !== "GET") return NextResponse.next();
  // Client-side (RSC) navigations render the page's own fallback instead.
  if (request.headers.has("rsc") || request.headers.has("next-router-prefetch")) return NextResponse.next();
  const slug = match[1] ?? "";
  try {
    const res = await fetch(`${apiBase()}/api/v1/public/companies/${slug}`, {
      method: "GET",
      redirect: "manual",
      cache: "no-store",
      headers: { Accept: "application/json", ...internalHeaders() },
      signal: AbortSignal.timeout(3000),
    });
    if (res.status === 410) {
      return new NextResponse(REMOVED_HTML, {
        status: 410,
        headers: { "content-type": "text/html; charset=utf-8", "x-robots-tag": "noindex", "cache-control": "no-store" },
      });
    }
  } catch {
    // API unreachable: let the page render its own "unavailable" state.
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/suppliers/:slug"],
};
