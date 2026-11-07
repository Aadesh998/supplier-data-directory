import { expect, test } from "@playwright/test";
import { adminLogin, hasAdmin } from "./helpers";

function csv(rows: string[][]): Buffer {
  return Buffer.from(rows.map((r) => r.map((c) => `"${c.replace(/"/g, '""')}"`).join(",")).join("\n") + "\n", "utf8");
}

test.describe("CSV import → dedup → commit", () => {
  test.skip(!hasAdmin, "Set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD");

  test("imports a file, detects a duplicate on re-import, merges it and commits", async ({ page }) => {
    const run = Date.now().toString(36);
    const header = ["display_name", "website_url", "city", "state", "categories", "roles"];
    const first = csv([
      header,
      [`E2E Import Rugs ${run} (Synthetic)`, `https://rugs-${run}.example.com`, "Bhadohi", "Uttar Pradesh", "rugs", "manufacturer"],
      [`E2E Import CNC ${run} (Synthetic)`, `https://cnc-${run}.example.com`, "Pune", "Maharashtra", "cnc-machining", "job_work_provider"],
    ]);

    await adminLogin(page);

    // 1. Upload with a new, internally-usable source. Public/commercial stay unchecked.
    await page.goto("/admin/imports");
    await page.getByLabel(/CSV file/).setInputFiles({ name: `e2e-${run}-a.csv`, mimeType: "text/csv", buffer: first });
    await page.getByLabel("New source").check();
    await page.getByLabel(/Provider/).fill(`E2E test provider ${run}`);
    await page.getByLabel(/Licence \/ permission reference/).fill("E2E-LICENSE");
    await page.getByRole("checkbox", { name: /Internal use/ }).check();
    await expect(page.getByRole("checkbox", { name: /Public display/ })).not.toBeChecked();
    await expect(page.getByRole("checkbox", { name: /Commercial\/API distribution/ })).not.toBeChecked();
    await page.getByRole("checkbox", { name: /I confirm the origin/ }).check();
    await page.getByRole("button", { name: "Upload and stage" }).click();
    await expect(page).toHaveURL(/\/admin\/imports\/[0-9a-f-]{36}/);
    await expect(page.getByRole("heading", { name: "Column mapping" })).toBeVisible();

    // 2. Dry run then commit, including clean pending rows.
    await page.getByRole("checkbox", { name: /Also create pending rows/ }).check();
    await page.getByRole("button", { name: "Preview (dry run)" }).click();
    await expect(page.getByText("Dry-run preview (nothing saved)")).toBeVisible();
    page.once("dialog", (d) => d.accept());
    await page.getByRole("button", { name: "Commit import" }).click();
    await expect(page.getByText("Commit result")).toBeVisible();

    // 3. Re-import a near-duplicate of the first row.
    const second = csv([header, [`E2E Import Rugs ${run} (Synthetic)`, `https://rugs-${run}.example.com`, "Bhadohi", "Uttar Pradesh", "rugs", "exporter"]]);
    await page.goto("/admin/imports");
    await page.getByLabel(/CSV file/).setInputFiles({ name: `e2e-${run}-b.csv`, mimeType: "text/csv", buffer: second });
    await page.getByLabel("Existing source").check();
    const sourceSelect = page.getByLabel("Where did this file come from?");
    const option = sourceSelect.locator("option", { hasText: `E2E test provider ${run}` });
    await sourceSelect.selectOption({ value: (await option.getAttribute("value")) ?? "" });
    await page.getByRole("checkbox", { name: /I confirm the origin/ }).check();
    await page.getByRole("button", { name: "Upload and stage" }).click();
    await expect(page).toHaveURL(/\/admin\/imports\/[0-9a-f-]{36}/);

    await page.getByRole("tab", { name: /Possible duplicates/ }).click();
    await page.getByRole("button", { name: "Merge into this" }).first().click();
    await expect(page.getByText(/: Merged\./)).toBeVisible();

    await page.getByRole("button", { name: "Preview (dry run)" }).click();
    await expect(page.getByText("Dry-run preview (nothing saved)")).toBeVisible();
    page.once("dialog", (d) => d.accept());
    await page.getByRole("button", { name: "Commit import" }).click();
    await expect(page.getByText("Commit result")).toBeVisible();

    // 4. Duplicate scan runs.
    await page.goto("/admin/duplicates");
    await page.getByRole("button", { name: "Scan for duplicates" }).click();
    await expect(page.getByText(/Scan complete/)).toBeVisible();
  });
});
