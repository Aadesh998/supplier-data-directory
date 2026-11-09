"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { useStates } from "@/lib/admin/hooks";
import { formatDateTime } from "@/lib/labels";
import type { GmapsLead, Paginated } from "@/lib/types";

function hostOf(url: string): string {
  try {
    return new URL(url).host.replace(/^www\./, "");
  } catch {
    return url;
  }
}

function LeadsPage() {
  const api = useApi();
  const { user } = useAuth();
  const toast = useToast();
  const qc = useQueryClient();
  const router = useRouter();
  const params = useSearchParams();
  const canPromote = user?.role === "owner" || user?.role === "admin";
  const job = params.get("job") ?? "";
  const states = useStates();

  const [status, setStatus] = useState(job ? "" : "new");
  const [state, setState] = useState("");
  const [q, setQ] = useState("");
  const [hasPhone, setHasPhone] = useState(false);
  const [hasWebsite, setHasWebsite] = useState(false);
  const [page, setPage] = useState(1);
  const reset = () => setPage(1);

  const leads = useQuery({
    queryKey: ["admin", "gmaps", "leads", status, state, q, hasPhone, hasWebsite, job, page],
    queryFn: () =>
      api.get<Paginated<GmapsLead>>("/admin/gmaps/leads", {
        status,
        state,
        q,
        job,
        has_phone: hasPhone ? "true" : "",
        has_website: hasWebsite ? "true" : "",
        page,
        limit: 50,
      }),
    placeholderData: keepPreviousData,
  });
  const invalidate = () => void qc.invalidateQueries({ queryKey: ["admin", "gmaps"] });
  const setLeadStatus = useMutation({
    mutationFn: ({ id, s }: { id: string; s: string }) => api.patch<GmapsLead>(`/admin/gmaps/leads/${id}`, { status: s }),
    onSuccess: invalidate,
    onError: (e) => toast.error(errorMessage(e)),
  });
  const promote = useMutation({
    mutationFn: (id: string) => api.post<{ company_id: string }>(`/admin/gmaps/leads/${id}/promote`),
    onSuccess: (r) => {
      toast.success("Private draft company created. Review it, then approve and publish.");
      invalidate();
      router.push(`/admin/companies/${r.company_id}`);
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="Google Maps leads"
        description="Businesses found by the Google Maps worker. Nothing here is public. Promote a lead to create a private draft company you can review and publish."
      />
      {job && (
        <p className="flex flex-wrap items-center gap-2 text-sm">
          Showing leads from job <span className="font-mono text-xs">{job}</span>
          <Link href="/admin/discovery/leads" className="link text-xs">
            Show all
          </Link>
        </p>
      )}
      <section className="card space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="min-w-56 flex-1">
            <label htmlFor="l-q" className="label">
              Search
            </label>
            <input id="l-q" className="input" placeholder="Name, address or phone" value={q} onChange={(e) => { setQ(e.target.value); reset(); }} />
          </div>
          <div>
            <label htmlFor="l-status" className="label">
              Status
            </label>
            <select id="l-status" className="input" value={status} onChange={(e) => { setStatus(e.target.value); reset(); }}>
              <option value="">All</option>
              <option value="new">New</option>
              <option value="contacted">Contacted</option>
              <option value="promoted">Promoted</option>
              <option value="dismissed">Dismissed</option>
            </select>
          </div>
          <div>
            <label htmlFor="l-state" className="label">
              State
            </label>
            <select id="l-state" className="input" value={state} onChange={(e) => { setState(e.target.value); reset(); }}>
              <option value="">All</option>
              {states.data?.data.map((s) => (
                <option key={s.code} value={s.code}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
          <label className="flex h-9 items-center gap-2 text-sm">
            <input type="checkbox" checked={hasPhone} onChange={(e) => { setHasPhone(e.target.checked); reset(); }} />
            Has phone
          </label>
          <label className="flex h-9 items-center gap-2 text-sm">
            <input type="checkbox" checked={hasWebsite} onChange={(e) => { setHasWebsite(e.target.checked); reset(); }} />
            Has website
          </label>
        </div>

        {leads.isLoading ? (
          <LoadingState />
        ) : leads.error ? (
          <ErrorState error={leads.error} onRetry={() => void leads.refetch()} />
        ) : !leads.data?.data.length ? (
          <EmptyState>No leads for these filters. The worker adds leads as it finishes jobs.</EmptyState>
        ) : (
          <>
            <p className="text-sm text-muted-foreground">{leads.data.page.total.toLocaleString("en-IN")} leads</p>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th scope="col">Business</th>
                    <th scope="col">Contact</th>
                    <th scope="col">Area</th>
                    <th scope="col">Rating</th>
                    <th scope="col">Status</th>
                    <th scope="col">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {leads.data.data.map((l) => (
                    <tr key={l.id}>
                      <td className="max-w-sm">
                        <p className="text-sm font-medium">{l.title}</p>
                        <p className="text-xs text-muted-foreground">
                          {l.category ?? "—"}
                          {l.maps_link && (
                            <>
                              {" · "}
                              <a href={l.maps_link} target="_blank" rel="noopener noreferrer" className="link">
                                map
                              </a>
                            </>
                          )}
                        </p>
                        {l.address && <p className="truncate text-xs text-muted-foreground" title={l.address}>{l.address}</p>}
                      </td>
                      <td className="text-xs">
                        {l.phone && (
                          <a href={`tel:${l.phone.replace(/\s+/g, "")}`} className="link block whitespace-nowrap">
                            {l.phone}
                          </a>
                        )}
                        {l.website && (
                          <a href={l.website} target="_blank" rel="noopener noreferrer" className="link block max-w-48 truncate">
                            {hostOf(l.website)}
                          </a>
                        )}
                        {!l.phone && !l.website && "—"}
                      </td>
                      <td className="text-xs">
                        {l.location ?? "—"}
                        {l.state_code && `, ${l.state_code}`}
                        {l.search_query && <p className="text-muted-foreground" title="Search that found it">{l.search_query}</p>}
                      </td>
                      <td className="whitespace-nowrap text-xs tabular-nums">
                        {l.review_rating != null ? `★ ${l.review_rating.toFixed(1)}` : "—"}
                        {l.review_count != null && <span className="text-muted-foreground"> ({l.review_count})</span>}
                      </td>
                      <td className="whitespace-nowrap">
                        <StatusBadge status={l.status} />
                        {l.promoted_company_id && (
                          <Link href={`/admin/companies/${l.promoted_company_id}`} className="link ml-1 text-xs">
                            company
                          </Link>
                        )}
                        <p className="text-xs text-muted-foreground" title={`Last seen ${formatDateTime(l.last_seen_at)}`}>
                          seen {l.seen_count}×
                        </p>
                      </td>
                      <td className="whitespace-nowrap">
                        {l.status !== "promoted" && (
                          <div className="flex justify-end gap-1">
                            {canPromote && (
                              <button type="button" className="btn-secondary btn-sm" disabled={promote.isPending} onClick={() => promote.mutate(l.id)}>
                                Promote
                              </button>
                            )}
                            {l.status !== "contacted" && (
                              <button type="button" className="btn-ghost btn-sm" onClick={() => setLeadStatus.mutate({ id: l.id, s: "contacted" })}>
                                Contacted
                              </button>
                            )}
                            {l.status !== "dismissed" && (
                              <button type="button" className="btn-ghost btn-sm" onClick={() => setLeadStatus.mutate({ id: l.id, s: "dismissed" })}>
                                Dismiss
                              </button>
                            )}
                          </div>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <AdminPagination page={leads.data.page} onPage={setPage} />
          </>
        )}
      </section>
    </div>
  );
}

export default function GmapsLeadsPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <LeadsPage />
    </Suspense>
  );
}
