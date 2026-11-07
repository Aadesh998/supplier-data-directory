import { describe, expect, it } from "vitest";
import { buildSubmissionPayload, defaultRegistrationValues, registrationSchema, stepForServerField, type RegistrationValues } from "./registration";

function filled(): RegistrationValues {
  const v = defaultRegistrationValues();
  v.company.display_name = "  Shree Namkeen  ";
  v.company.website_url = "https://shree.example.in";
  v.company.establishment_year = "1998";
  v.roles = ["manufacturer"];
  v.categories = ["namkeen"];
  v.products = [
    { name: "Bhujia", description: "", unit: "kg" },
    { name: "   ", description: "", unit: "" },
  ];
  v.facilities = [{ facility_type: "factory", address_line1: "", locality: "", city: "Rajkot", district: "", state_code: "GJ", postal_code: "360002" }];
  v.contacts = { public_business_email: "sales@shree.example.in", switchboard: "+91 281 234 5678", sales_department: "+91 98765 43210" };
  v.submitter = { name: "Asha", email: "asha@example.in", designation: "", phone: "" };
  v.consents.privacy_notice_acknowledged = true;
  return v;
}

describe("defaultRegistrationValues", () => {
  it("never pre-checks any permission", () => {
    const v = defaultRegistrationValues();
    expect(v.consents).toEqual({ privacy_notice_acknowledged: false, public_listing: false, commercial_api: false });
    expect(v.website_confirm).toBe("");
  });
});

describe("registrationSchema", () => {
  it("requires the privacy acknowledgement but not the optional permissions", () => {
    const v = filled();
    v.products = [{ name: "Bhujia", description: "", unit: "kg" }];
    expect(registrationSchema.safeParse(v).success).toBe(true);
    v.consents.privacy_notice_acknowledged = false;
    const res = registrationSchema.safeParse(v);
    expect(res.success).toBe(false);
  });

  it("rejects blank product rows", () => {
    expect(registrationSchema.safeParse(filled()).success).toBe(false);
  });

  it("validates PIN codes and category limits", () => {
    const v = filled();
    v.products = [];
    v.facilities[0]!.postal_code = "012345";
    expect(registrationSchema.safeParse(v).success).toBe(false);
    const w = filled();
    w.products = [];
    w.categories = Array.from({ length: 11 }, (_, i) => `c${i}`);
    expect(registrationSchema.safeParse(w).success).toBe(false);
  });
});

describe("buildSubmissionPayload", () => {
  it("builds the contract shape with consents, honeypot, version and timing", () => {
    const payload = buildSubmissionPayload(filled(), { formStartedAt: 1_700_000_000_000 });
    expect(payload).toEqual({
      company: { display_name: "Shree Namkeen", website_url: "https://shree.example.in", establishment_year: 1998 },
      roles: ["manufacturer"],
      categories: ["namkeen"],
      products: [{ name: "Bhujia", unit: "kg" }],
      facilities: [{ facility_type: "factory", city: "Rajkot", state_code: "GJ", postal_code: "360002" }],
      business_contacts: [
        { contact_type: "public_business_email", channel: "email", value: "sales@shree.example.in" },
        { contact_type: "switchboard", channel: "phone", value: "+91 281 234 5678" },
        { contact_type: "sales_department", channel: "phone", value: "+91 98765 43210" },
      ],
      submitter: { name: "Asha", email: "asha@example.in" },
      consents: { privacy_notice_acknowledged: true, public_listing: false, commercial_api: false },
      privacy_notice_version: "2026-11-v1",
      form_started_at: 1_700_000_000_000,
      website_confirm: "",
    });
  });

  it("passes optional permissions through only when explicitly given", () => {
    const v = filled();
    v.consents.public_listing = true;
    const p = buildSubmissionPayload(v, { formStartedAt: 1 });
    expect(p.consents).toEqual({ privacy_notice_acknowledged: true, public_listing: true, commercial_api: false });
  });
});

describe("stepForServerField", () => {
  it("maps dotted server paths to form steps", () => {
    expect(stepForServerField("company.display_name")).toBe(0);
    expect(stepForServerField("categories")).toBe(1);
    expect(stepForServerField("products.2.name")).toBe(2);
    expect(stepForServerField("facilities.0.postal_code")).toBe(3);
    expect(stepForServerField("submitter.email")).toBe(4);
    expect(stepForServerField("consents.privacy_notice_acknowledged")).toBe(5);
  });
});
