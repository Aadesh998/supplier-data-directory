import { forwardToApi } from "@/lib/proxy";

export const dynamic = "force-dynamic";

export function GET(req: Request) {
  return forwardToApi(req, "/healthz");
}
