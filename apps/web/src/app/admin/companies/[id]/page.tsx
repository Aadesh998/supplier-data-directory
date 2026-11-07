"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useMemo, useState, type FormEvent } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { SourceSelect } from "@/components/admin/SourceSelect";
import { BoolBadge, ErrorState, LoadingState, PageHeader, Section, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { ApiError, errorMessage } from "@/lib/api-error";
import { categoryPathLabels, useAdminCategories, useRoles, useStates } from "@/lib/admin/hooks";
import { FACILITY_TYPES, facilityLabel, formatDate, formatDateTime, humanize } from "@/lib/labels";
import type { AdminCompany, CompanyInput } from "@/lib/types";

type Setter = (c: AdminCompany) => void;

function useCompanyMutation<TVars>(id: string, fn: (vars: TVars) => Promise<AdminCompany>, success: string) {
  const qc = useQueryClient();
  const toast = useToast();
  return useMutation({
    mutationFn: fn,
    onSuccess: (data) => {
      qc.setQueryData(["admin", "company", id], data);
      void qc.invalidateQueries({ queryKey: ["admin", "companies"] });
      toast.success(success);
    },
    onError: (e) => {
      if (!(e instanceof ApiError && e.code === "publish_blocked")) toast.error(errorMessage(e));
    },
  });
}

// ---------------------------------------------------------------------------
// Publication
// ---------------------------------------------------------------------------

function PublicationPanel({ company }: { company: AdminCompany }) {
  const api = useApi();
  const { user } = useAuth();
  const [blocked, setBlocked] = useState<string[] | null>(null);
  const [status, setStatus] = useState("");
  const publish = useCompanyMutation(company.id, () => api.post<AdminCompany>(`/admin/companies/${company.id}/publish`), "Company published.");
  const unpublish = useCompanyMutation(
    company.id,
    (reason: string) => api.post<AdminCompany>(`/admin/companies/${company.id}/unpublish`, reason ? { reason } : {}),
    "Company unpublished.",
  );
  const archive = useCompanyMutation(company.id, () => api.post<AdminCompany>(`/admin/companies/${company.id}/archive`), "Company archived.");
  const setPub = useCompanyMutation(
    company.id,
    (s: string) => api.patch<AdminCompany>(`/admin/companies/${company.id}`, { publication_status: s } satisfies CompanyInput),
    "Status updated.",
  );

  const check = company.publish_check;
  const isPublished = company.publication_status === "published";

  function doPublish() {
    setBlocked(null);
    publish.mutate(undefined, {
      onError: (e) => {
        if (e instanceof ApiError && e.code === "publish_blocked") setBlocked(e.reasons.length ? e.reasons : [e.message]);
      },
    });
  }

  return (
    <Section title="Publication" id="pub-h">
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <span>
          Publication: <StatusBadge status={company.publication_status} />
        </span>
        <span>
          Verification: <StatusBadge status={company.verification_status} />
        </span>
        {company.published_at && <span className="text-slate-600">Published {formatDateTime(company.published_at)}</span>}
      </div>
      <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-700">
        <strong>Publishing does not imply verification.</strong> A published profile is shown with its verification status and source labels.
        Only fields whose source permits public display are shown.
      </p>
      {check && !check.allowed && !isPublished && (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-950">
          <p className="font-semibold">Cannot be published yet:</p>
          <ul className="list-disc pl-5">
            {check.reasons.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
      )}
      {blocked && (
        <div role="alert" className="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-950">
          <p className="font-semibold">Publishing was blocked by the policy check:</p>
          <ul className="list-disc pl-5">
            {blocked.map((r) => (
              <li key={r}>{r}</li>
            ))}
          </ul>
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        {!isPublished && (
          <button type="button" className="btn-primary" onClick={doPublish} disabled={publish.isPending}>
            {publish.isPending ? "Publishing…" : "Publish"}
          </button>
        )}
        {isPublished && (
          <button
            type="button"
            className="btn-secondary"
            disabled={unpublish.isPending}
            onClick={() => {
              const reason = window.prompt("Reason for unpublishing (recorded in the audit log):", "") ?? null;
              if (reason !== null) unpublish.mutate(reason);
            }}
          >
            Unpublish
          </button>
        )}
        {company.publication_status !== "archived" && (
          <button
            type="button"
            className="btn-danger"
            disabled={archive.isPending}
            onClick={() => {
              if (window.confirm("Archive this company? It will be removed from the public site.")) archive.mutate(undefined);
            }}
          >
            Archive
          </button>
        )}
        {isPublished && (
          <Link href={`/suppliers/${company.slug}`} className="btn-ghost" target="_blank">
            View public profile
          </Link>
        )}
      </div>
      <form
        className="flex flex-wrap items-end gap-2 border-t border-slate-100 pt-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (status) setPub.mutate(status);
        }}
      >
        <div>
          <label htmlFor="pub-status" className="label">
            Set workflow status
          </label>
          <select id="pub-status" className="input" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">Choose…</option>
            {["draft", "under_review", "approved_private", "needs_changes", "rejected"].map((s) => (
              <option key={s} value={s}>
                {humanize(s)}
              </option>
            ))}
          </select>
        </div>
        <button type="submit" className="btn-secondary" disabled={!status || setPub.isPending}>
          Update status
        </button>
        {user?.role === "reviewer" && <p className="w-full text-xs text-slate-600">As a reviewer, the API may refuse publishing actions.</p>}
      </form>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Core fields
// ---------------------------------------------------------------------------

function CoreFields({ company }: { company: AdminCompany }) {
  const api = useApi();
  const toast = useToast();
  const initial = {
    display_name: company.display_name ?? "",
    legal_name: company.legal_name ?? "",
    description: company.description ?? "",
    website_url: company.website_url ?? "",
    establishment_year: company.establishment_year ? String(company.establishment_year) : "",
    business_status: company.business_status ?? "unknown",
    slug: company.slug ?? "",
  };
  const [form, setForm] = useState(initial);
  const [sourceId, setSourceId] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const m = useCompanyMutation(company.id, (body: CompanyInput) => api.patch<AdminCompany>(`/admin/companies/${company.id}`, body), "Changes saved.");

  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    const body: CompanyInput = {};
    if (form.display_name !== initial.display_name) body.display_name = form.display_name.trim();
    if (form.legal_name !== initial.legal_name) body.legal_name = form.legal_name.trim();
    if (form.description !== initial.description) body.description = form.description.trim();
    if (form.website_url !== initial.website_url) body.website_url = form.website_url.trim();
    if (form.establishment_year !== initial.establishment_year && form.establishment_year.trim())
      body.establishment_year = Number(form.establishment_year);
    if (form.business_status !== initial.business_status) body.business_status = form.business_status;
    if (form.slug !== initial.slug) body.slug = form.slug.trim();
    if (Object.keys(body).length === 0) return toast.info("No changes to save.");
    if (!sourceId) return toast.error("Choose the source of this information before saving.");
    body.source_id = sourceId;
    m.mutate(body, {
      onError: (err) => {
        if (err instanceof ApiError) setFieldErrors(err.fieldErrors);
      },
    });
  }

  const err = (k: string) => fieldErrors[k] && <p className="field-error">{fieldErrors[k]}</p>;

  return (
    <Section title="Core details" id="core-h">
      <form onSubmit={onSubmit} className="space-y-3" noValidate>
        <div className="grid gap-3 md:grid-cols-2">
          <div>
            <label htmlFor="cd-name" className="label">
              Display name
            </label>
            <input id="cd-name" className="input" value={form.display_name} onChange={set("display_name")} />
            {err("display_name")}
          </div>
          <div>
            <label htmlFor="cd-legal" className="label">
              Legal name
            </label>
            <input id="cd-legal" className="input" value={form.legal_name} onChange={set("legal_name")} />
            {err("legal_name")}
          </div>
          <div className="md:col-span-2">
            <label htmlFor="cd-desc" className="label">
              Description
            </label>
            <textarea id="cd-desc" rows={4} className="input" value={form.description} onChange={set("description")} />
            {err("description")}
          </div>
          <div>
            <label htmlFor="cd-web" className="label">
              Website
            </label>
            <input id="cd-web" className="input" value={form.website_url} onChange={set("website_url")} />
            {err("website_url")}
          </div>
          <div>
            <label htmlFor="cd-year" className="label">
              Established
            </label>
            <input id="cd-year" inputMode="numeric" className="input" value={form.establishment_year} onChange={set("establishment_year")} />
            {err("establishment_year")}
          </div>
          <div>
            <label htmlFor="cd-bs" className="label">
              Business status
            </label>
            <select id="cd-bs" className="input" value={form.business_status} onChange={set("business_status")}>
              {["active", "inactive", "closed", "unknown"].map((s) => (
                <option key={s} value={s}>
                  {humanize(s)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="cd-slug" className="label">
              Slug
            </label>
            <input id="cd-slug" className="input font-mono" value={form.slug} onChange={set("slug")} />
            <p className="hint">Changing the slug keeps the old URL as a permanent redirect.</p>
            {err("slug")}
          </div>
        </div>
        <SourceSelect id="cd-source" value={sourceId} onChange={setSourceId} />
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          {m.isPending ? "Saving…" : "Save changes"}
        </button>
      </form>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Roles & categories
// ---------------------------------------------------------------------------

function Classification({ company }: { company: AdminCompany }) {
  const api = useApi();
  const toast = useToast();
  const roles = useRoles();
  const cats = useAdminCategories();
  const [selectedRoles, setSelectedRoles] = useState<string[]>(company.roles ?? []);
  const [selectedCats, setSelectedCats] = useState<string[]>(company.categories.map((c) => c.id).filter((x): x is string => !!x));
  const [filter, setFilter] = useState("");
  const [sourceId, setSourceId] = useState("");
  const labels = useMemo(() => categoryPathLabels(cats.data?.data ?? []), [cats.data]);
  const m = useCompanyMutation(company.id, (body: CompanyInput) => api.patch<AdminCompany>(`/admin/companies/${company.id}`, body), "Roles and categories saved.");

  const visibleCats = (cats.data?.data ?? [])
    .filter((c) => !filter.trim() || (labels[c.id] ?? c.name).toLowerCase().includes(filter.trim().toLowerCase()))
    .sort((a, b) => (labels[a.id] ?? "").localeCompare(labels[b.id] ?? ""));

  return (
    <Section title="Roles and categories" id="class-h">
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (!sourceId) return toast.error("Choose the source of this information before saving.");
          m.mutate({ roles: selectedRoles, category_ids: selectedCats, source_id: sourceId });
        }}
      >
        <fieldset>
          <legend className="label">Business roles</legend>
          <div className="grid gap-1 sm:grid-cols-2 lg:grid-cols-3">
            {roles.data?.data.map((r) => (
              <label key={r.code} className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={selectedRoles.includes(r.code)}
                  onChange={(e) => setSelectedRoles((s) => (e.target.checked ? [...s, r.code] : s.filter((x) => x !== r.code)))}
                />
                {r.name}
              </label>
            ))}
          </div>
        </fieldset>
        <fieldset>
          <legend className="label">Categories ({selectedCats.length} selected)</legend>
          <label htmlFor="cls-filter" className="sr-only">
            Filter categories
          </label>
          <input id="cls-filter" className="input mb-2" placeholder="Filter categories…" value={filter} onChange={(e) => setFilter(e.target.value)} />
          <div className="max-h-56 overflow-y-auto rounded-md border border-slate-200 p-2">
            {cats.isLoading ? (
              <p className="text-sm text-slate-600">Loading…</p>
            ) : (
              visibleCats.map((c) => (
                <label key={c.id} className="flex items-center gap-2 py-0.5 text-sm">
                  <input
                    type="checkbox"
                    checked={selectedCats.includes(c.id)}
                    onChange={(e) => setSelectedCats((s) => (e.target.checked ? [...s, c.id] : s.filter((x) => x !== c.id)))}
                  />
                  {labels[c.id] ?? c.name}
                  {c.active === false && <span className="text-xs text-slate-500">(inactive)</span>}
                </label>
              ))
            )}
          </div>
        </fieldset>
        <SourceSelect id="cls-source" value={sourceId} onChange={setSourceId} />
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          Save roles and categories
        </button>
      </form>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Facilities & products
// ---------------------------------------------------------------------------

const emptyFacility = { facility_type: "factory", name: "", address_line1: "", locality: "", city: "", district: "", state_code: "", postal_code: "", is_primary: false };

function Facilities({ company, onChange }: { company: AdminCompany; onChange: Setter }) {
  const api = useApi();
  const toast = useToast();
  const states = useStates();
  const [form, setForm] = useState(emptyFacility);
  const [sourceId, setSourceId] = useState("");
  const add = useCompanyMutation(
    company.id,
    (body: Record<string, unknown>) => api.post<AdminCompany>(`/admin/companies/${company.id}/facilities`, body),
    "Facility added.",
  );
  const remove = useCompanyMutation(company.id, (fid: string) => api.del<AdminCompany>(`/admin/companies/${company.id}/facilities/${fid}`), "Facility removed.");
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  return (
    <Section title="Facilities" id="fac-h">
      {company.facilities?.length ? (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">Type</th>
                <th scope="col">Address</th>
                <th scope="col">Primary</th>
                <th scope="col">Verification</th>
                <th scope="col">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {company.facilities.map((f) => (
                <tr key={f.id}>
                  <td>{facilityLabel(f.facility_type)}</td>
                  <td>{[f.name, f.address_line1, f.address_line2, f.locality, f.city, f.district, f.state_code, f.postal_code].filter(Boolean).join(", ")}</td>
                  <td>
                    <BoolBadge value={f.is_primary} />
                  </td>
                  <td>
                    <StatusBadge status={f.verification_status} />
                  </td>
                  <td>
                    <button
                      type="button"
                      className="btn-ghost btn-sm text-red-700"
                      onClick={() => window.confirm("Remove this facility?") && remove.mutate(f.id, { onSuccess: onChange })}
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <p className="text-sm text-slate-600">No facilities recorded.</p>
      )}
      <details className="rounded-md border border-slate-200 p-3">
        <summary className="cursor-pointer text-sm font-medium">Add a facility</summary>
        <form
          className="mt-3 space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (!form.city.trim() || !form.state_code) return toast.error("City and state are required.");
            if (!sourceId) return toast.error("Choose the source of this information.");
            const body: Record<string, unknown> = { facility_type: form.facility_type, is_primary: form.is_primary, source_id: sourceId };
            for (const k of ["name", "address_line1", "locality", "city", "district", "state_code", "postal_code"] as const) if (form[k].trim()) body[k] = form[k].trim();
            add.mutate(body, { onSuccess: () => setForm(emptyFacility) });
          }}
        >
          <div className="grid gap-3 md:grid-cols-3">
            <div>
              <label htmlFor="af-type" className="label">
                Type
              </label>
              <select id="af-type" className="input" value={form.facility_type} onChange={set("facility_type")}>
                {FACILITY_TYPES.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </select>
            </div>
            {(
              [
                ["name", "Name"],
                ["address_line1", "Address line 1"],
                ["locality", "Locality"],
                ["city", "City *"],
                ["district", "District"],
                ["postal_code", "PIN code"],
              ] as const
            ).map(([k, label]) => (
              <div key={k}>
                <label htmlFor={`af-${k}`} className="label">
                  {label}
                </label>
                <input id={`af-${k}`} className="input" value={form[k]} onChange={set(k)} />
              </div>
            ))}
            <div>
              <label htmlFor="af-state" className="label">
                State *
              </label>
              <select id="af-state" className="input" value={form.state_code} onChange={set("state_code")}>
                <option value="">Choose…</option>
                {states.data?.data.map((s) => (
                  <option key={s.code} value={s.code}>
                    {s.name}
                  </option>
                ))}
              </select>
            </div>
            <label className="flex items-center gap-2 self-end text-sm">
              <input type="checkbox" checked={form.is_primary} onChange={(e) => setForm((f) => ({ ...f, is_primary: e.target.checked }))} />
              Primary facility
            </label>
          </div>
          <SourceSelect id="af-source" value={sourceId} onChange={setSourceId} />
          <button type="submit" className="btn-primary" disabled={add.isPending}>
            Add facility
          </button>
        </form>
      </details>
    </Section>
  );
}

function Products({ company, onChange }: { company: AdminCompany; onChange: Setter }) {
  const api = useApi();
  const toast = useToast();
  const cats = useAdminCategories();
  const labels = useMemo(() => categoryPathLabels(cats.data?.data ?? []), [cats.data]);
  const [form, setForm] = useState({ name: "", description: "", unit: "", category_id: "" });
  const [sourceId, setSourceId] = useState("");
  const add = useCompanyMutation(
    company.id,
    (body: Record<string, unknown>) => api.post<AdminCompany>(`/admin/companies/${company.id}/products`, body),
    "Product added.",
  );
  const remove = useCompanyMutation(company.id, (pid: string) => api.del<AdminCompany>(`/admin/companies/${company.id}/products/${pid}`), "Product removed.");
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));

  return (
    <Section title="Products" id="prod-h">
      {company.products?.length ? (
        <ul className="divide-y divide-slate-100">
          {company.products.map((p) => (
            <li key={p.id} className="flex items-start justify-between gap-3 py-2 text-sm">
              <div>
                <p className="font-medium">
                  {p.name} {p.unit && <span className="font-normal text-slate-600">({p.unit})</span>}
                </p>
                {p.description && <p className="text-slate-700">{p.description}</p>}
                {p.category && <p className="text-xs text-slate-600">{p.category.name}</p>}
              </div>
              <button type="button" className="btn-ghost btn-sm text-red-700" onClick={() => window.confirm("Remove this product?") && remove.mutate(p.id, { onSuccess: onChange })}>
                Remove<span className="sr-only"> {p.name}</span>
              </button>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-slate-600">No products recorded.</p>
      )}
      <details className="rounded-md border border-slate-200 p-3">
        <summary className="cursor-pointer text-sm font-medium">Add a product</summary>
        <form
          className="mt-3 space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (!form.name.trim()) return toast.error("Product name is required.");
            if (!sourceId) return toast.error("Choose the source of this information.");
            const body: Record<string, unknown> = { name: form.name.trim(), source_id: sourceId };
            if (form.description.trim()) body.description = form.description.trim();
            if (form.unit.trim()) body.unit = form.unit.trim();
            if (form.category_id) body.category_id = form.category_id;
            add.mutate(body, { onSuccess: () => setForm({ name: "", description: "", unit: "", category_id: "" }) });
          }}
        >
          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <label htmlFor="ap-name" className="label">
                Name *
              </label>
              <input id="ap-name" className="input" value={form.name} onChange={set("name")} />
            </div>
            <div>
              <label htmlFor="ap-unit" className="label">
                Unit
              </label>
              <input id="ap-unit" className="input" value={form.unit} onChange={set("unit")} />
            </div>
            <div className="md:col-span-2">
              <label htmlFor="ap-desc" className="label">
                Description
              </label>
              <input id="ap-desc" className="input" value={form.description} onChange={set("description")} />
            </div>
            <div className="md:col-span-2">
              <label htmlFor="ap-cat" className="label">
                Category
              </label>
              <select id="ap-cat" className="input" value={form.category_id} onChange={set("category_id")}>
                <option value="">None</option>
                {(cats.data?.data ?? [])
                  .slice()
                  .sort((a, b) => (labels[a.id] ?? "").localeCompare(labels[b.id] ?? ""))
                  .map((c) => (
                    <option key={c.id} value={c.id}>
                      {labels[c.id]}
                    </option>
                  ))}
              </select>
            </div>
          </div>
          <SourceSelect id="ap-source" value={sourceId} onChange={setSourceId} />
          <button type="submit" className="btn-primary" disabled={add.isPending}>
            Add product
          </button>
        </form>
      </details>
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Read-only: contacts, capabilities, certifications, provenance
// ---------------------------------------------------------------------------

function OtherDetails({ company }: { company: AdminCompany }) {
  return (
    <Section title="Contacts, capabilities and certifications" id="other-h">
      <div className="grid gap-4 md:grid-cols-3">
        <div>
          <h3 className="mb-1 text-sm font-semibold">Contacts</h3>
          {company.contacts?.length ? (
            <ul className="space-y-1 text-sm">
              {company.contacts.map((c) => (
                <li key={c.id}>
                  <span className="text-slate-600">{humanize(c.contact_type)}:</span> <span className="font-mono">{c.value}</span>
                  {c.masked && <span className="ml-1 text-xs text-slate-500">(masked)</span>}{" "}
                  <BoolBadge value={c.is_public} yes="public" no="private" />
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-slate-600">None</p>
          )}
        </div>
        <div>
          <h3 className="mb-1 text-sm font-semibold">Capabilities</h3>
          {company.capabilities?.length ? (
            <ul className="space-y-1 text-sm">
              {company.capabilities.map((c) => (
                <li key={c.id}>
                  {c.name} <StatusBadge status={c.verification_status} />
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-slate-600">None</p>
          )}
        </div>
        <div>
          <h3 className="mb-1 text-sm font-semibold">Certifications</h3>
          {company.certifications?.length ? (
            <ul className="space-y-1 text-sm">
              {company.certifications.map((c) => (
                <li key={c.id}>
                  {c.certification_type}
                  {c.issuer && <span className="text-slate-600"> ({c.issuer})</span>} <StatusBadge status={c.status} />
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-slate-600">None</p>
          )}
        </div>
      </div>
    </Section>
  );
}

function ProvenanceTable({ company }: { company: AdminCompany }) {
  const rows = company.provenance ?? [];
  return (
    <Section title="Field provenance" id="prov-h">
      <p className="text-sm text-slate-700">Where each field came from and what its source allows. Public pages only show fields with public-display permission.</p>
      {rows.length === 0 ? (
        <p className="text-sm text-slate-600">No provenance recorded.</p>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">Entity</th>
                <th scope="col">Field</th>
                <th scope="col">Permission</th>
                <th scope="col">Source</th>
                <th scope="col">Internal</th>
                <th scope="col">Public</th>
                <th scope="col">Commercial</th>
                <th scope="col">Observed</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((p, i) => (
                <tr key={`${p.entity_id}-${p.field_name}-${i}`}>
                  <td className="text-xs">{humanize(p.entity_type)}</td>
                  <td className="font-mono text-xs">{p.field_name}</td>
                  <td>
                    <span
                      className={`badge ${
                        p.permission_status === "permitted"
                          ? "border-emerald-300 bg-emerald-50 text-emerald-900"
                          : p.permission_status === "unknown"
                            ? "border-slate-300 bg-slate-50 text-slate-700"
                            : "border-red-300 bg-red-50 text-red-900"
                      }`}
                    >
                      {p.permission_status}
                    </span>
                  </td>
                  <td className="text-xs">{p.source ? `${p.source.provider} (${humanize(p.source.source_type)})` : "—"}</td>
                  <td>
                    <BoolBadge value={p.source?.allowed_internal_use} />
                  </td>
                  <td>
                    <BoolBadge value={p.source?.allowed_public_display} />
                  </td>
                  <td>
                    <BoolBadge value={p.source?.allowed_commercial_api} />
                  </td>
                  <td className="whitespace-nowrap text-xs">{formatDate(p.observed_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Section>
  );
}

// ---------------------------------------------------------------------------
// Verification
// ---------------------------------------------------------------------------

function Verification({ company }: { company: AdminCompany }) {
  const api = useApi();
  const [form, setForm] = useState({ method: "website_check", result: "confirmed", field_name: "", notes: "", next_review_days: "365" });
  const m = useCompanyMutation(
    company.id,
    (body: Record<string, unknown>) => api.post<AdminCompany>(`/admin/companies/${company.id}/verify`, body),
    "Verification recorded.",
  );
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm((f) => ({ ...f, [k]: e.target.value }));
  const events = company.verification_events ?? [];

  return (
    <Section title="Verification" id="ver-h">
      <div className="flex flex-wrap gap-4 text-sm text-slate-700">
        <span>
          Status: <StatusBadge status={company.verification_status} />
        </span>
        <span>Last verified: {formatDate(company.last_verified_at)}</span>
        <span>Next review: {formatDate(company.next_review_at)}</span>
      </div>
      {events.length === 0 ? (
        <p className="text-sm text-slate-600">No verification events yet.</p>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">When</th>
                <th scope="col">Field</th>
                <th scope="col">Method</th>
                <th scope="col">Result</th>
                <th scope="col">By</th>
                <th scope="col">Notes</th>
                <th scope="col">Next review</th>
              </tr>
            </thead>
            <tbody>
              {events.map((ev) => (
                <tr key={ev.id}>
                  <td className="whitespace-nowrap text-xs">{formatDateTime(ev.checked_at)}</td>
                  <td className="text-xs">{ev.field_name ?? "Whole record"}</td>
                  <td className="text-xs">{humanize(ev.method)}</td>
                  <td>
                    <StatusBadge status={ev.result} />
                  </td>
                  <td className="text-xs">{ev.checked_by}</td>
                  <td className="text-xs">{ev.notes ?? ""}</td>
                  <td className="whitespace-nowrap text-xs">{formatDate(ev.next_review_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <form
        className="space-y-3 border-t border-slate-100 pt-3"
        onSubmit={(e) => {
          e.preventDefault();
          const body: Record<string, unknown> = { method: form.method, result: form.result };
          if (form.field_name.trim()) body.field_name = form.field_name.trim();
          if (form.notes.trim()) body.notes = form.notes.trim();
          const days = Number(form.next_review_days);
          if (Number.isInteger(days) && days >= 1 && days <= 1095) body.next_review_days = days;
          m.mutate(body, { onSuccess: () => setForm((f) => ({ ...f, notes: "", field_name: "" })) });
        }}
      >
        <h3 className="text-sm font-semibold">Record verification</h3>
        <div className="grid gap-3 md:grid-cols-4">
          <div>
            <label htmlFor="v-method" className="label">
              Method
            </label>
            <select id="v-method" className="input" value={form.method} onChange={set("method")}>
              {["supplier_confirmation", "official_document", "website_check", "manual_review", "phone_call"].map((s) => (
                <option key={s} value={s}>
                  {humanize(s)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="v-result" className="label">
              Result
            </label>
            <select id="v-result" className="input" value={form.result} onChange={set("result")}>
              {["confirmed", "partially_confirmed", "not_confirmed", "inconclusive"].map((s) => (
                <option key={s} value={s}>
                  {humanize(s)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="v-field" className="label">
              Field (optional)
            </label>
            <input id="v-field" className="input" value={form.field_name} onChange={set("field_name")} placeholder="e.g. website_url" />
          </div>
          <div>
            <label htmlFor="v-days" className="label">
              Re-check after (days)
            </label>
            <input id="v-days" type="number" min={1} max={1095} className="input" value={form.next_review_days} onChange={set("next_review_days")} />
          </div>
          <div className="md:col-span-4">
            <label htmlFor="v-notes" className="label">
              Evidence notes
            </label>
            <textarea id="v-notes" rows={2} className="input" value={form.notes} onChange={set("notes")} placeholder="What was checked and how" />
          </div>
        </div>
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          Record verification
        </button>
      </form>
    </Section>
  );
}

// ---------------------------------------------------------------------------

export default function CompanyDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const api = useApi();
  const qc = useQueryClient();
  const q = useQuery({ queryKey: ["admin", "company", id], queryFn: () => api.get<AdminCompany>(`/admin/companies/${id}`), enabled: !!id });
  const setCompany: Setter = (c) => qc.setQueryData(["admin", "company", id], c);

  if (q.isLoading) return <LoadingState />;
  if (q.error) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  const c = q.data;
  if (!c) return null;

  return (
    <div className="space-y-6">
      <p className="text-sm">
        <Link href="/admin/companies" className="link">
          ← All companies
        </Link>
      </p>
      <PageHeader
        title={c.display_name}
        description={
          <span className="font-mono text-xs">
            <span data-testid="company-slug">{c.slug}</span> · created {formatDate(c.created_at)} · updated {formatDateTime(c.updated_at)}
          </span>
        }
      />
      {/* key forces form state to reset when the record changes on the server */}
      <PublicationPanel company={c} />
      <CoreFields key={`core-${c.updated_at}`} company={c} />
      <Classification key={`class-${c.updated_at}`} company={c} />
      <Facilities company={c} onChange={setCompany} />
      <Products company={c} onChange={setCompany} />
      <OtherDetails company={c} />
      <ProvenanceTable company={c} />
      <Verification company={c} />
    </div>
  );
}
