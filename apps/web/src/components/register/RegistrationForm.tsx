"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";
import { Controller, useFieldArray, useForm, useWatch, type Control, type FieldPath } from "react-hook-form";
import { Field } from "@/components/ui/Field";
import { ApiError, toApiError } from "@/lib/api-error";
import { PRIVACY_NOTICE_VERSION } from "@/lib/constants";
import { FACILITY_TYPES, facilityLabel } from "@/lib/labels";
import {
  buildSubmissionPayload,
  defaultRegistrationValues,
  emptyFacility,
  emptyProduct,
  formPathForServerField,
  newIdempotencyKey,
  registrationSchema,
  stepForServerField,
  STEPS,
  type RegistrationValues,
} from "@/lib/registration";
import { flattenTree } from "@/lib/tree";
import type { AcceptedResponse, CategoryNode, Role, State } from "@/lib/types";
import { CategoryPicker } from "./CategoryPicker";

export type RegistrationOptions = {
  roles: Role[];
  categoryTree: CategoryNode[];
  states: State[];
};

const MAX_PRODUCTS = 30;
const MAX_FACILITIES = 10;

function useOptions(initial: RegistrationOptions) {
  const [options, setOptions] = useState(initial);
  const [loadError, setLoadError] = useState(false);
  const missing = initial.roles.length === 0 || initial.categoryTree.length === 0 || initial.states.length === 0;

  useEffect(() => {
    if (!missing) return;
    let cancelled = false;
    (async () => {
      try {
        const [r, c, s] = await Promise.all([
          fetch("/api/v1/roles").then((res) => (res.ok ? res.json() : Promise.reject(res))),
          fetch("/api/v1/categories/tree").then((res) => (res.ok ? res.json() : Promise.reject(res))),
          fetch("/api/v1/states").then((res) => (res.ok ? res.json() : Promise.reject(res))),
        ]);
        if (!cancelled) setOptions({ roles: r.data ?? [], categoryTree: c.data ?? [], states: s.data ?? [] });
      } catch {
        if (!cancelled) setLoadError(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [missing]);

  return { options, loadError };
}

function getError(errors: unknown, path: string): string | undefined {
  let cur: unknown = errors;
  for (const part of path.split(".")) {
    if (!cur || typeof cur !== "object") return undefined;
    cur = (cur as Record<string, unknown>)[part];
  }
  if (cur && typeof cur === "object") {
    const msg = (cur as { message?: unknown }).message;
    if (typeof msg === "string") return msg;
    const root = (cur as { root?: { message?: unknown } }).root;
    if (root && typeof root.message === "string") return root.message;
  }
  return undefined;
}

export function RegistrationForm({ initialOptions }: { initialOptions: RegistrationOptions }) {
  const router = useRouter();
  const { options, loadError } = useOptions(initialOptions);
  const [step, setStep] = useState(0);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const formStartedAt = useRef<number>(0);
  const idempotencyKey = useRef<string>("");
  const headingRef = useRef<HTMLHeadingElement>(null);
  const firstRender = useRef(true);

  useEffect(() => {
    formStartedAt.current = Date.now();
    idempotencyKey.current = newIdempotencyKey();
  }, []);

  const {
    register,
    control,
    handleSubmit,
    trigger,
    setError,
    formState: { errors },
  } = useForm<RegistrationValues>({
    resolver: zodResolver(registrationSchema),
    defaultValues: defaultRegistrationValues(),
    mode: "onTouched",
  });

  const products = useFieldArray({ control, name: "products" });
  const facilities = useFieldArray({ control, name: "facilities" });

  useEffect(() => {
    if (firstRender.current) {
      firstRender.current = false;
      return;
    }
    headingRef.current?.focus();
  }, [step]);

  const flatCategories = useMemo(() => flattenTree(options.categoryTree), [options.categoryTree]);
  const categoryNames = useMemo(
    () => Object.fromEntries(flatCategories.map(({ node, path }) => [node.slug, [...path, node.name].join(" › ")])),
    [flatCategories],
  );
  const roleNames = useMemo(() => Object.fromEntries(options.roles.map((r) => [r.code, r.name])), [options.roles]);
  const stateNames = useMemo(() => Object.fromEntries(options.states.map((s) => [s.code, s.name])), [options.states]);

  const isLast = step === STEPS.length - 1;

  async function goNext() {
    setFormError(null);
    const fields = STEPS[step]?.fields ?? [];
    const ok = await trigger(fields as unknown as FieldPath<RegistrationValues>[], { shouldFocus: true });
    if (ok) setStep((s) => Math.min(s + 1, STEPS.length - 1));
    else setFormError("Please fix the highlighted fields before continuing.");
  }

  function goBack() {
    setFormError(null);
    setStep((s) => Math.max(0, s - 1));
  }

  async function submit(values: RegistrationValues) {
    setSubmitting(true);
    setFormError(null);
    try {
      const payload = buildSubmissionPayload(values, { formStartedAt: formStartedAt.current || Date.now() });
      const res = await fetch("/api/v1/supplier-submissions", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
          "Idempotency-Key": idempotencyKey.current || newIdempotencyKey(),
        },
        body: JSON.stringify(payload),
      });
      if (!res.ok) throw await toApiError(res);
      const body = (await res.json().catch(() => ({}))) as Partial<AcceptedResponse>;
      router.push(`/register-supplier/submitted${body.id ? `?ref=${encodeURIComponent(body.id)}` : ""}`);
    } catch (err) {
      if (err instanceof ApiError && err.code === "validation_failed" && Object.keys(err.fieldErrors).length) {
        let firstStep = STEPS.length - 1;
        for (const [path, message] of Object.entries(err.fieldErrors)) {
          firstStep = Math.min(firstStep, stepForServerField(path));
          setError(formPathForServerField(path) as FieldPath<RegistrationValues>, { type: "server", message });
        }
        setStep(firstStep);
        setFormError("Some details need attention. Please review the highlighted fields.");
      } else if (err instanceof ApiError && err.status === 429) {
        setFormError("Too many submissions from your connection. Please wait a few minutes and try again.");
      } else if (err instanceof ApiError) {
        setFormError(`${err.message}${err.requestId ? ` (reference ${err.requestId})` : ""}`);
      } else {
        setFormError("We couldn't send your submission. Please check your connection and try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  const onFormSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    if (!isLast) {
      e.preventDefault();
      void goNext();
      return;
    }
    void handleSubmit(submit, () => {
      setFormError("Please fix the highlighted fields before submitting.");
    })(e);
  };

  const categoryError = getError(errors, "categories");
  const roleError = getError(errors, "roles");

  return (
    <div className="space-y-6">
      <ol className="grid grid-cols-6 gap-1" aria-label="Form progress">
        {STEPS.map((s, i) => (
          <li key={s.id} aria-current={i === step ? "step" : undefined} className="space-y-1">
            <span className={`block h-1.5 rounded-full ${i <= step ? "bg-accent-700" : "bg-slate-200"}`} aria-hidden="true" />
            <span className={`hidden text-xs sm:block ${i === step ? "font-semibold text-slate-900" : "text-slate-600"}`}>{s.title}</span>
            <span className="sr-only">
              Step {i + 1}: {s.title}
              {i < step ? " (completed)" : i === step ? " (current)" : ""}
            </span>
          </li>
        ))}
      </ol>

      {loadError && (
        <div role="alert" className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-900">
          We couldn&apos;t load the lists of roles, categories or states. Please refresh the page or try again later.
        </div>
      )}

      <form noValidate onSubmit={onFormSubmit} className="card space-y-6" aria-labelledby="step-heading">
        <div>
          <p className="text-sm text-slate-600">
            Step {step + 1} of {STEPS.length}
          </p>
          <h2 id="step-heading" ref={headingRef} tabIndex={-1} className="text-xl font-semibold focus:outline-none">
            {STEPS[step]?.title}
          </h2>
          <p className="mt-1 text-xs text-slate-600">
            Fields marked <span className="text-red-700">*</span> are required.
          </p>
        </div>

        <div aria-live="assertive" role="status">
          {formError && <p className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-900">{formError}</p>}
        </div>

        {/* Honeypot: hidden from people and assistive tech. Bots tend to fill it. */}
        <div aria-hidden="true" className="pointer-events-none absolute -left-[10000px] h-px w-px overflow-hidden">
          <label htmlFor="website_confirm">Leave this field empty</label>
          <input id="website_confirm" type="text" tabIndex={-1} autoComplete="off" {...register("website_confirm")} />
        </div>

        {step === 0 && (
          <fieldset className="space-y-4">
            <legend className="sr-only">Business identity</legend>
            <Field id="display_name" label="Business name (as you want it shown)" required error={getError(errors, "company.display_name")} hint="e.g. Shree Ganesh Namkeen or Precision CNC Works">
              <input className="input" autoComplete="organization" {...register("company.display_name")} />
            </Field>
            <Field id="legal_name" label="Registered legal name" optional error={getError(errors, "company.legal_name")} hint="If different, e.g. Shree Ganesh Foods Private Limited">
              <input className="input" {...register("company.legal_name")} />
            </Field>
            <Field
              id="description"
              label="What does the business make or do?"
              optional
              error={getError(errors, "company.description")}
              hint="Two or three factual sentences: main products, processes, materials, typical customers. Up to 2000 characters."
            >
              <textarea rows={5} className="input" {...register("company.description")} />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field id="website_url" label="Website" optional error={getError(errors, "company.website_url")} hint="Full address, e.g. https://example.in">
                <input type="url" inputMode="url" className="input" autoComplete="url" {...register("company.website_url")} />
              </Field>
              <Field id="establishment_year" label="Year established" optional error={getError(errors, "company.establishment_year")} hint="e.g. 1998">
                <input inputMode="numeric" maxLength={4} className="input" {...register("company.establishment_year")} />
              </Field>
            </div>
          </fieldset>
        )}

        {step === 1 && (
          <div className="space-y-6">
            <fieldset aria-describedby={roleError ? "roles-error" : "roles-hint"}>
              <legend className="label">
                Business roles <span className="text-red-700" aria-hidden="true">*</span>
              </legend>
              <p id="roles-hint" className="hint mb-2">
                Choose every role that applies (at least one). For example, a rug maker that also exports would pick Manufacturer and
                Exporter.
              </p>
              <Controller
                control={control}
                name="roles"
                render={({ field }) => (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {options.roles.map((r) => {
                      const checked = field.value.includes(r.code);
                      return (
                        <label key={r.code} className="flex cursor-pointer items-start gap-2 rounded-md border border-slate-200 px-3 py-2 hover:bg-slate-50">
                          <input
                            type="checkbox"
                            className="mt-1 size-4"
                            checked={checked}
                            onBlur={field.onBlur}
                            onChange={(e) =>
                              field.onChange(e.target.checked ? [...field.value, r.code] : field.value.filter((v) => v !== r.code))
                            }
                          />
                          <span>
                            <span className="block text-sm font-medium">{r.name}</span>
                            {r.description && <span className="block text-xs text-slate-600">{r.description}</span>}
                          </span>
                        </label>
                      );
                    })}
                  </div>
                )}
              />
              {roleError && (
                <p id="roles-error" className="field-error" role="alert">
                  {roleError}
                </p>
              )}
            </fieldset>

            <Controller
              control={control}
              name="categories"
              render={({ field }) => (
                <CategoryPicker categories={flatCategories} value={field.value} onChange={field.onChange} max={10} error={categoryError} />
              )}
            />
          </div>
        )}

        {step === 2 && (
          <fieldset className="space-y-4">
            <legend className="sr-only">Products</legend>
            <p className="text-sm text-slate-700">
              List your main products or services (optional, up to {MAX_PRODUCTS}). Plain names work best, e.g. &ldquo;Hand-knotted wool
              rugs&rdquo; or &ldquo;CNC turned brass fittings&rdquo;.
            </p>
            {products.fields.length === 0 && <p className="text-sm text-slate-600">No products added yet.</p>}
            <ul className="space-y-4">
              {products.fields.map((f, i) => (
                <li key={f.id} className="rounded-md border border-slate-200 p-4">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-sm font-semibold">Product {i + 1}</h3>
                    <button type="button" className="btn-ghost btn-sm" onClick={() => products.remove(i)}>
                      Remove<span className="sr-only"> product {i + 1}</span>
                    </button>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-[2fr_1fr]">
                    <Field id={`products.${i}.name`} label="Product name" required error={getError(errors, `products.${i}.name`)}>
                      <input className="input" {...register(`products.${i}.name` as const)} />
                    </Field>
                    <Field id={`products.${i}.unit`} label="Unit" optional error={getError(errors, `products.${i}.unit`)} hint="e.g. kg, pieces, sq. m">
                      <input className="input" {...register(`products.${i}.unit` as const)} />
                    </Field>
                    <Field id={`products.${i}.description`} label="Short description" optional className="sm:col-span-2" error={getError(errors, `products.${i}.description`)}>
                      <textarea rows={2} className="input" {...register(`products.${i}.description` as const)} />
                    </Field>
                  </div>
                </li>
              ))}
            </ul>
            {getError(errors, "products") && <p className="field-error">{getError(errors, "products")}</p>}
            <button type="button" className="btn-secondary" disabled={products.fields.length >= MAX_PRODUCTS} onClick={() => products.append(emptyProduct())}>
              Add a product
            </button>
          </fieldset>
        )}

        {step === 3 && (
          <fieldset className="space-y-4">
            <legend className="sr-only">Facilities</legend>
            <p className="text-sm text-slate-700">
              Add at least one place where the business operates. Only the locality, city, district and state may be shown publicly — street
              address and PIN code are kept private.
            </p>
            <ul className="space-y-4">
              {facilities.fields.map((f, i) => (
                <li key={f.id} className="rounded-md border border-slate-200 p-4">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-sm font-semibold">Facility {i + 1}</h3>
                    {facilities.fields.length > 1 && (
                      <button type="button" className="btn-ghost btn-sm" onClick={() => facilities.remove(i)}>
                        Remove<span className="sr-only"> facility {i + 1}</span>
                      </button>
                    )}
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Field id={`facilities.${i}.facility_type`} label="Facility type" required error={getError(errors, `facilities.${i}.facility_type`)}>
                      <select className="input" {...register(`facilities.${i}.facility_type` as const)}>
                        {FACILITY_TYPES.map((t) => (
                          <option key={t.value} value={t.value}>
                            {t.label}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field id={`facilities.${i}.address_line1`} label="Street address" optional error={getError(errors, `facilities.${i}.address_line1`)} hint="e.g. Plot 14, GIDC Estate">
                      <input className="input" autoComplete="address-line1" {...register(`facilities.${i}.address_line1` as const)} />
                    </Field>
                    <Field id={`facilities.${i}.locality`} label="Locality / industrial area" optional error={getError(errors, `facilities.${i}.locality`)}>
                      <input className="input" {...register(`facilities.${i}.locality` as const)} />
                    </Field>
                    <Field id={`facilities.${i}.city`} label="City or town" required error={getError(errors, `facilities.${i}.city`)} hint="e.g. Rajkot">
                      <input className="input" autoComplete="address-level2" {...register(`facilities.${i}.city` as const)} />
                    </Field>
                    <Field id={`facilities.${i}.district`} label="District" optional error={getError(errors, `facilities.${i}.district`)}>
                      <input className="input" {...register(`facilities.${i}.district` as const)} />
                    </Field>
                    <Field id={`facilities.${i}.state_code`} label="State / union territory" required error={getError(errors, `facilities.${i}.state_code`)}>
                      <select className="input" {...register(`facilities.${i}.state_code` as const)}>
                        <option value="">Choose…</option>
                        {options.states.map((s) => (
                          <option key={s.code} value={s.code}>
                            {s.name}
                          </option>
                        ))}
                      </select>
                    </Field>
                    <Field id={`facilities.${i}.postal_code`} label="PIN code" optional error={getError(errors, `facilities.${i}.postal_code`)} hint="6 digits, e.g. 360002">
                      <input inputMode="numeric" maxLength={6} className="input" autoComplete="postal-code" {...register(`facilities.${i}.postal_code` as const)} />
                    </Field>
                  </div>
                </li>
              ))}
            </ul>
            {getError(errors, "facilities") && <p className="field-error">{getError(errors, "facilities")}</p>}
            <button type="button" className="btn-secondary" disabled={facilities.fields.length >= MAX_FACILITIES} onClick={() => facilities.append(emptyFacility())}>
              Add another facility
            </button>
          </fieldset>
        )}

        {step === 4 && (
          <div className="space-y-6">
            <fieldset className="space-y-4">
              <legend className="text-base font-semibold">Business contact points (optional)</legend>
              <p className="text-sm text-slate-700">
                Shared business contacts — not personal ones. These may be shown on your public profile only if you give public listing
                permission in the next step.
              </p>
              {getError(errors, "contacts") && <p className="field-error">{getError(errors, "contacts")}</p>}
              <div className="grid gap-4 sm:grid-cols-3">
                <Field id="contacts.public_business_email" label="Business email" optional error={getError(errors, "contacts.public_business_email")} hint="e.g. sales@yourcompany.in">
                  <input type="email" className="input" {...register("contacts.public_business_email")} />
                </Field>
                <Field id="contacts.switchboard" label="Office / switchboard phone" optional error={getError(errors, "contacts.switchboard")} hint="e.g. +91 281 234 5678">
                  <input type="tel" className="input" {...register("contacts.switchboard")} />
                </Field>
                <Field id="contacts.sales_department" label="Sales department" optional error={getError(errors, "contacts.sales_department")} hint="Email or phone">
                  <input className="input" {...register("contacts.sales_department")} />
                </Field>
              </div>
            </fieldset>

            <fieldset className="space-y-4 rounded-md border border-accent-200 bg-accent-50 p-4">
              <legend className="px-1 text-base font-semibold">About you (kept private)</legend>
              <p className="text-sm text-slate-800">
                <strong>Your personal details are never published or included in any data product.</strong> We use them only to review this
                submission, contact you about it, and handle your requests.
              </p>
              <div className="grid gap-4 sm:grid-cols-2">
                <Field id="submitter.name" label="Your name" required error={getError(errors, "submitter.name")}>
                  <input className="input bg-white" autoComplete="name" {...register("submitter.name")} />
                </Field>
                <Field id="submitter.email" label="Your email" required error={getError(errors, "submitter.email")} hint="We'll use this to contact you about the submission.">
                  <input type="email" className="input bg-white" autoComplete="email" {...register("submitter.email")} />
                </Field>
                <Field id="submitter.designation" label="Your designation" optional error={getError(errors, "submitter.designation")} hint="e.g. Proprietor, Sales Manager">
                  <input className="input bg-white" autoComplete="organization-title" {...register("submitter.designation")} />
                </Field>
                <Field id="submitter.phone" label="Your phone" optional error={getError(errors, "submitter.phone")}>
                  <input type="tel" className="input bg-white" autoComplete="tel" {...register("submitter.phone")} />
                </Field>
              </div>
            </fieldset>
          </div>
        )}

        {step === 5 && (
          <div className="space-y-6">
            <fieldset className="space-y-4">
              <legend className="text-base font-semibold">Permissions</legend>
              <p className="text-sm text-slate-700">
                These are three separate choices. Only the first is required. You can withdraw the optional ones later through the{" "}
                <Link href="/request-correction" className="link" target="_blank">
                  request form
                </Link>
                .
              </p>

              <div className="rounded-md border border-slate-300 p-4">
                <label className="flex items-start gap-3">
                  <input
                    type="checkbox"
                    className="mt-1 size-5 shrink-0"
                    aria-describedby="consent-privacy-desc consent-privacy-error"
                    aria-invalid={getError(errors, "consents.privacy_notice_acknowledged") ? true : undefined}
                    {...register("consents.privacy_notice_acknowledged")}
                  />
                  <span>
                    <span className="block font-medium">
                      I have read the privacy notice and understand how this information will be processed{" "}
                      <span className="text-red-700">(required)</span>
                    </span>
                    <span id="consent-privacy-desc" className="mt-1 block text-sm text-slate-700">
                      We will store your submission privately and review it manually. Read the{" "}
                      <Link href="/privacy" className="link" target="_blank">
                        privacy notice (version {PRIVACY_NOTICE_VERSION})
                      </Link>{" "}
                      and{" "}
                      <Link href="/data-policy" className="link" target="_blank">
                        data policy
                      </Link>
                      .
                    </span>
                  </span>
                </label>
                {getError(errors, "consents.privacy_notice_acknowledged") && (
                  <p id="consent-privacy-error" className="field-error" role="alert">
                    {getError(errors, "consents.privacy_notice_acknowledged")}
                  </p>
                )}
              </div>

              <div className="rounded-md border border-slate-300 p-4">
                <label className="flex items-start gap-3">
                  <input type="checkbox" className="mt-1 size-5 shrink-0" aria-describedby="consent-public-desc" {...register("consents.public_listing")} />
                  <span>
                    <span className="block font-medium">
                      Publish a public profile for this business after review <span className="font-normal text-slate-600">(optional)</span>
                    </span>
                    <span id="consent-public-desc" className="mt-1 block text-sm text-slate-700">
                      If ticked and approved, a public page may show the business name, description, website, roles, categories, products,
                      facility city/state and the business contacts above. Search engines may index it. Your personal details are never shown.
                      If you leave this unticked, your details stay private.
                    </span>
                  </span>
                </label>
              </div>

              <div className="rounded-md border border-slate-300 p-4">
                <label className="flex items-start gap-3">
                  <input type="checkbox" className="mt-1 size-5 shrink-0" aria-describedby="consent-api-desc" {...register("consents.commercial_api")} />
                  <span>
                    <span className="block font-medium">
                      Allow future commercial/API distribution of the listed business fields{" "}
                      <span className="font-normal text-slate-600">(optional)</span>
                    </span>
                    <span id="consent-api-desc" className="mt-1 block text-sm text-slate-700">
                      In future we may offer a paid data service or API to other businesses. If ticked, it could include this business&apos;s
                      name, categories, products, city/state and business contacts. We do not sell data today. Your personal details are never
                      included. This is separate from public listing.
                    </span>
                  </span>
                </label>
              </div>
            </fieldset>

            <section aria-labelledby="review-h" className="space-y-3">
              <h3 id="review-h" className="text-base font-semibold">
                Review your submission
              </h3>
              <ReviewSummary control={control} roleNames={roleNames} categoryNames={categoryNames} stateNames={stateNames} onEdit={setStep} />
            </section>

            <p className="text-sm text-slate-700">
              After you submit, a person will review your details. Nothing is published before approval, and only with public listing
              permission.
            </p>
          </div>
        )}

        <div className="flex flex-col-reverse gap-3 border-t border-slate-200 pt-4 sm:flex-row sm:justify-between">
          {step > 0 ? (
            <button type="button" className="btn-secondary" onClick={goBack} disabled={submitting}>
              Back
            </button>
          ) : (
            <span />
          )}
          {isLast ? (
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? "Submitting…" : "Submit for review"}
            </button>
          ) : (
            <button type="submit" className="btn-primary">
              Continue
            </button>
          )}
        </div>
      </form>
      <p className="text-xs text-slate-600">
        Your answers are kept only in this browser tab until you submit. Nothing is saved to your device.
      </p>
    </div>
  );
}

function Row({ label, value }: { label: string; value?: React.ReactNode }) {
  if (!value) return null;
  return (
    <div className="grid gap-1 sm:grid-cols-[12rem_1fr]">
      <dt className="text-slate-600">{label}</dt>
      <dd className="break-words text-slate-900">{value}</dd>
    </div>
  );
}

function Block({ title, step, onEdit, children }: { title: string; step: number; onEdit: (step: number) => void; children: React.ReactNode }) {
  return (
    <div className="rounded-md border border-slate-200 p-4">
      <div className="mb-2 flex items-center justify-between">
        <h4 className="text-sm font-semibold">{title}</h4>
        <button type="button" className="btn-ghost btn-sm" onClick={() => onEdit(step)}>
          Edit<span className="sr-only"> {title}</span>
        </button>
      </div>
      <dl className="space-y-1 text-sm">{children}</dl>
    </div>
  );
}

function ReviewSummary({
  control,
  roleNames,
  categoryNames,
  stateNames,
  onEdit,
}: {
  control: Control<RegistrationValues>;
  roleNames: Record<string, string>;
  categoryNames: Record<string, string>;
  stateNames: Record<string, string>;
  onEdit: (step: number) => void;
}) {
  const values = useWatch({ control }) as RegistrationValues;
  const contacts = [values.contacts.public_business_email, values.contacts.switchboard, values.contacts.sales_department].filter((v) => v.trim());

  return (
    <div className="space-y-3">
      <Block title="Business identity" step={0} onEdit={onEdit}>
        <Row label="Business name" value={values.company.display_name} />
        <Row label="Legal name" value={values.company.legal_name} />
        <Row label="Description" value={values.company.description} />
        <Row label="Website" value={values.company.website_url} />
        <Row label="Year established" value={values.company.establishment_year} />
      </Block>
      <Block title="Roles and categories" step={1} onEdit={onEdit}>
        <Row label="Roles" value={values.roles.map((r) => roleNames[r] ?? r).join(", ")} />
        <Row label="Categories" value={values.categories.map((c) => categoryNames[c] ?? c).join("; ")} />
      </Block>
      <Block title="Products" step={2} onEdit={onEdit}>
        {values.products.length === 0 ? (
          <p className="text-slate-600">None listed</p>
        ) : (
          <ul className="list-disc pl-5">
            {values.products.map((p, i) => (
              <li key={i}>
                {p.name}
                {p.unit ? ` (${p.unit})` : ""}
                {p.description ? ` — ${p.description}` : ""}
              </li>
            ))}
          </ul>
        )}
      </Block>
      <Block title="Facilities" step={3} onEdit={onEdit}>
        <ul className="list-disc pl-5">
          {values.facilities.map((f, i) => (
            <li key={i}>
              {facilityLabel(f.facility_type)}:{" "}
              {[f.address_line1, f.locality, f.city, f.district, stateNames[f.state_code] ?? f.state_code, f.postal_code].filter((v) => v && v.trim()).join(", ")}
            </li>
          ))}
        </ul>
      </Block>
      <Block title="Contacts" step={4} onEdit={onEdit}>
        <Row label="Business contacts" value={contacts.length ? contacts.join(", ") : "None"} />
        <Row label="Your name (private)" value={values.submitter.name} />
        <Row label="Your email (private)" value={values.submitter.email} />
        <Row label="Designation (private)" value={values.submitter.designation} />
        <Row label="Phone (private)" value={values.submitter.phone} />
      </Block>
    </div>
  );
}
