// Discovery providers used by the cron runner. Both are plain HTTP calls, so they run inside a Worker.
//
// - OpenStreetMap via Overpass: returns named industrial/commercial features. Data © OpenStreetMap
//   contributors (ODbL). Respect the Overpass usage policy: low concurrency, identify yourself.
// - Google Places "IDs only": Text Search with field mask `places.id`. Only place IDs are returned and
//   stored (Google permits storing place IDs indefinitely); no names/addresses are requested.

export type Job = {
  id: string;
  job_key: string;
  provider: "osm_overpass" | "google_places_ids";
  keyword: string;
  search_query: string;
  osm_name_regex: string | null;
  latitude: number;
  longitude: number;
  radius_km: number;
};

export type Result = {
  provider_ref: string;
  name?: string;
  latitude?: number;
  longitude?: number;
  tags?: Record<string, string>;
};

/** Tags that indicate a business/industrial feature; the name regex alone is too noisy. */
const BUSINESS_FILTERS = [
  `["man_made"="works"]`,
  `["industrial"]`,
  `["craft"]`,
  `["building"~"^(industrial|factory|warehouse|manufacture)$"]`,
  `["landuse"="industrial"]`,
  `["office"~"^(company|industrial)$"]`,
  `["shop"~"^(wholesale|trade)$"]`,
];

/** Escape a value for use inside an Overpass QL double-quoted string. */
export function escapeOverpass(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/[\r\n]/g, " ");
}

/** Fallback name regex built from the keyword when the job has none. */
export function keywordRegex(keyword: string): string {
  const words = keyword
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter((w) => w.length >= 4);
  return words.length ? words.join("|") : keyword.toLowerCase();
}

/** Square bounding box (south,west,north,east) around a point. Index lookups on a bbox are far cheaper than around:. */
export function bbox(lat: number, lng: number, radiusKm: number): string {
  const r = Math.min(Math.max(radiusKm, 0.5), 50);
  const dLat = r / 111.32;
  const dLng = r / (111.32 * Math.cos((lat * Math.PI) / 180));
  const f = (n: number) => n.toFixed(4);
  return `${f(lat - dLat)},${f(lng - dLng)},${f(lat + dLat)},${f(lng + dLng)}`;
}

export function buildOverpassQuery(job: Job, limit = 200): string {
  const regex = escapeOverpass(job.osm_name_regex?.trim() || keywordRegex(job.keyword));
  // Tag filter first (indexed), then the name regex; a global bbox instead of around: avoids
  // per-geometry distance checks that time out on busy public instances.
  const statements = BUSINESS_FILTERS.map((f) => `  nwr${f}["name"~"${regex}",i];`).join("\n");
  return `[out:json][timeout:90][bbox:${bbox(job.latitude, job.longitude, job.radius_km)}];\n(\n${statements}\n);\nout tags center ${limit};`;
}

type OverpassElement = {
  type: "node" | "way" | "relation";
  id: number;
  lat?: number;
  lon?: number;
  center?: { lat: number; lon: number };
  tags?: Record<string, string>;
};

export type OverpassBody = { elements?: OverpassElement[]; remark?: string };

/**
 * Overpass reports timeouts and memory errors as HTTP 200 with a `remark` and partial/empty data.
 * Treat those as failures so the job is retried instead of being recorded as "0 results".
 */
export function parseOverpass(body: OverpassBody): Result[] {
  if (body.remark && /runtime error|timed out|out of memory|too many requests/i.test(body.remark)) {
    throw new Error(`overpass: ${body.remark}`);
  }
  const out: Result[] = [];
  const seen = new Set<string>();
  for (const el of body.elements ?? []) {
    const name = el.tags?.name?.trim();
    if (!name) continue;
    const ref = `${el.type}/${el.id}`;
    if (seen.has(ref)) continue;
    seen.add(ref);
    const lat = el.lat ?? el.center?.lat;
    const lon = el.lon ?? el.center?.lon;
    out.push({ provider_ref: ref, name, latitude: lat, longitude: lon, tags: el.tags ?? {} });
  }
  return out;
}

export async function runOverpass(job: Job, overpassUrl: string, userAgent: string, fetchFn: typeof fetch = fetch): Promise<Result[]> {
  const res = await fetchFn(overpassUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded", "User-Agent": userAgent, Accept: "application/json" },
    body: "data=" + encodeURIComponent(buildOverpassQuery(job)),
  });
  if (!res.ok) throw new Error(`overpass returned ${res.status}`);
  return parseOverpass((await res.json()) as OverpassBody);
}

export async function runPlacesIds(job: Job, apiKey: string | undefined, fetchFn: typeof fetch = fetch): Promise<Result[]> {
  if (!apiKey) throw new Error("GOOGLE_PLACES_API_KEY is not set; cannot run google_places_ids jobs");
  const res = await fetchFn("https://places.googleapis.com/v1/places:searchText", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Goog-Api-Key": apiKey, "X-Goog-FieldMask": "places.id" },
    body: JSON.stringify({
      textQuery: job.search_query,
      regionCode: "IN",
      pageSize: 20,
      locationBias: {
        circle: { center: { latitude: job.latitude, longitude: job.longitude }, radius: Math.min(job.radius_km * 1000, 50000) },
      },
    }),
  });
  if (!res.ok) throw new Error(`places returned ${res.status}`);
  const body = (await res.json()) as { places?: { id: string }[] };
  return (body.places ?? []).filter((p) => p.id).map((p) => ({ provider_ref: p.id }));
}
