"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { emptySource, SourceFields, sourceInputFromForm, type SourceFormValues } from "@/components/admin/SourceFields";
import { AdminPagination, BoolBadge, EmptyState, ErrorState, LoadingState, PageHeader } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDate, humanize } from "@/lib/labels";
import type { DataSource, Paginated } from "@/lib/types";

function toForm(s: DataSource): SourceFormValues {
  return {
    source_type: s.source_type,
    provider: s.provider,
    source_url: s.source_url ?? "",
    license_reference: s.license_reference ?? "",
    notes: s.notes ?? "",
    retention_expires_at: s.retention_expires_at ? s.retention_expires_at.slice(0, 10) : "",
    allowed_internal_use: s.allowed_internal_use,
    allowed_public_display: s.allowed_public_display,
    allowed_commercial_api: s.allowed_commercial_api,
  };
}

export default function SourcesPage() {
  const api = useApi();
  const { user } = useAuth();
  const toast = useToast();
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [editing, setEditing] = useState<{ id?: string; original?: DataSource; values: SourceFormValues } | null>(null);
  const q = useQuery({
    queryKey: ["admin", "sources", page],
    queryFn: () => api.get<Paginated<DataSource>>("/admin/sources", { page, limit: 25 }),
    placeholderData: keepPreviousData,
  });
  const save = useMutation({
    mutationFn: (e: NonNullable<typeof editing>) =>
      e.id ? api.patch<DataSource>(`/admin/sources/${e.id}`, sourceInputFromForm(e.values)) : api.post<DataSource>("/admin/sources", sourceInputFromForm(e.values)),
    onSuccess: (s) => {
      toast.success(`Saved source “${s.provider}”.`);
      setEditing(null);
      void qc.invalidateQueries({ queryKey: ["admin", "sources"] });
      void qc.invalidateQueries({ queryKey: ["admin", "companies"] });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  const revokingPublic = !!editing?.original?.allowed_public_display && !editing.values.allowed_public_display;

  return (
    <div className="space-y-4">
      <PageHeader
        title="Data sources"
        description="Every field in the directory is linked to a source. Permissions default to off and should only be enabled with documented evidence."
        actions={
          <button type="button" className="btn-primary" onClick={() => setEditing({ values: emptySource() })}>
            New source
          </button>
        }
      />
      {user?.role !== "owner" && (
        <p className="rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-700">Only owners can change permission flags on existing sources; the API will refuse otherwise.</p>
      )}
      {editing && (
        <form
          className="card space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            if (!editing.values.provider.trim()) return toast.error("Provider is required.");
            if (revokingPublic && !window.confirm("Turning off public display will unpublish every company that depends on this source. Continue?")) return;
            save.mutate(editing);
          }}
        >
          <h2 className="text-lg font-semibold">{editing.id ? `Edit “${editing.original?.provider}”` : "New source"}</h2>
          <SourceFields idPrefix="src" value={editing.values} onChange={(values) => setEditing((x) => (x ? { ...x, values } : x))} />
          {revokingPublic && (
            <p role="alert" className="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-900">
              Warning: turning off public display unpublishes affected companies immediately.
            </p>
          )}
          <div className="flex gap-2">
            <button type="submit" className="btn-primary" disabled={save.isPending}>
              {save.isPending ? "Saving…" : "Save source"}
            </button>
            <button type="button" className="btn-ghost" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </div>
        </form>
      )}
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No sources yet.</EmptyState>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">Provider</th>
                  <th scope="col">Type</th>
                  <th scope="col">Licence ref.</th>
                  <th scope="col">Internal</th>
                  <th scope="col">Public</th>
                  <th scope="col">Commercial</th>
                  <th scope="col">Retention</th>
                  <th scope="col">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {q.data.data.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <span className="font-medium">{s.provider}</span>
                      {s.source_url && <div className="max-w-xs truncate text-xs text-slate-600">{s.source_url}</div>}
                    </td>
                    <td className="text-xs">{humanize(s.source_type)}</td>
                    <td className="text-xs">{s.license_reference ?? "—"}</td>
                    <td>
                      <BoolBadge value={s.allowed_internal_use} />
                    </td>
                    <td>
                      <BoolBadge value={s.allowed_public_display} />
                    </td>
                    <td>
                      <BoolBadge value={s.allowed_commercial_api} />
                    </td>
                    <td className="text-xs">{formatDate(s.retention_expires_at)}</td>
                    <td>
                      <button type="button" className="btn-ghost btn-sm" onClick={() => setEditing({ id: s.id, original: s, values: toForm(s) })}>
                        Edit<span className="sr-only"> {s.provider}</span>
                      </button>
                    </td>
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
