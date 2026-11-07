"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { facilityLabel, formatDateTime, humanize } from "@/lib/labels";
import type { Submission, SubmissionDetail } from "@/lib/types";
import { useApi } from "./AuthProvider";
import { JsonBlock, StatusBadge } from "./ui";

type RejectStatus = "rejected" | "needs_changes" | "spam";

function ConsentRow({ label, at, required }: { label: string; at: string | null | undefined; required?: boolean }) {
  return (
    <tr>
      <th scope="row" className="text-left font-normal">
        {label}
        {required && <span className="text-xs text-slate-500"> (required)</span>}
      </th>
      <td>
        {at ? (
          <span className="badge border-emerald-300 bg-emerald-50 text-emerald-900">Given {formatDateTime(at)}</span>
        ) : (
          <span className="badge border-slate-300 bg-slate-50 text-slate-700">Not given</span>
        )}
      </td>
    </tr>
  );
}

export function SubmissionReview({ submission }: { submission: SubmissionDetail }) {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const [notes, setNotes] = useState("");
  const [reason, setReason] = useState("");
  const [rejectStatus, setRejectStatus] = useState<RejectStatus>("rejected");
  const [reasonError, setReasonError] = useState<string | null>(null);
  const [current, setCurrent] = useState<Submission>(submission);
  const [companyId, setCompanyId] = useState<string | null>(submission.company_id);

  const refresh = (s: Submission) => {
    setCurrent(s);
    void qc.invalidateQueries({ queryKey: ["admin", "submissions"] });
    void qc.invalidateQueries({ queryKey: ["admin", "stats"] });
  };

  const approve = useMutation({
    mutationFn: () => api.post<{ submission: Submission; company_id: string }>(`/admin/submissions/${submission.id}/approve`, notes.trim() ? { notes: notes.trim() } : {}),
    onSuccess: (res) => {
      refresh(res.submission);
      setCompanyId(res.company_id);
      toast.success("Approved. A private company record was created — it is not published.");
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  const reject = useMutation({
    mutationFn: () => api.post<{ submission: Submission }>(`/admin/submissions/${submission.id}/reject`, { reason: reason.trim(), status: rejectStatus }),
    onSuccess: (res) => {
      refresh(res.submission);
      toast.success(`Submission marked as ${humanize(rejectStatus).toLowerCase()}.`);
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  const p = submission.payload;
  const open = current.status === "submitted" || current.status === "under_review";
  const busy = approve.isPending || reject.isPending;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3 text-sm">
        <span>
          Status: <StatusBadge status={current.status} />
        </span>
        <span className="text-slate-600">Submitted {formatDateTime(current.submitted_at)}</span>
        {current.reviewed_at && <span className="text-slate-600">Reviewed {formatDateTime(current.reviewed_at)}</span>}
        {companyId && (
          <Link href={`/admin/companies/${companyId}`} className="link">
            Open company record
          </Link>
        )}
      </div>

      <section className="card space-y-3" aria-labelledby="consents-h">
        <h2 id="consents-h" className="text-lg font-semibold">
          Permissions
        </h2>
        <p className="text-sm text-slate-700">Privacy notice version: <span className="font-mono">{current.privacy_notice_version}</span></p>
        <table className="w-full text-sm">
          <tbody>
            <ConsentRow label="Privacy notice acknowledged" at={current.consents?.storage_acknowledged_at} required />
            <ConsentRow label="Public listing permission" at={current.consents?.public_listing_at} />
            <ConsentRow label="Future commercial/API distribution" at={current.consents?.commercial_api_at} />
          </tbody>
        </table>
        {!current.consents?.public_listing_at && (
          <p className="text-sm text-amber-900">No public listing permission: the resulting company must stay private.</p>
        )}
      </section>

      <section className="card space-y-3" aria-labelledby="payload-h">
        <h2 id="payload-h" className="text-lg font-semibold">
          Submitted details
        </h2>
        <dl className="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[12rem_1fr]">
          <dt className="text-slate-600">Business name</dt>
          <dd className="font-medium">{p.company.display_name}</dd>
          {p.company.legal_name && (
            <>
              <dt className="text-slate-600">Legal name</dt>
              <dd>{p.company.legal_name}</dd>
            </>
          )}
          {p.company.website_url && (
            <>
              <dt className="text-slate-600">Website</dt>
              <dd className="break-all">{p.company.website_url}</dd>
            </>
          )}
          {p.company.establishment_year && (
            <>
              <dt className="text-slate-600">Established</dt>
              <dd>{p.company.establishment_year}</dd>
            </>
          )}
          {p.company.description && (
            <>
              <dt className="text-slate-600">Description</dt>
              <dd className="whitespace-pre-line">{p.company.description}</dd>
            </>
          )}
          <dt className="text-slate-600">Roles</dt>
          <dd>{p.roles.map(humanize).join(", ")}</dd>
          <dt className="text-slate-600">Categories</dt>
          <dd className="font-mono text-xs">{p.categories.join(", ")}</dd>
          <dt className="text-slate-600">Products</dt>
          <dd>{p.products?.length ? p.products.map((x) => x.name).join(", ") : "—"}</dd>
          <dt className="text-slate-600">Facilities</dt>
          <dd>
            <ul className="space-y-1">
              {p.facilities.map((f, i) => (
                <li key={i}>
                  {facilityLabel(f.facility_type)}: {[f.address_line1, f.locality, f.city, f.district, f.state_code, f.postal_code].filter(Boolean).join(", ")}
                </li>
              ))}
            </ul>
          </dd>
          <dt className="text-slate-600">Business contacts</dt>
          <dd>{p.business_contacts?.length ? p.business_contacts.map((c) => `${humanize(c.contact_type)}: ${c.value}`).join("; ") : "—"}</dd>
          <dt className="text-slate-600">Submitter (private)</dt>
          <dd>
            {p.submitter?.name}
            {p.submitter?.designation ? `, ${p.submitter.designation}` : ""} · {submission.submitter_email ?? p.submitter?.email}
            {p.submitter?.phone ? ` · ${p.submitter.phone}` : ""}
          </dd>
        </dl>
        <details>
          <summary className="cursor-pointer text-sm font-medium">Raw payload</summary>
          <JsonBlock value={p} />
        </details>
        {submission.review_notes && <p className="text-sm text-slate-700">Review notes: {submission.review_notes}</p>}
      </section>

      {open ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <section className="card space-y-3" aria-labelledby="approve-h">
            <h2 id="approve-h" className="text-lg font-semibold">
              Approve
            </h2>
            <p className="text-sm text-slate-700">Creates a private company record (approved_private) with provenance. It does not publish anything.</p>
            <div>
              <label htmlFor="approve-notes" className="label">
                Notes (optional)
              </label>
              <textarea id="approve-notes" rows={2} className="input" value={notes} onChange={(e) => setNotes(e.target.value)} />
            </div>
            <button type="button" className="btn-primary" disabled={busy} onClick={() => approve.mutate()}>
              {approve.isPending ? "Approving…" : "Approve as private"}
            </button>
          </section>
          <section className="card space-y-3" aria-labelledby="reject-h">
            <h2 id="reject-h" className="text-lg font-semibold">
              Reject or request changes
            </h2>
            <div>
              <label htmlFor="reject-status" className="label">
                Outcome
              </label>
              <select id="reject-status" className="input" value={rejectStatus} onChange={(e) => setRejectStatus(e.target.value as RejectStatus)}>
                <option value="rejected">Rejected</option>
                <option value="needs_changes">Needs changes</option>
                <option value="spam">Spam</option>
              </select>
            </div>
            <div>
              <label htmlFor="reject-reason" className="label">
                Reason <span className="text-red-700">*</span>
              </label>
              <textarea
                id="reject-reason"
                rows={2}
                className="input"
                value={reason}
                aria-invalid={reasonError ? true : undefined}
                aria-describedby={reasonError ? "reject-reason-error" : undefined}
                onChange={(e) => setReason(e.target.value)}
              />
              {reasonError && (
                <p id="reject-reason-error" className="field-error" role="alert">
                  {reasonError}
                </p>
              )}
            </div>
            <button
              type="button"
              className="btn-danger"
              disabled={busy}
              onClick={() => {
                if (!reason.trim()) {
                  setReasonError("Enter a reason.");
                  return;
                }
                setReasonError(null);
                reject.mutate();
              }}
            >
              {reject.isPending ? "Saving…" : "Confirm outcome"}
            </button>
          </section>
        </div>
      ) : (
        <p className="rounded-md border border-slate-200 bg-white px-4 py-3 text-sm text-slate-700">This submission has been reviewed ({humanize(current.status)}).</p>
      )}
    </div>
  );
}
