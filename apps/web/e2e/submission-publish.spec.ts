import { expect, test } from "@playwright/test";
import { adminLogin, approveSubmission, companySlug, hasAdmin, submitRegistration, uniqueName } from "./helpers";

test.describe("supplier submission → review → approve private → publish", () => {
  test.skip(!hasAdmin, "Set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD");

  test("an approved, permitted submission becomes a public profile only after publishing", async ({ page, request }) => {
    const name = uniqueName("E2E Publish Namkeen");
    await submitRegistration(page, { name, publicListing: true });

    await adminLogin(page);
    const companyId = await approveSubmission(page, name);

    await page.goto(`/admin/companies/${companyId}`);
    await expect(page.getByRole("heading", { name })).toBeVisible();
    const slug = await companySlug(page);

    // Approved privately: not yet public.
    const before = await request.get(`/suppliers/${slug}`);
    expect(before.status()).toBe(404);

    await expect(page.getByText("Publishing does not imply verification.")).toBeVisible();
    await page.getByRole("button", { name: "Publish" }).click();
    await expect(page.getByRole("button", { name: "Unpublish" })).toBeVisible();

    await page.goto(`/suppliers/${slug}`);
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
    await expect(page.getByText("Synthetic demo record — not a real business").first()).toBeVisible();
    await expect(page.getByRole("link", { name: "Request a correction or removal" }).first()).toHaveAttribute(
      "href",
      `/request-correction?company=${slug}`,
    );
    // Submitter details are never published.
    await expect(page.getByText("e2e-tester@example.com")).toHaveCount(0);
    await expect(page.getByText("E2E Tester")).toHaveCount(0);
  });
});
