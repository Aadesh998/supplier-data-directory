import { forwardToApi } from "@/lib/proxy";

// Same-origin API proxy: /api/<path> -> ${API_INTERNAL_URL}/api/<path>.
// Implemented as a route handler (not a next.config rewrite) so that
// API_INTERNAL_URL is read at runtime inside the standalone Docker image.
export const dynamic = "force-dynamic";

type Ctx = { params: Promise<{ path: string[] }> };

async function handle(req: Request, ctx: Ctx): Promise<Response> {
  const { path } = await ctx.params;
  const upstreamPath = `/api/${path.map(encodeURIComponent).join("/")}`;
  return forwardToApi(req, upstreamPath);
}

export const GET = handle;
export const HEAD = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
export const OPTIONS = handle;
