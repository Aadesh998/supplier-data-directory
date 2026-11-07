"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Fragment, useState, type FormEvent } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, JsonBlock, LoadingState, PageHeader } from "@/components/admin/ui";
import { formatDateTime } from "@/lib/labels";
import type { AuditLog, Paginated } from "@/lib/types";

type Filters = { entity_type: string; entity_id: string; action: string };

export default function AuditPage() {
  const api = useApi();
  const [draft, setDraft] = useState<Filters>({ entity_type: "", entity_id: "", action: "" });
  const [filters, setFilters] = useState<Filters>(draft);
  const [page, setPage] = useState(1);
  const [open, setOpen] = useState<string | null>(null);
  const q = useQuery({
    queryKey: ["admin", "audit", filters, page],
    queryFn: () => api.get<Paginated<AuditLog>>("/admin/audit-logs", { ...filters, page, limit: 50 }),
    placeholderData: keepPreviousData,
  });
  const set = (k: keyof Filters) => (e: { target: { value: string } }) => setDraft((d) => ({ ...d, [k]: e.target.value }));
  function apply(e: FormEvent) {
    e.preventDefault();
    setFilters(draft);
    setPage(1);
  }
  return (
    <div className="space-y-4">
      <PageHeader title="Audit log" description="Every administrative change, approval, export and access to personal details is recorded here." />
      <form onSubmit={apply} className="card grid gap-3 md:grid-cols-4" role="search">
        <div>
          <label htmlFor="al-type" className="label">
            Entity type
          </label>
          <input id="al-type" className="input" value={draft.entity_type} onChange={set("entity_type")} placeholder="e.g. company" />
        </div>
        <div>
          <label htmlFor="al-id" className="label">
            Entity ID
          </label>
          <input id="al-id" className="input font-mono" value={draft.entity_id} onChange={set("entity_id")} />
        </div>
        <div>
          <label htmlFor="al-action" className="label">
            Action
          </label>
          <input id="al-action" className="input" value={draft.action} onChange={set("action")} placeholder="e.g. company.publish" />
        </div>
        <div className="flex items-end">
          <button type="submit" className="btn-primary">
            Filter
          </button>
        </div>
      </form>
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No audit entries match.</EmptyState>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">When</th>
                  <th scope="col">Actor</th>
                  <th scope="col">Action</th>
                  <th scope="col">Entity</th>
                  <th scope="col">Request</th>
                  <th scope="col">
                    <span className="sr-only">Details</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {q.data.data.map((a) => (
                  <Fragment key={a.id}>
                    <tr>
                      <td className="whitespace-nowrap text-xs">{formatDateTime(a.created_at)}</td>
                      <td className="text-xs">{a.actor_email ?? a.actor_type}</td>
                      <td className="font-mono text-xs">{a.action}</td>
                      <td className="text-xs">
                        {a.entity_type}
                        {a.entity_id && <div className="font-mono text-[11px] text-slate-500">{a.entity_id}</div>}
                      </td>
                      <td className="font-mono text-[11px] text-slate-500">{a.request_id ?? ""}</td>
                      <td>
                        {(a.before || a.after) && (
                          <button type="button" className="btn-ghost btn-sm" aria-expanded={open === a.id} onClick={() => setOpen((x) => (x === a.id ? null : a.id))}>
                            {open === a.id ? "Hide" : "Diff"}
                          </button>
                        )}
                      </td>
                    </tr>
                    {open === a.id && (
                      <tr>
                        <td colSpan={6}>
                          <div className="grid gap-3 md:grid-cols-2">
                            <div>
                              <p className="mb-1 text-xs font-semibold">Before</p>
                              <JsonBlock value={a.before} />
                            </div>
                            <div>
                              <p className="mb-1 text-xs font-semibold">After</p>
                              <JsonBlock value={a.after} />
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
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
