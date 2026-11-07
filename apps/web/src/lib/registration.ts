import { z } from "zod";
import { PRIVACY_NOTICE_VERSION } from "./constants";
import type { FacilityType, SupplierSubmissionRequest } from "./types";

// ---------------------------------------------------------------------------
// Validation schemas — one per step, merged into the full form schema.
// All inputs stay strings in the form; conversion happens in the payload builder.
// ---------------------------------------------------------------------------

const PIN_RE = /^[1-9][0-9]{5}$/;
const PHONE_RE = /^\+?[0-9][0-9 ()-]{5,24}$/;
const URL_RE = /^https?:\/\/[^\s/$.?#][^\s]*\.[^\s]+$/i;

const optionalMax = (max: number, label: string) => z.string().max(max, `${label} must be ${max} characters or fewer`);
const required = (label: string, max: number) =>
  z
    .string()
    .max(max, `${label} must be ${max} characters or fewer`)
    .refine((v) => v.trim().length > 0, `${label} is required`);

const emailOrEmpty = (msg: string) => z.string().refine((v) => v.trim() === "" || z.email().safeParse(v.trim()).success, msg);
const phoneOrEmpty = (msg: string) => z.string().refine((v) => v.trim() === "" || PHONE_RE.test(v.trim()), msg);

const currentYear = new Date().getFullYear();

export const identitySchema = z.object({
  company: z.object({
    display_name: z
      .string()
      .max(200, "Business name must be 200 characters or fewer")
      .refine((v) => v.trim().length >= 2, "Enter the business name (at least 2 characters)"),
    legal_name: optionalMax(200, "Legal name"),
    description: optionalMax(2000, "Description"),
    website_url: z
      .string()
      .max(300, "Website address is too long")
      .refine((v) => v.trim() === "" || URL_RE.test(v.trim()), "Enter a full web address starting with https:// or http://"),
    establishment_year: z
      .string()
      .refine(
        (v) => v.trim() === "" || (/^\d{4}$/.test(v.trim()) && Number(v) >= 1800 && Number(v) <= currentYear),
        `Enter a four-digit year between 1800 and ${currentYear}`,
      ),
  }),
});

export const classificationSchema = z.object({
  roles: z.array(z.string()).min(1, "Choose at least one business role").max(8, "Choose at most 8 roles"),
  categories: z.array(z.string()).min(1, "Choose at least one category").max(10, "Choose at most 10 categories"),
});

export const productSchema = z.object({
  name: required("Product name", 200),
  description: optionalMax(1000, "Product description"),
  unit: optionalMax(50, "Unit"),
});

export const productsSchema = z.object({
  products: z.array(productSchema).max(30, "You can list up to 30 products"),
});

export const facilitySchema = z.object({
  facility_type: z.string().min(1, "Choose the facility type"),
  address_line1: optionalMax(200, "Address"),
  locality: optionalMax(120, "Locality"),
  city: required("City or town", 120),
  district: optionalMax(120, "District"),
  state_code: z.string().regex(/^[A-Z]{2}$/, "Choose a state or union territory"),
  postal_code: z.string().refine((v) => v.trim() === "" || PIN_RE.test(v.trim()), "Enter a valid 6-digit PIN code (e.g. 360002)"),
});

export const facilitiesSchema = z.object({
  facilities: z.array(facilitySchema).min(1, "Add at least one facility").max(10, "You can list up to 10 facilities"),
});

export const contactsSchema = z.object({
  contacts: z.object({
    public_business_email: emailOrEmpty("Enter a valid email address, e.g. sales@yourcompany.in"),
    switchboard: phoneOrEmpty("Enter a valid phone number, e.g. +91 281 234 5678"),
    sales_department: z
      .string()
      .max(200, "Too long")
      .refine(
        (v) => v.trim() === "" || z.email().safeParse(v.trim()).success || PHONE_RE.test(v.trim()),
        "Enter a sales email address or phone number",
      ),
  }),
  submitter: z.object({
    name: required("Your name", 120),
    email: z.string().refine((v) => z.email().safeParse(v.trim()).success, "Enter a valid email address so we can contact you"),
    designation: optionalMax(120, "Designation"),
    phone: phoneOrEmpty("Enter a valid phone number, e.g. +91 98765 43210"),
  }),
});

export const consentsSchema = z.object({
  consents: z.object({
    privacy_notice_acknowledged: z.boolean().refine((v) => v === true, "Please confirm you have read the privacy notice to continue"),
    public_listing: z.boolean(),
    commercial_api: z.boolean(),
  }),
  website_confirm: z.string(),
});

export const registrationSchema = identitySchema
  .extend(classificationSchema.shape)
  .extend(productsSchema.shape)
  .extend(facilitiesSchema.shape)
  .extend(contactsSchema.shape)
  .extend(consentsSchema.shape);

export type RegistrationValues = z.infer<typeof registrationSchema>;

export const emptyFacility = (): RegistrationValues["facilities"][number] => ({
  facility_type: "factory",
  address_line1: "",
  locality: "",
  city: "",
  district: "",
  state_code: "",
  postal_code: "",
});

export const emptyProduct = (): RegistrationValues["products"][number] => ({ name: "", description: "", unit: "" });

export function defaultRegistrationValues(): RegistrationValues {
  return {
    company: { display_name: "", legal_name: "", description: "", website_url: "", establishment_year: "" },
    roles: [],
    categories: [],
    products: [],
    facilities: [emptyFacility()],
    contacts: { public_business_email: "", switchboard: "", sales_department: "" },
    submitter: { name: "", email: "", designation: "", phone: "" },
    // Never pre-checked.
    consents: { privacy_notice_acknowledged: false, public_listing: false, commercial_api: false },
    website_confirm: "",
  };
}

export const STEPS = [
  { id: "identity", title: "Business identity", fields: ["company"] },
  { id: "classification", title: "Roles and categories", fields: ["roles", "categories"] },
  { id: "products", title: "Products", fields: ["products"] },
  { id: "facilities", title: "Facilities", fields: ["facilities"] },
  { id: "contacts", title: "Contacts", fields: ["contacts", "submitter"] },
  { id: "review", title: "Permissions and review", fields: ["consents"] },
] as const;

/** Which step a server-side (dotted) field path belongs to. */
export function stepForServerField(path: string): number {
  const root = path.split(".")[0] ?? "";
  switch (root) {
    case "company":
      return 0;
    case "roles":
    case "categories":
      return 1;
    case "products":
      return 2;
    case "facilities":
      return 3;
    case "business_contacts":
    case "submitter":
      return 4;
    default:
      return 5;
  }
}

/** Map a server field path to a form field path we can attach an error to. */
export function formPathForServerField(path: string): string {
  if (path.startsWith("business_contacts")) return "contacts";
  if (path === "privacy_notice_version" || path === "form_started_at") return "consents";
  return path;
}

// ---------------------------------------------------------------------------
// Payload builder
// ---------------------------------------------------------------------------

function clean(value: string | undefined): string | undefined {
  const v = value?.trim();
  return v ? v : undefined;
}

function compact<T extends Record<string, unknown>>(obj: T): T {
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(obj)) if (v !== undefined) out[k] = v;
  return out as T;
}

