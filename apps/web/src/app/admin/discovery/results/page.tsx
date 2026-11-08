"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDateTime } from "@/lib/labels";
import type { DiscoveryJobResult, Paginated } from "@/lib/types";

function osmUrl(ref: string): string | null {
  const m = /^(node|way|relation)\/(\d+)$/.exec(ref);
  return m ? `https://www.openstreetmap.org/${m[1]}/${m[2]}` : null;
}

export default function DiscoveryJobResultsPage() {
  const api = useApi();
  const { user } = useAuth();
  const toast = useToast();
  const qc = useQueryClient();
  const canPromote = user?.role === "owner" || user?.role === "admin";
  const [status, setStatus] = useState("new");
  const [provider, setProvider] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  const results = useQuery({
    queryKey: ["admin", "discovery", "results", status, provider, q, page],
    queryFn: () => api.get<Paginated<DiscoveryJobResult>>("/admin/discovery/results", { status, provider, q, page, limit: 50 }),
    placeholderData: keepPreviousData,
  });
  const invalidate = () => void qc.invalidateQueries({ queryKey: ["admin", "discovery"] });
  const setResultStatus = useMutation({
    mutationFn: ({ id, s }: { id: string; s: string }) => api.patch<DiscoveryJobResult>(`/admin/discovery/results/${id}`, { status: s }),
    onSuccess: invalidate,
    onError: (e) => toast.error(errorMessage(e)),
  });
  const promote = useMutation({
    mutationFn: (id: string) => api.post<{ company_id: string }>(`/admin/discovery/results/${id}/promote`),
    onSuccess: () => {
      toast.success("Draft company created (private, internal-only source). Check Duplicates.");
      invalidate();
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="Discovery results"
        description="Leads found by discovery jobs. Nothing here is public. Promote an OpenStreetMap result to a private draft company, or mark a lead contacted after inviting the business to register."
      />
      <p className="rounded-md border border-slate-200 bg-slate-50 px-3 py-2 text-sm text-slate-800">
        OpenStreetMap data © OpenStreetMap contributors, ODbL. Promoted records use an internal-only source until you review the
        licence terms. Google results store only the place ID, so they cannot be promoted. Contact the business and ask it to
        register.
      </p>
      <section className="card space-y-3">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <label htmlFor="r-status" className="label">
              Status
            </label>
            <select id="r-status" className="input" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
              <option value="">All</option>
              <option value="new">New</option>
              <option value="contacted">Contacted</option>
              <option value="promoted">Promoted</option>
              <option value="dismissed">Dismissed</option>
            </select>
          </div>
          <div>
            <label htmlFor="r-provider" className="label">
              Provider
            </label>
            <select id="r-provider" className="input" value={provider} onChange={(e) => { setProvider(e.target.value); setPage(1); }}>
              <option value="">All</option>
              <option value="osm_overpass">OpenStreetMap</option>
              <option value="google_places_ids">Google place IDs</option>
            </select>
          </div>
          <div>
            <label htmlFor="r-q" className="label">
              Name
            </label>
            <input id="r-q" className="input" value={q} onChange={(e) => { setQ(e.target.value); setPage(1); }} />
          </div>
        </div>
        {results.isLoading ? (
          <LoadingState />
        ) : results.error ? (
          <ErrorState error={results.error} onRetry={() => void results.refetch()} />
        ) : !results.data?.data.length ? (
          <EmptyState>No results for these filters.</EmptyState>
        ) : (
          <>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th scope="col">Lead</th>
                    <th scope="col">Category</th>
                    <th scope="col">Area</th>
                    <th scope="col">Status</th>
                    <th scope="col">Seen</th>
                    <th scope="col"><span className="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody>
                  {results.data.data.map((r) => {
                    const link = r.provider === "osm_overpass" ? osmUrl(r.provider_ref) : null;
                    return (
                      <tr key={r.id}>
                        <td>
                          <p className="text-sm font-medium">{r.name ?? <span className="font-mono text-xs">{r.provider_ref}</span>}</p>
                          <p className="text-xs text-slate-500">
                            {r.provider === "osm_overpass" ? "OpenStreetMap" : "Google place ID"}
                            {link && (
                              <>
                                {" · "}
                                <a href={link} target="_blank" rel="noopener noreferrer" className="link">
                                  view
                                </a>
                              </>
                            )}
                            {r.tags.website && <> · {r.tags.website}</>}
                          </p>
                        </td>
                        <td className="text-xs">{r.category_slug ?? "—"}</td>
                        <td className="text-xs">
                          {r.tags["addr:city"] ?? r.job_location ?? "—"}
                          {r.job_state_code && `, ${r.job_state_code}`}
                        </td>
                        <td>
                          <StatusBadge status={r.status} />
                          {r.matched_company_id && (
                            <Link href={`/admin/companies/${r.matched_company_id}`} className="link ml-1 text-xs">
                              company
                            </Link>
                          )}
                        </td>
                        <td className="whitespace-nowrap text-xs">
                          {r.seen_count}× · {formatDateTime(r.last_seen_at)}
                        </td>
                        <td className="whitespace-nowrap">
                          {r.status !== "promoted" && (
                            <div className="flex gap-1">
                              {canPromote && r.provider === "osm_overpass" && r.name && (
                                <button type="button" className="btn-secondary btn-sm" disabled={promote.isPending} onClick={() => promote.mutate(r.id)}>
                                  Promote
                                </button>
                              )}
                              {r.status !== "contacted" && (
                                <button type="button" className="btn-ghost btn-sm" onClick={() => setResultStatus.mutate({ id: r.id, s: "contacted" })}>
                                  Contacted
                                </button>
                              )}
                              {r.status !== "dismissed" && (
                                <button type="button" className="btn-ghost btn-sm" onClick={() => setResultStatus.mutate({ id: r.id, s: "dismissed" })}>
                                  Dismiss
                                </button>
                              )}
                            </div>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            <AdminPagination page={results.data.page} onPage={setPage} />
          </>
        )}
      </section>
    </div>
  );
}
