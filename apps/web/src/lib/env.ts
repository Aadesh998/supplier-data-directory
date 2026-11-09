import "server-only";

// Read environment variables at *runtime*. Accessing process.env through a
// computed key prevents Next.js from inlining NEXT_PUBLIC_* values at build
// time, so the same Docker image can be configured per environment.
function readEnv(name: string): string | undefined {
  const env: Record<string, string | undefined> = process.env;
  const value = env[name];
  return value === undefined || value === "" ? undefined : value;
}

export function apiInternalUrl(): string {
  return (readEnv("API_INTERNAL_URL") ?? "http://localhost:8080").replace(/\/+$/, "");
}

/** Headers for the web server's own server-side calls to the API (never sent from the browser). */
export function internalApiHeaders(): Record<string, string> {
  const token = readEnv("INTERNAL_API_TOKEN");
  return token ? { "X-Internal-Token": token } : {};
}

export function siteUrl(): string {
  return (readEnv("NEXT_PUBLIC_SITE_URL") ?? "http://localhost:3000").replace(/\/+$/, "");
}

export function indexingAllowed(): boolean {
  return readEnv("NEXT_PUBLIC_ALLOW_INDEXING") === "true";
}

export function privacyEmail(): string {
  return readEnv("NEXT_PUBLIC_PRIVACY_EMAIL") ?? "privacy@example.com";
}

export function absoluteUrl(path: string): string {
  return `${siteUrl()}${path.startsWith("/") ? path : `/${path}`}`;
}

/** Inbox for buyer enquiries and full-profile requests (falls back to the privacy inbox). */
export function contactEmail(): string {
  return readEnv("NEXT_PUBLIC_CONTACT_EMAIL") ?? privacyEmail();
}
