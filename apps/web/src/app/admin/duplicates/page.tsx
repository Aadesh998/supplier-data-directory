"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDate, humanize } from "@/lib/labels";
import type { AdminCompanySummary, DuplicatePair, Paginated } from "@/lib/types";

function CompanyColumn({ c, label }: { c: AdminCompanySummary; label: string }) {
  return (
    <div className="space-y-1 rounded-md border border-slate-200 p-3 text-sm">
      <p className="text-xs font-semibold tracking-wide text-slate-500 uppercase">{label}</p>
      <Link href={`/admin/companies/${c.id}`} className="link font-semibold" target="_blank">
        {c.display_name}
      </Link>
      {c.legal_name && <p className="text-slate-700">{c.legal_name}</p>}
      <p className="text-slate-700">{[c.city, c.state_code].filter(Boolean).join(", ") || "No location"}</p>
      <p className="text-slate-700">{c.website_domain ?? "No website"}</p>
      <p className="flex flex-wrap gap-1">
        <StatusBadge status={c.publication_status} /> <StatusBadge status={c.verification_status} />
      </p>
      <p className="text-xs text-slate-600">{c.roles.map(humanize).join(", ")}</p>
      <p className="text-xs text-slate-600">{c.categories.map((x) => x.name).join(", ")}</p>
      <p className="text-xs text-slate-500">Created {formatDate(c.created_at)}</p>
    </div>
  );
}

export default function DuplicatesPage() {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const [status, setStatus] = useState("open");
  const [page, setPage] = useState(1);
  const q = useQuery({
    queryKey: ["admin", "duplicates", status, page],
    queryFn: () => api.get<Paginated<DuplicatePair>>("/admin/duplicates", { status, page, limit: 20 }),
    placeholderData: keepPreviousData,
  });
  const scan = useMutation({
    mutationFn: () => api.post<{ created: number }>("/admin/duplicates/scan"),
    onSuccess: (r) => {
      toast.success(`Scan complete: ${r.created} new candidate pair${r.created === 1 ? "" : "s"}.`);
      void qc.invalidateQueries({ queryKey: ["admin", "duplicates"] });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const resolve = useMutation({
    mutationFn: (v: { id: string; action: "merge" | "dismiss"; keep?: "a" | "b" }) =>
      api.post<DuplicatePair>(`/admin/duplicates/${v.id}/resolve`, { action: v.action, ...(v.keep ? { keep: v.keep } : {}) }),
    onSuccess: (p) => {
      toast.success(p.status === "merged" ? "Records merged. Provenance was preserved." : "Pair dismissed.");
      void qc.invalidateQueries({ queryKey: ["admin", "duplicates"] });
      void qc.invalidateQueries({ queryKey: ["admin", "companies"] });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  return (
    <div className="space-y-4">
      <PageHeader
        title="Duplicate review"
        description="Possible duplicates are never merged automatically. Compare the records and decide."
        actions={
          <button type="button" className="btn-primary" disabled={scan.isPending} onClick={() => scan.mutate()}>
            {scan.isPending ? "Scanning…" : "Scan for duplicates"}
          </button>
        }
      />
      <div>
        <label htmlFor="dup-status" className="label">
          Status
        </label>
        <select
          id="dup-status"
          className="input max-w-xs"
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
        >
          {["open", "merged", "dismissed"].map((s) => (
            <option key={s} value={s}>
              {humanize(s)}
            </option>
          ))}
        </select>
      </div>
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No {status} duplicate pairs.</EmptyState>
      ) : (
        <>
          <ul className="space-y-4">
            {q.data.data.map((p) => (
              <li key={p.id} className="card space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                  <p>
                    Similarity <strong>{Math.round(p.score * 100)}%</strong>
                    {p.reasons.length > 0 && <span className="text-slate-600"> — {p.reasons.join(", ")}</span>}
                  </p>
                  <StatusBadge status={p.status} />
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  <CompanyColumn c={p.company_a} label="Record A" />
                  <CompanyColumn c={p.company_b} label="Record B" />
                </div>
                {p.status === "open" && (
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      className="btn-secondary"
                      disabled={resolve.isPending}
                      onClick={() => window.confirm(`Merge B into A and keep "${p.company_a.display_name}"?`) && resolve.mutate({ id: p.id, action: "merge", keep: "a" })}
                    >
                      Merge — keep A
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      disabled={resolve.isPending}
                      onClick={() => window.confirm(`Merge A into B and keep "${p.company_b.display_name}"?`) && resolve.mutate({ id: p.id, action: "merge", keep: "b" })}
                    >
                      Merge — keep B
                    </button>
                    <button type="button" className="btn-ghost" disabled={resolve.isPending} onClick={() => resolve.mutate({ id: p.id, action: "dismiss" })}>
                      Not duplicates — dismiss
                    </button>
                  </div>
                )}
              </li>
            ))}
          </ul>
          <AdminPagination page={q.data.page} onPage={setPage} />
        </>
      )}
    </div>
  );
}
