import { expect, test } from "@playwright/test";
import { adminLogin, hasAdmin } from "./helpers";

test("a correction request can be submitted from a profile link and reaches the admin queue", async ({ page }) => {
  const ref = `e2e-company-${Date.now().toString(36)}`;
  await page.goto(`/request-correction?company=${ref}`);
  await expect(page.getByRole("heading", { level: 1, name: "Request a correction or removal" })).toBeVisible();
  await expect(page.getByLabel(/Which business or profile/)).toHaveValue(ref);
  await expect(page.getByLabel(/What would you like us to do/)).toHaveValue("correction");

  await page.getByLabel(/Your name/).fill("E2E Requester");
  await page.getByLabel(/Your email/).fill("e2e-requester@example.com");
  await page.getByLabel(/Details/).fill("Please update our city. (E2E test)");
  await page.getByRole("button", { name: "Send request" }).click();
  await expect(page.getByRole("heading", { name: "Your request has been received" })).toBeVisible();

  if (!hasAdmin) return;
  await adminLogin(page);
  await page.goto("/admin/privacy-requests");
  await expect(page.getByText(`Business: ${ref}`)).toBeVisible();
});
