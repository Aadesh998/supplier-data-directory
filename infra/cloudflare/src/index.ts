// Cloudflare entrypoint: routes traffic to the API and web containers and runs scheduled jobs.

import { Container, getContainer } from "@cloudflare/containers";
import { isApiPath, withClientIp } from "./routing";
import { runDiscoveryCycle, type ApiCall } from "./runner";

export interface Env {
  API: DurableObjectNamespace<ApiContainer>;
  WEB: DurableObjectNamespace<WebContainer>;
  // vars (wrangler.jsonc)
  PUBLIC_SITE_URL: string;
  NEXT_PUBLIC_ALLOW_INDEXING: string;
  PRIVACY_EMAIL: string;
  CONTACT_EMAIL?: string;
  MAIL_FROM: string;
  PRIVACY_NOTICE_VERSION: string;
  DISCOVERY_ENABLED: string;
  DISCOVERY_JOBS_PER_RUN: string;
  DISCOVERY_PROVIDERS: string;
  OVERPASS_URL: string;
  // Optional: tables live in this schema of a shared database (e.g. peoplvance_dir on Railway).
  DATABASE_SCHEMA?: string;
  // secrets (wrangler secret put)
  DATABASE_URL: string;
  DATA_ENCRYPTION_KEY: string;
  INTERNAL_API_TOKEN: string;
  WORKER_TOKEN: string;
  SMTP_HOST?: string;
  SMTP_PORT?: string;
  SMTP_USERNAME?: string;
  SMTP_PASSWORD?: string;
  ADMIN_NOTIFY_EMAIL?: string;
  GOOGLE_PLACES_API_KEY?: string;
}

const CONTAINER_NAME = "main"; // single instance: the API's in-memory rate limiter assumes one process

function clean(vars: Record<string, string | undefined>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(vars)) if (v !== undefined && v !== "") out[k] = v;
  return out;
}

export class ApiContainer extends Container<Env> {
  defaultPort = 8080;
  sleepAfter = "15m";

  constructor(ctx: ConstructorParameters<typeof Container>[0], env: Env) {
    super(ctx, env);
    const origin = env.PUBLIC_SITE_URL.replace(/\/+$/, "");
    this.envVars = clean({
      APP_ENV: "production",
      HTTP_ADDR: ":8080",
      MIGRATE_ON_START: "true",
      DATABASE_URL: env.DATABASE_URL,
      DATABASE_SCHEMA: env.DATABASE_SCHEMA,
      DATA_ENCRYPTION_KEY: env.DATA_ENCRYPTION_KEY,
      INTERNAL_API_TOKEN: env.INTERNAL_API_TOKEN,
      WORKER_TOKEN: env.WORKER_TOKEN,
      PUBLIC_SITE_URL: origin,
      CORS_ALLOWED_ORIGINS: origin,
      COOKIE_SECURE: "true",
      // The Worker overwrites X-Forwarded-For with CF-Connecting-IP, so it can be trusted.
      TRUST_PROXY_HEADERS: "true",
      PRIVACY_NOTICE_VERSION: env.PRIVACY_NOTICE_VERSION,
      MAIL_FROM: env.MAIL_FROM,
      SMTP_HOST: env.SMTP_HOST,
      SMTP_PORT: env.SMTP_PORT,
      SMTP_USERNAME: env.SMTP_USERNAME,
      SMTP_PASSWORD: env.SMTP_PASSWORD,
      ADMIN_NOTIFY_EMAIL: env.ADMIN_NOTIFY_EMAIL,
    });
  }
}

export class WebContainer extends Container<Env> {
  defaultPort = 3000;
  sleepAfter = "15m";

  constructor(ctx: ConstructorParameters<typeof Container>[0], env: Env) {
    super(ctx, env);
    const origin = env.PUBLIC_SITE_URL.replace(/\/+$/, "");
    this.envVars = clean({
      // Server-side calls go back through this Worker to the API container, authenticated with
      // INTERNAL_API_TOKEN so they are not rate-limited as a single IP.
      API_INTERNAL_URL: origin,
      INTERNAL_API_TOKEN: env.INTERNAL_API_TOKEN,
      NEXT_PUBLIC_SITE_URL: origin,
      NEXT_PUBLIC_ALLOW_INDEXING: env.NEXT_PUBLIC_ALLOW_INDEXING,
      NEXT_PUBLIC_PRIVACY_EMAIL: env.PRIVACY_EMAIL,
      NEXT_PUBLIC_CONTACT_EMAIL: env.CONTACT_EMAIL ?? env.PRIVACY_EMAIL,
    });
  }
}

function apiCaller(env: Env): ApiCall {
  const api = getContainer(env.API, CONTAINER_NAME);
  return (path, body) =>
    api.fetch(
      new Request(`http://api.internal${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${env.WORKER_TOKEN}` },
        body: JSON.stringify(body),
      }),
    );
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const { pathname } = new URL(request.url);
    const target = isApiPath(pathname) ? env.API : env.WEB;
    return getContainer(target, CONTAINER_NAME).fetch(withClientIp(request));
  },

  async scheduled(controller: ScheduledController, env: Env): Promise<void> {
    if (controller.cron === "17 3 * * *") {
      const res = await apiCaller(env)("/api/v1/worker/maintenance", {});
      console.log(JSON.stringify({ msg: "maintenance", status: res.status }));
      return;
    }
    if (env.DISCOVERY_ENABLED !== "true") return;
    const summary = await runDiscoveryCycle(apiCaller(env), {
      workerId: `cf-cron-${controller.scheduledTime}`,
      jobsPerRun: Math.min(Math.max(Number.parseInt(env.DISCOVERY_JOBS_PER_RUN, 10) || 4, 1), 20),
      providers: env.DISCOVERY_PROVIDERS.split(",").map((p) => p.trim()).filter(Boolean),
      overpassUrl: env.OVERPASS_URL,
      userAgent: `BharatManufacturingDirectory-discovery/1.0 (+${env.PUBLIC_SITE_URL})`,
      googleApiKey: env.GOOGLE_PLACES_API_KEY,
    });
    console.log(JSON.stringify({ msg: "discovery_cycle", ...summary }));
  },
} satisfies ExportedHandler<Env>;
