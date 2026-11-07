"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { formatDateTime } from "@/lib/labels";
import type { Paginated, Submission } from "@/lib/types";

const STATUSES = ["submitted", "under_review", "approved", "rejected", "needs_changes", "spam", "withdrawn"];

export default function SubmissionsPage() {
  const api = useApi();
  const [status, setStatus] = useState("submitted");
  const [page, setPage] = useState(1);
  const q = useQuery({
    queryKey: ["admin", "submissions", status, page],
    queryFn: () => api.get<Paginated<Submission>>("/admin/submissions", { status, page, limit: 25 }),
    placeholderData: keepPreviousData,
  });
  return (
    <div className="space-y-4">
      <PageHeader title="Supplier submissions" description="Self-registrations awaiting review. Approving creates a private record; publishing is a separate step." />
      <div className="flex items-end gap-2">
        <div>
          <label htmlFor="sub-status" className="label">
            Status
          </label>
          <select
            id="sub-status"
            className="input"
            value={status}
            onChange={(e) => {
              setStatus(e.target.value);
              setPage(1);
            }}
          >
            <option value="">All</option>
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s.replace(/_/g, " ")}
              </option>
            ))}
          </select>
        </div>
      </div>
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : !q.data?.data.length ? (
        <EmptyState>No submissions with this status.</EmptyState>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">Business</th>
                  <th scope="col">Status</th>
                  <th scope="col">Submitted</th>
                  <th scope="col">Public listing</th>
                  <th scope="col">Commercial/API</th>
                </tr>
              </thead>
              <tbody>
                {q.data.data.map((s) => (
                  <tr key={s.id}>
                    <td>
                      <Link href={`/admin/submissions/${s.id}`} className="link font-medium">
                        {s.company_name}
                      </Link>
                    </td>
                    <td>
                      <StatusBadge status={s.status} />
                    </td>
                    <td className="whitespace-nowrap text-xs">{formatDateTime(s.submitted_at)}</td>
                    <td className="text-xs">{s.consents?.public_listing_at ? "Given" : "Not given"}</td>
                    <td className="text-xs">{s.consents?.commercial_api_at ? "Given" : "Not given"}</td>
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
