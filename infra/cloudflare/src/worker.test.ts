import { describe, expect, it, vi } from "vitest";
import { bbox, buildOverpassQuery, escapeOverpass, keywordRegex, parseOverpass, runPlacesIds, type Job } from "./providers";
import { isApiPath, withClientIp } from "./routing";
import { runDiscoveryCycle, type ApiCall } from "./runner";

const job = (over: Partial<Job> = {}): Job => ({
  id: "11111111-1111-1111-1111-111111111111",
  job_key: "j1",
  provider: "osm_overpass",
  keyword: "vitrified tiles",
  search_query: "vitrified tiles manufacturer in Morbi, Gujarat",
  osm_name_regex: "vitrified|granito",
  latitude: 22.8173,
  longitude: 70.8377,
  radius_km: 15,
  ...over,
});

describe("overpass", () => {
  it("builds a bounded, business-filtered query", () => {
    const q = buildOverpassQuery(job());
    expect(q).toContain('nwr["man_made"="works"]["name"~"vitrified|granito",i];');
    expect(q).toContain("out tags center 200;");
    expect(q.startsWith("[out:json][timeout:90][bbox:22.6826,70.6915,22.9520,70.9839];")).toBe(true);
  });

  it("escapes quotes and caps the radius", () => {
    expect(escapeOverpass('a"b\\c')).toBe('a\\"b\\\\c');
    const q = buildOverpassQuery(job({ osm_name_regex: 'x"];out;', radius_km: 500 }));
    expect(q).toContain('x\\"];out;');
    expect(q).toContain(`[bbox:${bbox(22.8173, 70.8377, 50)}]`);
    expect(bbox(22.8173, 70.8377, 500)).toBe(bbox(22.8173, 70.8377, 50));
  });

  it("falls back to keyword words", () => {
    expect(keywordRegex("CNC machining")).toBe("machining");
    expect(keywordRegex("AAC blocks")).toBe("blocks");
  });

  it("treats Overpass runtime-error remarks as failures, not empty results", () => {
    expect(() => parseOverpass({ elements: [], remark: 'runtime error: Query timed out in "query" at line 1 after 66 seconds.' })).toThrow(
      /timed out/,
    );
    expect(parseOverpass({ elements: [], remark: "some informational note" })).toEqual([]);
  });

  it("parses named nodes and ways, skipping unnamed and duplicates", () => {
    const out = parseOverpass({
      elements: [
        { type: "node", id: 1, lat: 22.8, lon: 70.8, tags: { name: "A Tiles", man_made: "works" } },
        { type: "way", id: 2, center: { lat: 22.9, lon: 70.9 }, tags: { name: "B Ceramics" } },
        { type: "way", id: 3, tags: { building: "industrial" } },
        { type: "node", id: 1, lat: 22.8, lon: 70.8, tags: { name: "A Tiles" } },
      ],
    });
    expect(out.map((r) => r.provider_ref)).toEqual(["node/1", "way/2"]);
    expect(out[1]).toMatchObject({ name: "B Ceramics", latitude: 22.9, longitude: 70.9 });
  });
});

describe("places ids", () => {
  it("requests only place ids", async () => {
    const fetchFn = vi.fn(async (_url: string | URL | Request, init?: RequestInit) => {
      expect((init?.headers as Record<string, string>)["X-Goog-FieldMask"]).toBe("places.id");
      return new Response(JSON.stringify({ places: [{ id: "abc" }, { id: "def" }] }));
    });
    const out = await runPlacesIds(job({ provider: "google_places_ids" }), "key", fetchFn as unknown as typeof fetch);
    expect(out).toEqual([{ provider_ref: "abc" }, { provider_ref: "def" }]);
  });

  it("fails clearly without a key", async () => {
    await expect(runPlacesIds(job({ provider: "google_places_ids" }), undefined)).rejects.toThrow(/GOOGLE_PLACES_API_KEY/);
  });
});

describe("routing", () => {
  it("sends API paths to the API container", () => {
    expect(isApiPath("/api/v1/roles")).toBe(true);
    expect(isApiPath("/healthz")).toBe(true);
    expect(isApiPath("/apiary")).toBe(false);
    expect(isApiPath("/suppliers/x")).toBe(false);
  });

  it("replaces spoofed forwarding headers with CF-Connecting-IP", () => {
    const req = new Request("https://site.example/api/v1/admin/login", {
      headers: { "X-Forwarded-For": "1.2.3.4", "X-Real-IP": "5.6.7.8", "CF-Connecting-IP": "203.0.113.9" },
    });
    const out = withClientIp(req);
    expect(out.headers.get("X-Forwarded-For")).toBe("203.0.113.9");
    expect(out.headers.get("X-Real-IP")).toBeNull();
    expect(out.headers.get("X-Forwarded-Proto")).toBe("https");
  });
});

describe("runner", () => {
  it("claims, runs, completes and records failures", async () => {
    const calls: { path: string; body: any }[] = [];
    const api: ApiCall = async (path, body) => {
      calls.push({ path, body });
      if (path.endsWith("/claim")) {
        return new Response(JSON.stringify({ jobs: [job(), job({ id: "2", job_key: "j2" })] }));
      }
      return new Response("{}");
    };
    let n = 0;
    const fetchFn = vi.fn(async () => {
      n++;
      if (n === 2) return new Response("busy", { status: 429 });
      return new Response(JSON.stringify({ elements: [{ type: "node", id: 9, lat: 1, lon: 2, tags: { name: "X Works" } }] }));
    });
    const summary = await runDiscoveryCycle(api, {
      workerId: "w",
      jobsPerRun: 4,
      providers: ["osm_overpass"],
      overpassUrl: "https://overpass.test/api/interpreter",
      userAgent: "test",
      fetchFn: fetchFn as unknown as typeof fetch,
      sleep: async () => {},
    });
    expect(summary).toMatchObject({ claimed: 2, completed: 1, failed: 1, results: 1 });
    expect(calls[0]).toMatchObject({ path: "/api/v1/worker/jobs/claim", body: { worker_id: "w", limit: 4 } });
    expect(calls[1]?.path).toBe("/api/v1/worker/jobs/11111111-1111-1111-1111-111111111111/complete");
    expect(calls[1]?.body.results[0]).toMatchObject({ provider_ref: "node/9", name: "X Works" });
    expect(calls[2]).toMatchObject({ path: "/api/v1/worker/jobs/2/fail", body: { error: "overpass returned 429" } });
  });

  it("reports a failed claim without throwing", async () => {
    const api: ApiCall = async () => new Response("nope", { status: 401 });
    const s = await runDiscoveryCycle(api, { workerId: "w", jobsPerRun: 1, providers: [], overpassUrl: "", userAgent: "" });
    expect(s.claimed).toBe(0);
    expect(s.errors[0]).toMatch(/claim failed: 401/);
  });
});
