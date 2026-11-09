"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState, type FormEvent } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDateTime } from "@/lib/labels";
import type { DiscoveryJob, JobBatch, JobStats, Paginated } from "@/lib/types";

const STATUSES = ["pending", "running", "done", "failed", "paused"] as const;

function Stats({ stats }: { stats: JobStats }) {
  const tiles: [string, number][] = [
    ["Due now", stats.due_now],
    ...STATUSES.map((s) => [s.charAt(0).toUpperCase() + s.slice(1), stats.by_status[s] ?? 0] as [string, number]),
    ["Results found", stats.total_results],
  ];
  return (
    <section className="card space-y-3">
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
        {tiles.map(([label, n]) => (
          <div key={label} className="rounded-md border border-slate-200 p-3">
            <dt className="text-xs text-slate-600">{label}</dt>
            <dd className="text-xl font-semibold tabular-nums">{n.toLocaleString("en-IN")}</dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-slate-600">
        Last worker run: {stats.last_run_at ? formatDateTime(stats.last_run_at) : "never"}
        {stats.last_batch && (
          <>
            {" · "}Last upload: {stats.last_batch.filename} ({formatDateTime(stats.last_batch.created_at)})
          </>
        )}
      </p>
    </section>
  );
}

function Upload({ canEdit }: { canEdit: boolean }) {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const fileRef = useRef<HTMLInputElement>(null);
  const [reset, setReset] = useState(false);
  const [result, setResult] = useState<JobBatch | null>(null);
  const upload = useMutation({
    mutationFn: (fd: FormData) => api.request<JobBatch>("POST", "/admin/discovery/jobs/upload", { body: fd }),
    onSuccess: (b) => {
      setResult(b);
      toast.success(`Uploaded: ${b.inserted} new, ${b.updated} updated, ${b.skipped} skipped.`);
      void qc.invalidateQueries({ queryKey: ["admin", "discovery", "jobs"] });
      if (fileRef.current) fileRef.current.value = "";
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const file = fileRef.current?.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.set("file", file);
    if (reset) fd.set("reset_state", "true");
    upload.mutate(fd);
  }
  if (!canEdit) return null;
  return (
    <section className="card space-y-3">
      <h2 className="text-lg font-semibold">Upload jobs CSV</h2>
      <p className="text-sm text-slate-700">
        Rows are matched by <code>job_id</code>: new ids are added, existing ids get their search details updated. Their run
        history is kept unless you tick “reset”. Required columns: <code>job_id, keyword, search_query, location, latitude,
        longitude, radius_km</code>. Optional: <code>provider</code> (<code>osm_overpass</code> default, or{" "}
        <code>google_places_ids</code>, or <code>gmaps_scraper</code> for the Google Maps worker), <code>priority, category_slug, state_code, osm_name_regex, refresh_interval_days</code>.
      </p>
      <form onSubmit={onSubmit} className="flex flex-wrap items-end gap-3">
        <div>
          <label htmlFor="jobs-file" className="label">
            CSV file
          </label>
          <input id="jobs-file" ref={fileRef} type="file" accept=".csv,text/csv" className="input" required />
        </div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={reset} onChange={(e) => setReset(e.target.checked)} />
          Reset run state of existing jobs (re-run them)
        </label>
        <button type="submit" className="btn-primary" disabled={upload.isPending}>
          {upload.isPending ? "Uploading…" : "Upload"}
        </button>
      </form>
      {result && result.errors.length > 0 && (
        <details className="text-sm">
          <summary className="cursor-pointer">{result.errors.length} warning(s)</summary>
          <ul className="mt-2 max-h-48 list-disc overflow-auto pl-5 text-xs text-slate-700">
            {result.errors.map((e) => (
              <li key={e}>{e}</li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}

export default function DiscoveryJobsPage() {
  const api = useApi();
  const { user } = useAuth();
  const toast = useToast();
  const qc = useQueryClient();
  const canEdit = user?.role === "owner" || user?.role === "admin";
  const [status, setStatus] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);

  const stats = useQuery({
    queryKey: ["admin", "discovery", "jobs", "stats"],
    queryFn: () => api.get<JobStats>("/admin/discovery/jobs/stats"),
    refetchInterval: 30_000,
  });
  const jobs = useQuery({
    queryKey: ["admin", "discovery", "jobs", "list", status, q, page],
    queryFn: () => api.get<Paginated<DiscoveryJob>>("/admin/discovery/jobs", { status, q, page, limit: 50 }),
    placeholderData: keepPreviousData,
  });
  const action = useMutation({
    mutationFn: (body: { action: string; status?: string; job_ids?: string[] }) =>
      api.post<{ affected: number }>("/admin/discovery/jobs/actions", body),
    onSuccess: (r) => {
      toast.success(`${r.affected} job(s) updated.`);
      void qc.invalidateQueries({ queryKey: ["admin", "discovery", "jobs"] });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title="Discovery jobs"
        description="Search jobs that workers claim on a schedule (e.g. 4 every 5 minutes). Finished jobs run again after their refresh interval, so results stay current."
      />
      {stats.data ? <Stats stats={stats.data} /> : stats.error ? <ErrorState error={stats.error} /> : <LoadingState />}
      <Upload canEdit={canEdit} />
      <section className="card space-y-3">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <label htmlFor="job-status" className="label">
              Status
            </label>
            <select id="job-status" className="input" value={status} onChange={(e) => { setStatus(e.target.value); setPage(1); }}>
              <option value="">All</option>
              {STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label htmlFor="job-q" className="label">
              Search
            </label>
            <input id="job-q" className="input" value={q} placeholder="tiles Morbi, or a job_id" onChange={(e) => { setQ(e.target.value); setPage(1); }} />
          </div>
          {canEdit && (
            <div className="flex flex-wrap gap-2">
              <button type="button" className="btn-secondary btn-sm" disabled={action.isPending} onClick={() => action.mutate({ action: "pause", status: status || undefined })}>
                Pause {status || "all"}
              </button>
              <button type="button" className="btn-secondary btn-sm" disabled={action.isPending} onClick={() => action.mutate({ action: "resume" })}>
                Resume paused
              </button>
              <button type="button" className="btn-secondary btn-sm" disabled={action.isPending} onClick={() => action.mutate({ action: "retry_failed" })}>
                Retry failed
              </button>
            </div>
          )}
        </div>
        {jobs.isLoading ? (
          <LoadingState />
        ) : jobs.error ? (
          <ErrorState error={jobs.error} onRetry={() => void jobs.refetch()} />
        ) : !jobs.data?.data.length ? (
          <EmptyState>No jobs. Upload a jobs CSV to start.</EmptyState>
        ) : (
          <>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th scope="col">P</th>
                    <th scope="col">Search</th>
                    <th scope="col">Provider</th>
                    <th scope="col">Status</th>
                    <th scope="col">Results</th>
                    <th scope="col">Last run</th>
                    <th scope="col">Next run</th>
                    {canEdit && <th scope="col"><span className="sr-only">Actions</span></th>}
                  </tr>
                </thead>
                <tbody>
                  {jobs.data.data.map((j) => (
                    <tr key={j.id}>
                      <td className="tabular-nums">{j.priority}</td>
                      <td>
                        <p className="text-sm">{j.search_query}</p>
                        <p className="font-mono text-xs text-slate-500">{j.job_key}</p>
                        {j.last_error && <p className="text-xs text-red-700">{j.last_error}</p>}
                      </td>
                      <td className="text-xs">{j.provider === "osm_overpass" ? "OpenStreetMap" : j.provider === "gmaps_scraper" ? "Maps scraper" : "Places IDs"}</td>
                      <td>
                        <StatusBadge status={j.status} />
                        {j.attempts > 0 && <span className="ml-1 text-xs text-slate-500">try {j.attempts}/{j.max_attempts}</span>}
                      </td>
                      <td className="tabular-nums text-sm">
                        {j.results_count ?? "—"}
                        {j.total_results > 0 && <span className="text-xs text-slate-500"> ({j.total_results} new total)</span>}
                      </td>
                      <td className="whitespace-nowrap text-xs">{j.last_run_at ? formatDateTime(j.last_run_at) : "—"}</td>
                      <td className="whitespace-nowrap text-xs">{j.next_run_after ? formatDateTime(j.next_run_after) : "asap"}</td>
                      {canEdit && (
                        <td className="whitespace-nowrap">
                          {j.status === "paused" ? (
                            <button type="button" className="btn-ghost btn-sm" onClick={() => action.mutate({ action: "resume", job_ids: [j.id] })}>
                              Resume
                            </button>
                          ) : j.status !== "running" ? (
                            <button type="button" className="btn-ghost btn-sm" onClick={() => action.mutate({ action: "run_now", job_ids: [j.id] })}>
                              Run next
                            </button>
                          ) : null}
                        </td>
                      )}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <AdminPagination page={jobs.data.page} onPage={setPage} />
          </>
        )}
      </section>
    </div>
  );
}
