"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDateTime } from "@/lib/labels";
import type { DiscoveryLead, DiscoverySearchResponse, DiscoveryStatus, Paginated } from "@/lib/types";

function Leads() {
  const api = useApi();
  const q = useQuery({ queryKey: ["admin", "discovery", "leads"], queryFn: () => api.get<Paginated<DiscoveryLead>>("/admin/discovery/leads") });
  return (
    <section className="card space-y-3">
      <h2 className="text-lg font-semibold">Saved leads</h2>
      <p className="text-sm text-slate-700">Only the provider place ID is stored. Use it to invite the business to register itself; nothing is imported.</p>
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} />
      ) : !q.data?.data.length ? (
        <EmptyState>No leads saved.</EmptyState>
      ) : (
        <div className="table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th scope="col">Place ID</th>
                <th scope="col">Query</th>
                <th scope="col">Status</th>
                <th scope="col">Notes</th>
                <th scope="col">Saved</th>
              </tr>
            </thead>
            <tbody>
              {q.data.data.map((l) => (
                <tr key={l.id}>
                  <td className="font-mono text-xs">{l.provider_place_id}</td>
                  <td className="text-xs">{l.search_query ?? "—"}</td>
                  <td>
                    <StatusBadge status={l.status} />
                  </td>
                  <td className="text-xs">{l.notes ?? ""}</td>
                  <td className="whitespace-nowrap text-xs">{formatDateTime(l.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function SearchPanel() {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const [query, setQuery] = useState("");
  const [location, setLocation] = useState("");
  const [lastQuery, setLastQuery] = useState("");
  const search = useMutation({
    mutationFn: () => api.post<DiscoverySearchResponse>("/admin/discovery/search", { query: query.trim(), ...(location.trim() ? { location: location.trim() } : {}) }),
    onError: (e) => toast.error(errorMessage(e)),
  });
  const save = useMutation({
    mutationFn: (placeId: string) => api.post<DiscoveryLead>("/admin/discovery/leads", { provider_place_id: placeId, search_query: lastQuery }),
    onSuccess: () => {
      toast.success("Place ID saved as a lead.");
      void qc.invalidateQueries({ queryKey: ["admin", "discovery", "leads"] });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!query.trim()) return;
    setLastQuery(query.trim());
    search.mutate();
  }

  const data = search.data;
  return (
    <section className="card space-y-4">
      <form onSubmit={onSubmit} className="grid gap-3 md:grid-cols-[2fr_1fr_auto]" role="search">
        <div>
          <label htmlFor="d-q" className="label">
            What to look for
          </label>
          <input id="d-q" className="input" value={query} onChange={(e) => setQuery(e.target.value)} placeholder="e.g. vitrified tile manufacturer" />
        </div>
        <div>
          <label htmlFor="d-loc" className="label">
            Location
          </label>
          <input id="d-loc" className="input" value={location} onChange={(e) => setLocation(e.target.value)} placeholder="e.g. Morbi, Gujarat" />
        </div>
        <div className="flex items-end">
          <button type="submit" className="btn-primary" disabled={search.isPending}>
            {search.isPending ? "Searching…" : "Search"}
          </button>
        </div>
      </form>
      <p className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-950">
        Results are shown for review only and are <strong>not stored</strong>. Only a place ID you explicitly save is kept, as a lead to invite.
      </p>
      {data && (
        <div className="space-y-3">
          {data.notice && <p className="text-sm text-slate-700">{data.notice}</p>}
          {data.results.length === 0 ? (
            <EmptyState>No results.</EmptyState>
          ) : (
            <ul className="divide-y divide-slate-100">
              {data.results.map((r) => (
                <li key={r.provider_place_id} className="flex flex-wrap items-start justify-between gap-3 py-3 text-sm">
                  <div>
                    <p className="font-medium">{r.display_name}</p>
                    {r.formatted_address && <p className="text-slate-700">{r.formatted_address}</p>}
                    {r.google_maps_uri && (
                      <a href={r.google_maps_uri} className="link text-xs" target="_blank" rel="noopener noreferrer">
                        View on Google Maps
                      </a>
                    )}
                  </div>
                  <button type="button" className="btn-secondary btn-sm" disabled={save.isPending} onClick={() => save.mutate(r.provider_place_id)}>
                    Save place ID as lead
                  </button>
                </li>
              ))}
            </ul>
          )}
          {data.attribution && <p className="text-xs text-slate-700">{data.attribution}</p>}
        </div>
      )}
    </section>
  );
}

export default function DiscoveryPage() {
  const api = useApi();
  const status = useQuery({ queryKey: ["admin", "discovery", "status"], queryFn: () => api.get<DiscoveryStatus>("/admin/discovery/status") });
  return (
    <div className="space-y-4">
      <PageHeader title="Discovery" description="Optional, terms-compliant lookup to find businesses you might invite to register. It is not a data source for the directory." />
      {status.isLoading ? (
        <LoadingState />
      ) : status.error ? (
        <ErrorState error={status.error} onRetry={() => status.refetch()} />
      ) : !status.data?.enabled ? (
        <div className="card space-y-2">
          <p className="font-semibold">Discovery is disabled{status.data?.provider ? ` (${status.data.provider})` : ""}.</p>
          <p className="text-sm text-slate-700">{status.data?.reason ?? "No reason given."}</p>
        </div>
      ) : (
        <>
          <p className="text-sm text-slate-700">Provider: {status.data.provider}</p>
          <SearchPanel />
        </>
      )}
      <Leads />
    </div>
  );
}
