import { defineCloudflareConfig } from "@opennextjs/cloudflare";

// Every public page fetches with `cache: "no-store"`, so no incremental cache is configured.
export default defineCloudflareConfig();
