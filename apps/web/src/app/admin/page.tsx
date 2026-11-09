"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useApi } from "@/components/admin/AuthProvider";
import { ErrorState, LoadingState, PageHeader } from "@/components/admin/ui";
import { humanize } from "@/lib/labels";
import type { Stats } from "@/lib/types";

function Tile({ label, value, href }: { label: string; value: number | undefined; href?: string }) {
  const body = (
    <>
      <span className="block text-sm text-slate-600">{label}</span>
      <span className="mt-1 block text-3xl font-semibold tabular-nums text-slate-950">{value ?? 0}</span>
    </>
  );
  return href ? (
    <Link href={href} className="card block hover:border-accent-200 hover:bg-accent-50">
      {body}
    </Link>
  ) : (
    <div className="card">{body}</div>
  );
}

function Breakdown({ title, data }: { title: string; data: Record<string, number> | undefined }) {
  const entries = Object.entries(data ?? {});
  const total = entries.reduce((s, [, n]) => s + n, 0);
  return (
    <section className="card">
      <h2 className="mb-3 text-lg font-semibold">{title}</h2>
      {entries.length === 0 ? (
        <p className="text-sm text-slate-600">No companies yet.</p>
      ) : (
        <table className="w-full text-sm">
          <tbody>
            {entries.map(([k, n]) => (
              <tr key={k} className="border-b border-slate-100 last:border-0">
                <th scope="row" className="py-1.5 text-left font-normal text-slate-700">
                  {humanize(k)}
                </th>
                <td className="py-1.5 text-right font-medium tabular-nums">{n}</td>
                <td className="w-1/3 py-1.5 pl-3">
                  <div className="h-2 rounded bg-muted" aria-hidden="true">
                    <div className="h-2 rounded bg-primary" style={{ width: `${total ? (n / total) * 100 : 0}%` }} />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

export default function AdminDashboard() {
  const api = useApi();
  const q = useQuery({ queryKey: ["admin", "stats"], queryFn: () => api.get<Stats>("/admin/stats") });
  return (
    <div>
      <PageHeader title="Dashboard" description="Work queues and the state of the directory. Publishing does not imply verification." />
      {q.isLoading ? (
        <LoadingState />
      ) : q.error ? (
        <ErrorState error={q.error} onRetry={() => q.refetch()} />
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <Tile label="Submissions awaiting review" value={q.data?.submissions_pending} href="/admin/submissions" />
            <Tile label="Imports staged" value={q.data?.imports_staged} href="/admin/imports" />
            <Tile label="Open duplicate pairs" value={q.data?.open_duplicates} href="/admin/duplicates" />
            <Tile label="Open privacy requests" value={q.data?.privacy_requests_open} href="/admin/privacy-requests" />
            <Tile label="Companies due for re-verification" value={q.data?.due_for_review} href="/admin/companies?verification_status=stale" />
          </div>
          <div className="grid gap-4 lg:grid-cols-2">
            <Breakdown title="Companies by publication status" data={q.data?.companies_by_publication_status} />
            <Breakdown title="Companies by verification status" data={q.data?.companies_by_verification_status} />
          </div>
        </div>
      )}
    </div>
  );
}
