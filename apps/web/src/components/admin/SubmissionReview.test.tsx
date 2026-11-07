import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { jsonResponse } from "@/test/fixtures";
import type { Session, SubmissionDetail } from "@/lib/types";
import { AdminProviders, makeQueryClient } from "./AdminProviders";
import { SubmissionReview } from "./SubmissionReview";

const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace, prefetch: vi.fn() }),
  usePathname: () => "/admin/submissions/abc",
}));

const session: Session = { user: { id: "u1", email: "rev@example.in", role: "reviewer" }, csrf_token: "csrf-xyz" };

const submission: SubmissionDetail = {
  id: "8f9a6c1e-0000-4000-8000-000000000001",
  status: "submitted",
  company_name: "Shree Namkeen",
  submitted_at: "2026-11-05T10:00:00Z",
  reviewed_at: null,
  privacy_notice_version: "2026-11-v1",
  consents: { storage_acknowledged_at: "2026-11-05T10:00:00Z", public_listing_at: "2026-11-05T10:00:00Z", commercial_api_at: null },
  company_id: null,
  review_notes: null,
  submitter_email: "asha@example.in",
  payload: {
    company: { display_name: "Shree Namkeen" },
    roles: ["manufacturer"],
    categories: ["namkeen"],
    facilities: [{ facility_type: "factory", city: "Rajkot", state_code: "GJ" }],
    submitter: { name: "Asha", email: "asha@example.in" },
    consents: { privacy_notice_acknowledged: true, public_listing: true, commercial_api: false },
    privacy_notice_version: "2026-11-v1",
    form_started_at: 1,
  },
};

function renderReview() {
  return render(
    <AdminProviders initialSession={session} queryClient={makeQueryClient()}>
      <SubmissionReview submission={submission} />
    </AdminProviders>,
  );
}

describe("SubmissionReview", () => {
  const fetchMock = vi.fn();
  beforeEach(() => {
    fetchMock.mockReset();
    replace.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });
  afterEach(() => vi.unstubAllGlobals());

  it("shows consents with timestamps and the privacy notice version", () => {
    renderReview();
    expect(screen.getByText("2026-11-v1")).toBeInTheDocument();
    const rows = screen.getAllByRole("row");
    expect(rows.some((r) => /Public listing permission/.test(r.textContent ?? "") && /Given/.test(r.textContent ?? ""))).toBe(true);
    expect(rows.some((r) => /commercial\/API/.test(r.textContent ?? "") && /Not given/.test(r.textContent ?? ""))).toBe(true);
  });

  it("approves with the CSRF header and shows the new company link", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ submission: { ...submission, status: "approved" }, company_id: "c-42" }));
    const user = userEvent.setup();
    renderReview();
    await user.type(screen.getByLabelText(/Notes/), "Looks right");
    await user.click(screen.getByRole("button", { name: "Approve as private" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`/api/v1/admin/submissions/${submission.id}/approve`);
    expect(init.method).toBe("POST");
    expect((init.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-xyz");
    expect(JSON.parse(String(init.body))).toEqual({ notes: "Looks right" });
    expect(await screen.findByRole("link", { name: "Open company record" })).toHaveAttribute("href", "/admin/companies/c-42");
  });

  it("requires a reason and sends the chosen status when rejecting", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ submission: { ...submission, status: "needs_changes" } }));
    const user = userEvent.setup();
    renderReview();
    await user.click(screen.getByRole("button", { name: "Confirm outcome" }));
    expect(await screen.findByText("Enter a reason.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();

    await user.selectOptions(screen.getByLabelText("Outcome"), "needs_changes");
    await user.type(screen.getByLabelText(/Reason/), "Please add a facility address");
    await user.click(screen.getByRole("button", { name: "Confirm outcome" }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe(`/api/v1/admin/submissions/${submission.id}/reject`);
    expect((init.headers as Record<string, string>)["X-CSRF-Token"]).toBe("csrf-xyz");
    expect(JSON.parse(String(init.body))).toEqual({ reason: "Please add a facility address", status: "needs_changes" });
    expect(await screen.findByText(/This submission has been reviewed/)).toBeInTheDocument();
  });

  it("redirects to /login on 401", async () => {
    fetchMock.mockResolvedValue(jsonResponse({ error: { code: "unauthorized", message: "no", request_id: "r" } }, 401));
    const user = userEvent.setup();
    renderReview();
    await user.click(screen.getByRole("button", { name: "Approve as private" }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/login?next=%2Fadmin%2Fsubmissions%2Fabc"));
  });
});
