import { expect, test } from "@playwright/test";
import { adminLogin, approveSubmission, companySlug, hasAdmin, submitRegistration, uniqueName } from "./helpers";

test.describe("publication is denied without permission", () => {
  test.skip(!hasAdmin, "Set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD");

  test("approved without public-listing permission: publish is blocked and the profile stays absent", async ({ page, request }) => {
    const name = uniqueName("E2E Private Tiles");
    await submitRegistration(page, { name, publicListing: false });

    await adminLogin(page);
    const companyId = await approveSubmission(page, name);
    await page.goto(`/admin/companies/${companyId}`);
    const slug = await companySlug(page);

    await page.getByRole("button", { name: "Publish" }).click();
    await expect(page.getByText("Publishing was blocked by the policy check:")).toBeVisible();

    const res = await request.get(`/suppliers/${slug}`);
    expect(res.status()).toBe(404);

    const sitemap = await (await request.get("/sitemap.xml")).text();
    expect(sitemap).not.toContain(`/suppliers/${slug}`);
  });

  test("a rejected submission never creates a public profile", async ({ page, request }) => {
    const name = uniqueName("E2E Rejected Bricks");
    await submitRegistration(page, { name, publicListing: true });

    await adminLogin(page);
    await page.goto("/admin/submissions");
    await page.getByRole("link", { name }).click();
    await page.getByLabel("Outcome").selectOption("rejected");
    await page.getByLabel(/Reason/).fill("E2E rejection");
    await page.getByRole("button", { name: "Confirm outcome" }).click();
    await expect(page.getByText(/This submission has been reviewed/)).toBeVisible();

    const search = await request.get(`/api/v1/public/companies?q=${encodeURIComponent(name)}`);
    expect(search.ok()).toBeTruthy();
    const body = (await search.json()) as { data: { display_name: string }[] };
    expect(body.data.some((c) => c.display_name === name)).toBe(false);
  });
});
