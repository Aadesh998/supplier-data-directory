import { expect, type Page } from "@playwright/test";

export const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "";
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "";
export const hasAdmin = !!ADMIN_EMAIL && !!ADMIN_PASSWORD;

export function uniqueName(prefix: string): string {
  return `${prefix} ${Date.now().toString(36)} (Synthetic)`;
}

export async function adminLogin(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(ADMIN_EMAIL);
  await page.getByLabel("Password").fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(/\/admin/);
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
}

type SubmitOptions = { name: string; publicListing: boolean; commercial?: boolean };

/** Walk through the public registration form and submit it. */
export async function submitRegistration(page: Page, opts: SubmitOptions) {
  await page.goto("/register-supplier");
  await page.getByLabel(/Business name/).fill(opts.name);
  await page.getByLabel(/What does the business make/).fill("End-to-end test record. Not a real business.");
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Roles and categories" })).toBeVisible();
  await page.locator("fieldset").first().getByRole("checkbox").first().check();
  const categorySearch = page.getByPlaceholder(/Search categories/);
  await categorySearch.fill("namkeen");
  const categoryBoxes = page.locator("fieldset").nth(1).getByRole("checkbox");
  if ((await categoryBoxes.count()) === 0) await categorySearch.fill("");
  await page.locator("fieldset").nth(1).getByRole("checkbox").first().check();
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Products" })).toBeVisible();
  await page.getByRole("button", { name: "Add a product" }).click();
  await page.getByLabel(/Product name/).fill("Test bhujia");
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Facilities" })).toBeVisible();
  await page.getByLabel(/City or town/).fill("Rajkot");
  await page.getByLabel(/State \/ union territory/).selectOption("GJ");
  await page.getByLabel(/PIN code/).fill("360002");
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Contacts" })).toBeVisible();
  await page.getByLabel(/Business email/).fill("sales@example.com");
  await page.getByLabel(/Your name/).fill("E2E Tester");
  // Unique per run: the API caps registrations at 3 per email per day.
  await page.getByLabel(/Your email/).fill(`e2e-tester+${Date.now().toString(36)}@example.com`);
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByRole("heading", { name: "Permissions and review" })).toBeVisible();
  await page.getByRole("checkbox", { name: /I have read the privacy notice/ }).check();
  if (opts.publicListing) await page.getByRole("checkbox", { name: /Publish a public profile/ }).check();
  if (opts.commercial) await page.getByRole("checkbox", { name: /future commercial\/API distribution/ }).check();

  // The API rejects forms completed implausibly fast (anti-spam timing).
  await page.waitForTimeout(Number(process.env.E2E_FORM_DELAY_MS ?? 6000));
  await page.getByRole("button", { name: "Submit for review" }).click();
  await expect(page).toHaveURL(/\/register-supplier\/submitted/);
  await expect(page.getByRole("heading", { name: /submission has been received/ })).toBeVisible();
}

/** Open the submission with this business name from the admin queue and approve it. Returns the company ID. */
export async function approveSubmission(page: Page, name: string): Promise<string> {
  await page.goto("/admin/submissions");
  await page.getByRole("link", { name }).click();
  await expect(page.getByRole("heading", { name })).toBeVisible();
  await page.getByRole("button", { name: "Approve as private" }).click();
  const link = page.getByRole("link", { name: "Open company record" });
  await expect(link).toBeVisible();
  const href = (await link.getAttribute("href")) ?? "";
  const id = href.split("/").pop() ?? "";
  expect(id).not.toBe("");
  return id;
}

/** Read the slug shown in the company page header. */
export async function companySlug(page: Page): Promise<string> {
  return ((await page.getByTestId("company-slug").textContent()) ?? "").trim();
}
