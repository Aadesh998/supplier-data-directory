export type SearchFilterValues = {
  q?: string;
  category?: string;
  state?: string;
  city?: string;
  role?: string;
};

export const SEARCH_KEYS = ["q", "category", "state", "city", "role"] as const;

/** Build a /search query string. Empty values are dropped; page resets to 1. */
export function buildSearchQuery(values: SearchFilterValues): string {
  const qs = new URLSearchParams();
  for (const key of SEARCH_KEYS) {
    const v = values[key]?.trim();
    if (v) qs.set(key, v.slice(0, 100));
  }
  return qs.toString();
}