export function buildSubmissionPayload(values: RegistrationValues, meta: { formStartedAt: number }): SupplierSubmissionRequest {
  const year = clean(values.company.establishment_year);
  const contacts: NonNullable<SupplierSubmissionRequest["business_contacts"]> = [];
  const email = clean(values.contacts.public_business_email);
  if (email) contacts.push({ contact_type: "public_business_email", channel: "email", value: email });
  const switchboard = clean(values.contacts.switchboard);
  if (switchboard) contacts.push({ contact_type: "switchboard", channel: "phone", value: switchboard });
  const sales = clean(values.contacts.sales_department);
  if (sales) contacts.push({ contact_type: "sales_department", channel: sales.includes("@") ? "email" : "phone", value: sales });

  const products = values.products
    .filter((p) => p.name.trim())
    .map((p) => compact({ name: p.name.trim(), description: clean(p.description), unit: clean(p.unit) }));

  return {
    company: compact({
      display_name: values.company.display_name.trim(),
      legal_name: clean(values.company.legal_name),
      description: clean(values.company.description),
      website_url: clean(values.company.website_url),
      establishment_year: year ? Number(year) : undefined,
    }),
    roles: [...values.roles],
    categories: [...values.categories],
    products,
    facilities: values.facilities.map((f) =>
      compact({
        facility_type: f.facility_type as FacilityType,
        address_line1: clean(f.address_line1),
        locality: clean(f.locality),
        city: f.city.trim(),
        district: clean(f.district),
        state_code: f.state_code,
        postal_code: clean(f.postal_code),
      }),
    ),
    business_contacts: contacts,
    submitter: compact({
      name: values.submitter.name.trim(),
      email: values.submitter.email.trim(),
      designation: clean(values.submitter.designation),
      phone: clean(values.submitter.phone),
    }),
    consents: {
      privacy_notice_acknowledged: values.consents.privacy_notice_acknowledged === true,
      public_listing: values.consents.public_listing === true,
      commercial_api: values.consents.commercial_api === true,
    },
    privacy_notice_version: PRIVACY_NOTICE_VERSION,
    form_started_at: meta.formStartedAt,
    website_confirm: values.website_confirm,
  };
}

export function newIdempotencyKey(): string {
  const c = globalThis.crypto;
  if (c && typeof c.randomUUID === "function") return c.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}`;
}
