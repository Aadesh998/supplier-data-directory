import { expect, test } from "@playwright/test";

test.describe("SEO guards", () => {
  test("robots.txt keeps crawlers out of /admin", async ({ request }) => {
    const res = await request.get("/robots.txt");
    expect(res.ok()).toBeTruthy();
    const text = await res.text();
    const disallows = text
      .split("\n")
      .filter((l) => /^disallow:/i.test(l.trim()))
      .map((l) => l.split(":").slice(1).join(":").trim());
    // Either staging mode (Disallow: /) or production rules that list /admin explicitly.
    expect(disallows.some((d) => d === "/" || d === "/admin")).toBe(true);
    if (!disallows.includes("/")) {
      expect(disallows).toEqual(expect.arrayContaining(["/admin", "/login", "/api/", "/search"]));
      expect(text).toMatch(/Sitemap: .*\/sitemap\.xml/);
    }
  });

  for (const path of ["/admin", "/admin/companies", "/login"]) {
    test(`${path} is noindex`, async ({ page, request }) => {
      const res = await request.get(path);
      expect(res.headers()["x-robots-tag"] ?? "").toContain("noindex");
      await page.goto(path);
      const robots = await page.locator('meta[name="robots"]').first().getAttribute("content");
      expect(robots ?? "").toContain("noindex");
    });
  }

  test("search results are noindex", async ({ page }) => {
    await page.goto("/search?q=test");
    const robots = await page.locator('meta[name="robots"]').first().getAttribute("content");
    expect(robots ?? "").toContain("noindex");
  });

  test("sitemap only lists public routes", async ({ request }) => {
    const xml = await (await request.get("/sitemap.xml")).text();
    expect(xml).toContain("<urlset");
    expect(xml).not.toMatch(/\/admin|\/login|\/search|\/api\//);
  });
});
