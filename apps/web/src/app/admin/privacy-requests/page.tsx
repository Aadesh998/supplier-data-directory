"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDateTime, humanize, PRIVACY_REQUEST_TYPES } from "@/lib/labels";
import type { Paginated, PrivacyRequestRecord } from "@/lib/types";

const ACTIONS = [
  { value: "none", label: "No data action" },
  { value: "unpublish", label: "Unpublish the company" },
  { value: "withdraw_public", label: "Withdraw public-listing permission" },
  { value: "withdraw_commercial", label: "Withdraw commercial/API permission" },
  { value: "erase_company", label: "Erase the company record" },
];

const typeLabel = (t: string) => PRIVACY_REQUEST_TYPES.find((x) => x.value === t)?.label ?? humanize(t);

function ResolveForm({ req, onDone }: { req: PrivacyRequestRecord; onDone: () => void }) {
  const api = useApi();
  const toast = useToast();
  const [status, setStatus] = useState<"in_progress" | "resolved" | "rejected">("resolved");
  const [action, setAction] = useState("none");
  const [companyId, setCompanyId] = useState(req.company_id ?? "");
  const [notes, setNotes] = useState("");
  const m = useMutation({
    mutationFn: () => {
      const body: Record<string, unknown> = { status, action };
      if (companyId.trim()) body.company_id = companyId.trim();
      if (notes.trim()) body.resolution_notes = notes.trim();
      return api.post<PrivacyRequestRecord>(`/admin/privacy-requests/${req.id}/resolve`, body);
    },
    onSuccess: () => {
      toast.success("Request updated.");
      onDone();
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const needsCompany = action !== "none";
  return (
    <form
      className="mt-3 grid gap-3 rounded-md border border-slate-200 bg-slate-50 p-3 md:grid-cols-2"
      onSubmit={(e) => {
        e.preventDefault();
        if (needsCompany && !companyId.trim()) return toast.error("Link a company ID for this action.");
        if (action === "erase_company" && !window.confirm("Erase this company permanently? This cannot be undone.")) return;
        m.mutate();
      }}
    >
      <div>
        <label htmlFor={`pr-status-${req.id}`} className="label">
          New status
        </label>
        <select id={`pr-status-${req.id}`} className="input" value={status} onChange={(e) => setStatus(e.target.value as typeof status)}>
          <option value="in_progress">In progress</option>
          <option value="resolved">Resolved</option>
          <option value="rejected">Rejected</option>
        </select>
      </div>
      <div>
        <label htmlFor={`pr-action-${req.id}`} className="label">
          Action on data
        </label>
        <select id={`pr-action-${req.id}`} className="input" value={action} onChange={(e) => setAction(e.target.value)}>
          {ACTIONS.map((a) => (
            <option key={a.value} value={a.value}>
              {a.label}
            </option>
          ))}
        </select>
      </div>
      <div>
        <label htmlFor={`pr-company-${req.id}`} className="label">
          Company ID {needsCompany && <span className="text-red-700">*</span>}
        </label>
        <input id={`pr-company-${req.id}`} className="input font-mono" value={companyId} onChange={(e) => setCompanyId(e.target.value)} placeholder="UUID from the company page" />
      </div>
      <div>
        <label htmlFor={`pr-notes-${req.id}`} className="label">
          Resolution notes
        </label>
        <input id={`pr-notes-${req.id}`} className="input" value={notes} onChange={(e) => setNotes(e.target.value)} />
      </div>
      <div className="md:col-span-2">
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          {m.isPending ? "Saving…" : "Save"}
        </button>
      </div>
    </form>
  );
}

export default function PrivacyRequestsPage() {
  const api = useApi();
  const qc = useQueryClient();
  const [status, setStatus] = useState("received");
  const [page, setPage] = useState(1);
  const [open, setOpen] = useState<string | null>(null);
  const q = useQuery({
    queryKey: ["admin", "privacy-requests", status, page],
    queryFn: () => api.get<Paginated<PrivacyRequestRecord>>("/admin/privacy-requests", { status, page, limit: 25 }),
    placeholderData: keepPreviousData,
  });
  return (
    <div className="space-y-4">
      <PageHeader title="Privacy requests" description="Corrections, removals, permission withdrawals and access requests from the public form. Handle them promptly." />
      <div>
        <label htmlFor="prq-status" className="label">
          Status
        </label>
        <select
          id="prq-status"
          className="input max-w-xs"
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
        >
          <option value="">All</option>
          {["received", "in_progress", "resolved", "rejected"].map((s) => (
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
        <EmptyState>No requests with this status.</EmptyState>
      ) : (
        <>
          <ul className="space-y-3">
            {q.data.data.map((r) => (
              <li key={r.id} className="card">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="space-y-1 text-sm">
                    <p className="font-semibold">{typeLabel(r.request_type)}</p>
                    <p>
                      {r.requester} · <a className="link" href={`mailto:${r.email}`}>{r.email}</a>
                    </p>
                    {r.company_reference && <p className="text-slate-700">Business: {r.company_reference}</p>}
                    {r.details && <p className="whitespace-pre-line text-slate-700">{r.details}</p>}
                    <p className="text-xs text-slate-600">
                      Received {formatDateTime(r.received_at)}
                      {r.resolved_at && ` · resolved ${formatDateTime(r.resolved_at)}`}
                    </p>
                    {r.resolution_notes && <p className="text-xs text-slate-700">Notes: {r.resolution_notes}</p>}
                  </div>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={r.status} />
                    {(r.status === "received" || r.status === "in_progress") && (
                      <button type="button" className="btn-secondary btn-sm" aria-expanded={open === r.id} onClick={() => setOpen((x) => (x === r.id ? null : r.id))}>
                        {open === r.id ? "Close" : "Resolve"}
                      </button>
                    )}
                  </div>
                </div>
                {open === r.id && (
                  <ResolveForm
                    req={r}
                    onDone={() => {
                      setOpen(null);
                      void qc.invalidateQueries({ queryKey: ["admin", "privacy-requests"] });
                      void qc.invalidateQueries({ queryKey: ["admin", "stats"] });
                    }}
                  />
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
