"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useState } from "react";
import { useForm, type FieldPath } from "react-hook-form";
import { z } from "zod";
import { Field } from "@/components/ui/Field";
import { ApiError, toApiError } from "@/lib/api-error";
import { PRIVACY_REQUEST_TYPES } from "@/lib/labels";
import type { AcceptedResponse, PrivacyRequest, PrivacyRequestType } from "@/lib/types";

const TYPES = PRIVACY_REQUEST_TYPES.map((t) => t.value) as [PrivacyRequestType, ...PrivacyRequestType[]];

const schema = z.object({
  request_type: z.enum(TYPES, { message: "Choose what you would like us to do" }),
  requester_name: z
    .string()
    .max(200, "Name must be 200 characters or fewer")
    .refine((v) => v.trim().length > 0, "Your name is required"),
  email: z.string().refine((v) => z.email().safeParse(v.trim()).success, "Enter a valid email address so we can reply"),
  company_reference: z.string().max(300, "Too long"),
  details: z.string().max(5000, "Please keep details under 5000 characters"),
  website_confirm: z.string(),
});

type Values = z.infer<typeof schema>;

export function buildPrivacyRequestPayload(v: Values): PrivacyRequest {
  const out: PrivacyRequest = {
    request_type: v.request_type,
    requester_name: v.requester_name.trim(),
    email: v.email.trim(),
    website_confirm: v.website_confirm,
  };
  if (v.company_reference.trim()) out.company_reference = v.company_reference.trim();
  if (v.details.trim()) out.details = v.details.trim();
  return out;
}

export function CorrectionRequestForm({ companyReference, privacyEmail }: { companyReference?: string; privacyEmail: string }) {
  const [done, setDone] = useState<AcceptedResponse | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      request_type: (companyReference ? "correction" : "") as PrivacyRequestType,
      requester_name: "",
      email: "",
      company_reference: companyReference ?? "",
      details: "",
      website_confirm: "",
    },
  });

  async function onSubmit(values: Values) {
    setFormError(null);
    try {
      const res = await fetch("/api/v1/privacy-requests", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify(buildPrivacyRequestPayload(values)),
      });
      if (!res.ok) throw await toApiError(res);
      setDone((await res.json().catch(() => ({ id: "", status: "received" }))) as AcceptedResponse);
    } catch (err) {
      if (err instanceof ApiError && err.code === "validation_failed") {
        for (const [path, message] of Object.entries(err.fieldErrors)) setError(path as FieldPath<Values>, { type: "server", message });
        setFormError("Please check the highlighted fields.");
      } else if (err instanceof ApiError) {
        setFormError(err.message);
      } else {
        setFormError("We couldn't send your request. Please try again, or email us.");
      }
    }
  }

  if (done) {
    return (
      <div role="status" className="card space-y-3 border-emerald-300 bg-emerald-50">
        <h2 className="text-xl font-semibold text-emerald-950">Your request has been received</h2>
        {done.id && (
          <p className="text-sm">
            Reference: <span className="font-mono">{done.id}</span>
          </p>
        )}
        <p className="text-slate-800">
          A person will review it and reply by email. If you asked us to withdraw a public listing or remove information, we will act on it
          as soon as we have confirmed the request. If you need to add anything, email{" "}
          <a href={`mailto:${privacyEmail}`} className="link">
            {privacyEmail}
          </a>
          .
        </p>
      </div>
    );
  }

  return (
    <form noValidate onSubmit={handleSubmit(onSubmit)} className="card space-y-4">
      <div aria-live="assertive" role="status">
        {formError && <p className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-900">{formError}</p>}
      </div>
      <div aria-hidden="true" className="pointer-events-none absolute -left-[10000px] h-px w-px overflow-hidden">
        <label htmlFor="pr-website-confirm">Leave this field empty</label>
        <input id="pr-website-confirm" type="text" tabIndex={-1} autoComplete="off" {...register("website_confirm")} />
      </div>
      <Field id="request_type" label="What would you like us to do?" required error={errors.request_type?.message}>
        <select className="input" {...register("request_type")}>
          <option value="" disabled>
            Choose…
          </option>
          {PRIVACY_REQUEST_TYPES.map((t) => (
            <option key={t.value} value={t.value}>
              {t.label}
            </option>
          ))}
        </select>
      </Field>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field id="requester_name" label="Your name" required error={errors.requester_name?.message}>
          <input className="input" autoComplete="name" {...register("requester_name")} />
        </Field>
        <Field id="email" label="Your email" required error={errors.email?.message} hint="We'll reply to this address.">
          <input type="email" className="input" autoComplete="email" {...register("email")} />
        </Field>
      </div>
      <Field
        id="company_reference"
        label="Which business or profile?"
        optional
        error={errors.company_reference?.message}
        hint="Business name, profile address or reference number."
      >
        <input className="input" {...register("company_reference")} />
      </Field>
      <Field id="details" label="Details" optional error={errors.details?.message} hint="What should be changed or removed, and why. Please don't include sensitive personal information.">
        <textarea rows={6} className="input" {...register("details")} />
      </Field>
      <p className="text-xs text-slate-600">
        We use these details only to handle your request, as described in the <a href="/privacy" className="link">privacy notice</a>.
      </p>
      <button type="submit" className="btn-primary" disabled={isSubmitting}>
        {isSubmitting ? "Sending…" : "Send request"}
      </button>
    </form>
  );
}
