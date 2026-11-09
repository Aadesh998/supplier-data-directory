"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState, type FormEvent } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { SourceSelect } from "@/components/admin/SourceSelect";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { useStates } from "@/lib/admin/hooks";
import { formatDate } from "@/lib/labels";
import type { AdminCompany, AdminCompanySummary, Paginated } from "@/lib/types";

const PUBLICATION = ["draft", "submitted", "under_review", "approved_private", "published", "rejected", "needs_changes", "archived"];
const VERIFICATION = ["unverified", "partially_verified", "verified", "stale"];

type Filters = { q: string; publication_status: string; verification_status: string; category: string; state: string };

type BulkPublishResult = { published: number; remaining: number; blocked: Record<string, number> };

/** Publishes every approved company that passes the publish checks, and explains the rest. */
function PublishEligible() {
  const api = useApi();
  const qc = useQueryClient();
  const toast = useToast();
  const [confirming, setConfirming] = useState(false);
  const [result, setResult] = useState<BulkPublishResult | null>(null);
  const m = useMutation({
    mutationFn: () => api.post<BulkPublishResult>("/admin/companies/publish-eligible", {}),
    onSuccess: (r) => {
      setResult(r);
      setConfirming(false);
      toast.success(r.published === 1 ? "Published 1 company." : `Published ${r.published} companies.`);
      void qc.invalidateQueries();
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  return (
    <div className="flex flex-col items-end gap-2">
      {confirming ? (
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Publish every approved company that passes the checks?</span>
          <button type="button" className="btn-primary" disabled={m.isPending} onClick={() => m.mutate()}>
            {m.isPending ? "Publishing…" : "Yes, publish"}
          </button>
          <button type="button" className="btn-ghost" disabled={m.isPending} onClick={() => setConfirming(false)}>
            Cancel
          </button>
        </div>
      ) : (
        <button type="button" className="btn-primary" onClick={() => setConfirming(true)}>
          Publish all eligible
        </button>
      )}
      {result && result.remaining > 0 && (
        <div role="status" className="max-w-md rounded-md border border-amber-500/25 bg-amber-500/10 p-3 text-sm">
          <p className="font-medium">{result.remaining} approved companies are still private:</p>
          <ul className="mt-1 list-disc pl-5 text-muted-foreground">
            {Object.entries(result.blocked)
              .sort((a, b) => b[1] - a[1])
              .map(([reason, n]) => (
                <li key={reason}>
                  {n} — {reason}
                </li>
              ))}
          </ul>
          <p className="mt-1 text-xs text-muted-foreground">Fix them on each company page, then run this again.</p>
        </div>
      )}
    </div>
  );
}

function CompanyActions() {
  const { user } = useAuth();
  const canPublish = user?.role === "owner" || user?.role === "admin";
  return (
    <div className="flex flex-wrap items-start justify-end gap-2">
      <CreateCompany />
      {canPublish && <PublishEligible />}
    </div>
  );
}

function CreateCompany() {
  const api = useApi();
  const router = useRouter();
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [sourceId, setSourceId] = useState("");
  const m = useMutation({
    mutationFn: () => api.post<AdminCompany>("/admin/companies", { display_name: name.trim(), source_id: sourceId }),
    onSuccess: (c) => {
      toast.success("Company created as a private draft.");
      router.push(`/admin/companies/${c.id}`);
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  if (!open)
    return (
      <button type="button" className="btn-secondary" onClick={() => setOpen(true)}>
        Add company manually
      </button>
    );
  return (
    <form
      className="card w-full space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (name.trim().length < 2 || !sourceId) return toast.error("Enter a name and choose a source.");
        m.mutate();
      }}
    >
      <h2 className="font-semibold">Add a company</h2>
      <div className="grid gap-3 md:grid-cols-2">
        <div>
          <label htmlFor="new-name" className="label">
            Display name <span className="text-red-700">*</span>
          </label>
          <input id="new-name" className="input" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <SourceSelect id="new-source" value={sourceId} onChange={setSourceId} />
      </div>
      <div className="flex gap-2">
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          Create
        </button>
        <button type="button" className="btn-ghost" onClick={() => setOpen(false)}>
          Cancel
        </button>
      </div>
    </form>
  );
}

function CompaniesInner() {
  const api = useApi();
  const sp = useSearchParams();
  const states = useStates();
  const initial: Filters = {
    q: sp.get("q") ?? "",
    publication_status: sp.get("publication_status") ?? "",
    verification_status: sp.get("verification_status") ?? "",
    category: sp.get("category") ?? "",
    state: sp.get("state") ?? "",
  };
  const [draft, setDraft] = useState<Filters>(initial);
  const [filters, setFilters] = useState<Filters>(initial);
  const [page, setPage] = useState(1);

  const q = useQuery({
    queryKey: ["admin", "companies", filters, page],
    queryFn: () => api.get<Paginated<AdminCompanySummary>>("/admin/companies", { ...filters, page, limit: 25 }),
    placeholderData: keepPreviousData,
  });

  function apply(e: FormEvent) {
    e.preventDefault();
    setFilters(draft);
    setPage(1);
  }

  const set = (k: keyof Filters) => (e: { target: { value: string } }) => setDraft((d) => ({ ...d, [k]: e.target.value }));

  return (
    <div className="space-y-4">
      <PageHeader title="Companies" description="All company records, including private ones. Only published records with public-display permission appear on the public site." actions={<CompanyActions />} />
      <form onSubmit={apply} className="card grid gap-3 md:grid-cols-6" role="search">
        <div className="md:col-span-2">
          <label htmlFor="cf-q" className="label">
            Search
          </label>
          <input id="cf-q" className="input" value={draft.q} onChange={set("q")} placeholder="Name, legal name or domain" />
        </div>
        <div>
          <label htmlFor="cf-pub" className="label">
            Publication
          </label>
          <select id="cf-pub" className="input" value={draft.publication_status} onChange={set("publication_status")}>
            <option value="">Any</option>
            {PUBLICATION.map((s) => (
              <option key={s} value={s}>
                {s.replace(/_/g, " ")}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="cf-ver" className="label">
            Verification
          </label>
          <select id="cf-ver" className="input" value={draft.verification_status} onChange={set("verification_status")}>
            <option value="">Any</option>
            {VERIFICATION.map((s) => (
              <option key={s} value={s}>
                {s.replace(/_/g, " ")}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="cf-state" className="label">
            State
          </label>
          <select id="cf-state" className="input" value={draft.state} onChange={set("state")}>
            <option value="">Any</option>
            {states.data?.data.map((s) => (
              <option key={s.code} value={s.code}>
                {s.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="cf-cat" className="label">
            Category slug
          </label>
          <input id="cf-cat" className="input" value={draft.category} onChange={set("category")} placeholder="e.g. namkeen" />
        </div>
        <div className="flex items-end md:col-span-6 md:justify-end">
          <button type="submit" className="btn-primary">
            Apply filters
          </button>
        </div>
      </form>

      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No companies match these filters.</EmptyState>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">Name</th>
                  <th scope="col">Location</th>
                  <th scope="col">Publication</th>
                  <th scope="col">Verification</th>
                  <th scope="col">Categories</th>
                  <th scope="col">Updated</th>
                </tr>
              </thead>
              <tbody>
                {q.data.data.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <Link href={`/admin/companies/${c.id}`} className="link font-medium">
                        {c.display_name}
                      </Link>
                      {c.website_domain && <div className="text-xs text-slate-600">{c.website_domain}</div>}
                    </td>
                    <td>{[c.city, c.state_code].filter(Boolean).join(", ") || "—"}</td>
                    <td>
                      <StatusBadge status={c.publication_status} />
                    </td>
                    <td>
                      <StatusBadge status={c.verification_status} />
                    </td>
                    <td className="text-xs">{c.categories.map((x) => x.name).join(", ") || "—"}</td>
                    <td className="whitespace-nowrap text-xs">{formatDate(c.updated_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <AdminPagination page={q.data.page} onPage={setPage} />
        </>
      )}
    </div>
  );
}

export default function CompaniesPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <CompaniesInner />
    </Suspense>
  );
}
