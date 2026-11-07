import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { categoryTree, jsonResponse, roles, states } from "@/test/fixtures";
import { RegistrationForm } from "./RegistrationForm";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push, replace: vi.fn(), back: vi.fn(), prefetch: vi.fn(), refresh: vi.fn() }),
  usePathname: () => "/register-supplier",
  useSearchParams: () => new URLSearchParams(),
}));

const options = { roles, categoryTree, states };

function setup() {
  const user = userEvent.setup();
  render(<RegistrationForm initialOptions={options} />);
  return user;
}

async function fillToConsentStep(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText(/Business name/), "Shree Namkeen");
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { name: "Roles and categories" });
  await user.click(screen.getByRole("checkbox", { name: /Manufacturer/ }));
  await user.click(screen.getByRole("checkbox", { name: /Namkeen/ }));
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { name: "Products" });
  await user.click(screen.getByRole("button", { name: "Add a product" }));
  await user.type(screen.getByLabelText(/Product name/), "Bhujia");
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { name: "Facilities" });
  await user.type(screen.getByLabelText(/City or town/), "Rajkot");
  await user.selectOptions(screen.getByLabelText(/State \/ union territory/), "GJ");
  await user.type(screen.getByLabelText(/PIN code/), "360002");
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { name: "Contacts" });
  await user.type(screen.getByLabelText(/Your name/), "Asha Patel");
  await user.type(screen.getByLabelText(/Your email/), "asha@example.in");
  await user.click(screen.getByRole("button", { name: "Continue" }));

  await screen.findByRole("heading", { name: "Permissions and review" });
}

describe("RegistrationForm", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    push.mockReset();
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("blocks the first step until the business name is valid", async () => {
    const user = setup();
    await user.click(screen.getByRole("button", { name: "Continue" }));
    expect(await screen.findByText(/Enter the business name/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Business identity" })).toBeInTheDocument();
    await user.type(screen.getByLabelText(/Website/), "not a url");
    await user.type(screen.getByLabelText(/Business name/), "Shree Namkeen");
    await user.click(screen.getByRole("button", { name: "Continue" }));
    expect(await screen.findByText(/Enter a full web address/)).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Business identity" })).toBeInTheDocument();
  });

  it("requires at least one role and one category", async () => {
    const user = setup();
    await user.type(screen.getByLabelText(/Business name/), "Shree Namkeen");
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await screen.findByRole("heading", { name: "Roles and categories" });
    await user.click(screen.getByRole("button", { name: "Continue" }));
    expect(await screen.findByText("Choose at least one business role")).toBeInTheDocument();
    expect(screen.getByText("Choose at least one category")).toBeInTheDocument();
  });

  it("renders a hidden honeypot field", () => {
    setup();
    const honeypot = document.getElementById("website_confirm") as HTMLInputElement;
    expect(honeypot).not.toBeNull();
    expect(honeypot.tabIndex).toBe(-1);
    expect(honeypot.getAttribute("autocomplete")).toBe("off");
    expect(honeypot.closest("[aria-hidden='true']")).not.toBeNull();
  });

  it("shows three separate, unchecked permission checkboxes and requires the privacy acknowledgement", async () => {
    const user = setup();
    await fillToConsentStep(user);
    const privacy = screen.getByRole("checkbox", { name: /I have read the privacy notice/ });
    const listing = screen.getByRole("checkbox", { name: /Publish a public profile/ });
    const commercial = screen.getByRole("checkbox", { name: /future commercial\/API distribution/ });
    expect(privacy).not.toBeChecked();
    expect(listing).not.toBeChecked();
    expect(commercial).not.toBeChecked();
    expect(screen.getByText(/version 2026-11-v1/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Submit for review" }));
    expect(await screen.findByText(/Please confirm you have read the privacy notice/)).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows a review summary and submits the contract payload with an Idempotency-Key", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ id: "sub-123", status: "submitted" }, 202));
    const user = setup();
    await fillToConsentStep(user);

    // Review summary
    expect(screen.getByText("Shree Namkeen")).toBeInTheDocument();
    expect(screen.getByText(/Food and beverages › Namkeen/)).toBeInTheDocument();
    expect(screen.getByText(/Rajkot, Gujarat, 360002/)).toBeInTheDocument();

    await user.click(screen.getByRole("checkbox", { name: /I have read the privacy notice/ }));
    await user.click(screen.getByRole("checkbox", { name: /Publish a public profile/ }));
    await user.click(screen.getByRole("button", { name: "Submit for review" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/v1/supplier-submissions");
    expect(init.method).toBe("POST");
    const headers = init.headers as Record<string, string>;
    expect(headers["Content-Type"]).toBe("application/json");
    expect(headers["Idempotency-Key"]).toMatch(/.{8,}/);

    const body = JSON.parse(String(init.body));
    expect(body).toMatchObject({
      company: { display_name: "Shree Namkeen" },
      roles: ["manufacturer"],
      categories: ["namkeen"],
      products: [{ name: "Bhujia" }],
      facilities: [{ facility_type: "factory", city: "Rajkot", state_code: "GJ", postal_code: "360002" }],
      submitter: { name: "Asha Patel", email: "asha@example.in" },
      consents: { privacy_notice_acknowledged: true, public_listing: true, commercial_api: false },
      privacy_notice_version: "2026-11-v1",
      website_confirm: "",
    });
    expect(typeof body.form_started_at).toBe("number");
    expect(body.form_started_at).toBeGreaterThan(0);
    expect(body.business_contacts).toEqual([]);

    await waitFor(() => expect(push).toHaveBeenCalledWith("/register-supplier/submitted?ref=sub-123"));
  });

  it("surfaces server validation errors inline on the right step", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(
        { error: { code: "validation_failed", message: "Invalid", request_id: "r1", details: { fields: { "facilities.0.city": "is not a known city" } } } },
        422,
      ),
    );
    const user = setup();
    await fillToConsentStep(user);
    await user.click(screen.getByRole("checkbox", { name: /I have read the privacy notice/ }));
    await user.click(screen.getByRole("button", { name: "Submit for review" }));
    expect(await screen.findByRole("heading", { name: "Facilities" })).toBeInTheDocument();
    expect(await screen.findByText("is not a known city")).toBeInTheDocument();
  });

  it("reuses the same Idempotency-Key when retrying in the same session", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ error: { code: "server_error", message: "Oops", request_id: "r" } }, 500));
    fetchMock.mockResolvedValueOnce(jsonResponse({ id: "sub-9", status: "submitted" }, 202));
    const user = setup();
    await fillToConsentStep(user);
    await user.click(screen.getByRole("checkbox", { name: /I have read the privacy notice/ }));
    await user.click(screen.getByRole("button", { name: "Submit for review" }));
    expect(await screen.findByText(/Oops/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Submit for review" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const k1 = (fetchMock.mock.calls[0]![1] as RequestInit).headers as Record<string, string>;
    const k2 = (fetchMock.mock.calls[1]![1] as RequestInit).headers as Record<string, string>;
    expect(k1["Idempotency-Key"]).toBe(k2["Idempotency-Key"]);
  });
});
