"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { AdminPagination, BoolBadge, EmptyState, ErrorState, JsonBlock, LoadingState, PageHeader, Section, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { errorMessage } from "@/lib/api-error";
import { formatDateTime, humanize } from "@/lib/labels";
import type { CommitResult, DataList, ImportField, ImportJob, ImportRow, Paginated } from "@/lib/types";

const NOT_MAPPED = "";

function MappingEditor({ job }: { job: ImportJob }) {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const fields = useQuery({ queryKey: ["admin", "import-fields"], queryFn: () => api.get<DataList<ImportField>>("/admin/import-fields") });
  const [mapping, setMapping] = useState<Record<string, string>>(job.column_mapping ?? {});
  const save = useMutation({
    mutationFn: () => {
      const clean = Object.fromEntries(Object.entries(mapping).filter(([, v]) => v));
      return api.post<ImportJob>(`/admin/imports/${job.id}/mapping`, { column_mapping: clean, async: true });
    },
    onSuccess: (j) => {
      qc.setQueryData(["admin", "import", job.id], j);
      toast.success("Mapping saved; re-checking rows in the background.");
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const editable = (job.status === "staged" || job.status === "uploaded") && !job.task;
  const headers = job.headers ?? [];

  return (
    <Section title="Column mapping" id="map-h">
      <p className="text-sm text-slate-700">Match each directory field to a column in the CSV. Re-applying the mapping re-runs normalisation and duplicate detection.</p>
      {fields.isLoading ? (
        <LoadingState />
      ) : fields.error ? (
        <ErrorState error={fields.error} />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {fields.data?.data.map((f) => (
            <div key={f.field}>
              <label htmlFor={`map-${f.field}`} className="label">
                {f.label}
                {f.required && <span className="ml-1 text-red-700">*</span>}
              </label>
              <select
                id={`map-${f.field}`}
                className="input"
                disabled={!editable}
                value={mapping[f.field] ?? NOT_MAPPED}
                onChange={(e) => setMapping((m) => ({ ...m, [f.field]: e.target.value }))}
              >
                <option value={NOT_MAPPED}>— not mapped —</option>
                {headers.map((h) => (
                  <option key={h} value={h}>
                    {h}
                  </option>
                ))}
              </select>
              {f.description && <p className="hint">{f.description}</p>}
            </div>
          ))}
        </div>
      )}
      {editable ? (
        <button type="button" className="btn-primary" disabled={save.isPending} onClick={() => save.mutate()}>
          {save.isPending ? "Applying…" : "Apply mapping"}
        </button>
      ) : (
        <p className="text-sm text-slate-600">Mapping can only be changed while the job is staged.</p>
      )}
    </Section>
  );
}

function RowsTable({ job }: { job: ImportJob }) {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const [filter, setFilter] = useState<"" | "errors" | "duplicates" | "clean">("");
  const [disposition, setDisposition] = useState("");
  const [page, setPage] = useState(1);
  const [expanded, setExpanded] = useState<string | null>(null);
  const q = useQuery({
    queryKey: ["admin", "import-rows", job.id, filter, disposition, page, job.task_finished_at ?? ""],
    queryFn: () => api.get<Paginated<ImportRow>>(`/admin/imports/${job.id}/rows`, { filter, disposition, page, limit: 50 }),
    placeholderData: keepPreviousData,
  });
  const update = useMutation({
    mutationFn: (v: { rowId: string; disposition: ImportRow["disposition"]; matched_company_id?: string }) =>
      api.patch<ImportRow>(`/admin/imports/${job.id}/rows/${v.rowId}`, {
        disposition: v.disposition,
        ...(v.matched_company_id ? { matched_company_id: v.matched_company_id } : {}),
      }),
    onSuccess: (row) => {
      void qc.invalidateQueries({ queryKey: ["admin", "import-rows", job.id] });
      void qc.invalidateQueries({ queryKey: ["admin", "import", job.id] });
      toast.success(`Row ${row.row_number}: ${humanize(row.disposition)}.`);
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const editable = job.status === "staged";

  const tabs: { value: typeof filter; label: string }[] = [
    { value: "", label: "All rows" },
    { value: "errors", label: `Errors (${job.error_rows})` },
    { value: "duplicates", label: `Possible duplicates (${job.duplicate_rows})` },
    { value: "clean", label: "Clean" },
  ];

  return (
    <Section title="Rows" id="rows-h">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div role="tablist" aria-label="Row filter" className="flex flex-wrap gap-1">
          {tabs.map((t) => (
            <button
              key={t.value || "all"}
              type="button"
              role="tab"
              aria-selected={filter === t.value}
              className={filter === t.value ? "btn-primary btn-sm" : "btn-secondary btn-sm"}
              onClick={() => {
                setFilter(t.value);
                setPage(1);
              }}
            >
              {t.label}
            </button>
          ))}
        </div>
        <div>
          <label htmlFor="row-disp" className="label">
            Disposition
          </label>
          <select
            id="row-disp"
            className="input"
            value={disposition}
            onChange={(e) => {
              setDisposition(e.target.value);
              setPage(1);
            }}
          >
            <option value="">Any</option>
            {["pending", "accepted", "rejected", "merged"].map((d) => (
              <option key={d} value={d}>
                {humanize(d)}
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
        <EmptyState>No rows match.</EmptyState>
      ) : (
        <>
          <div className="table-wrap">
            <table className="table">
              <thead>
                <tr>
                  <th scope="col">#</th>
                  <th scope="col">Name / location</th>
                  <th scope="col">Problems</th>
                  <th scope="col">Duplicate candidates</th>
                  <th scope="col">Disposition</th>
                  <th scope="col">
                    <span className="sr-only">Details</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {q.data.data.map((r) => {
                  const nd = (r.normalized_data ?? {}) as Record<string, unknown>;
                  const name = String(nd.display_name ?? Object.values(r.raw_data)[0] ?? "—");
                  const loc = [nd.city, nd.state_code].filter(Boolean).join(", ");
                  return (
                    <tr key={r.id}>
                      <td className="tabular-nums">{r.row_number}</td>
                      <td>
                        <span className="font-medium">{name}</span>
                        {loc && <div className="text-xs text-slate-600">{loc}</div>}
                        {r.created_company_id && (
                          <Link href={`/admin/companies/${r.created_company_id}`} className="link text-xs">
                            Created company
                          </Link>
                        )}
                      </td>
                      <td className="text-xs">
                        {r.validation_errors?.map((e, i) => (
                          <p key={`e${i}`} className="text-red-800">
                            <span className="font-mono">{e.field}</span>: {e.message}
                          </p>
                        ))}
                        {r.validation_warnings?.map((w, i) => (
                          <p key={`w${i}`} className="text-amber-900">
                            <span className="font-mono">{w.field}</span>: {w.message}
                          </p>
                        ))}
                        {!r.validation_errors?.length && !r.validation_warnings?.length && <span className="text-slate-500">None</span>}
                      </td>
                      <td className="text-xs">
                        {r.duplicate_candidates?.length ? (
                          <ul className="space-y-1">
                            {r.duplicate_candidates.map((c) => (
                              <li key={c.company_id} className="flex flex-wrap items-center gap-1">
                                <Link href={`/admin/companies/${c.company_id}`} className="link" target="_blank">
                                  {c.display_name}
                                </Link>
                                <span className="text-slate-600">({Math.round(c.score * 100)}%)</span>
                                {c.reasons?.length ? <span className="text-slate-500">{c.reasons.join(", ")}</span> : null}
                                {editable && (
                                  <button
                                    type="button"
                                    className="btn-secondary btn-sm"
                                    disabled={update.isPending}
                                    onClick={() => update.mutate({ rowId: r.id, disposition: "merged", matched_company_id: c.company_id })}
                                  >
                                    Merge into this
                                  </button>
                                )}
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <span className="text-slate-500">None</span>
                        )}
                      </td>
                      <td>
                        <StatusBadge status={r.disposition} />
                        {editable && (
                          <div className="mt-1 flex flex-wrap gap-1">
                            {r.disposition !== "accepted" && (
                              <button type="button" className="btn-secondary btn-sm" disabled={update.isPending || !!r.validation_errors?.length} onClick={() => update.mutate({ rowId: r.id, disposition: "accepted" })}>
                                Accept
                              </button>
                            )}
                            {r.disposition !== "rejected" && (
                              <button type="button" className="btn-secondary btn-sm" disabled={update.isPending} onClick={() => update.mutate({ rowId: r.id, disposition: "rejected" })}>
                                Reject
                              </button>
                            )}
                            {r.disposition !== "pending" && (
                              <button type="button" className="btn-ghost btn-sm" disabled={update.isPending} onClick={() => update.mutate({ rowId: r.id, disposition: "pending" })}>
                                Reset
                              </button>
                            )}
                          </div>
                        )}
                      </td>
                      <td>
                        <button type="button" className="btn-ghost btn-sm" aria-expanded={expanded === r.id} onClick={() => setExpanded((x) => (x === r.id ? null : r.id))}>
                          {expanded === r.id ? "Hide" : "Details"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {expanded &&
            (() => {
              const r = q.data.data.find((x) => x.id === expanded);
              if (!r) return null;
              return (
                <div className="grid gap-3 md:grid-cols-3">
                  <div>
                    <h3 className="mb-1 text-sm font-semibold">Row {r.row_number}: raw</h3>
                    <JsonBlock value={r.raw_data} />
                  </div>
                  <div>
                    <h3 className="mb-1 text-sm font-semibold">Normalised</h3>
                    <JsonBlock value={r.normalized_data ?? {}} />
                  </div>
                  <div>
                    <h3 className="mb-1 text-sm font-semibold">Transformation log</h3>
                    {r.transformation_log?.length ? (
                      <ul className="list-disc space-y-1 pl-5 text-xs">
                        {r.transformation_log.map((l, i) => (
                          <li key={i}>{l}</li>
                        ))}
                      </ul>
                    ) : (
                      <p className="text-xs text-slate-600">No transformations.</p>
                    )}
                  </div>
                </div>
              );
            })()}
          <AdminPagination page={q.data.page} onPage={setPage} />
        </>
      )}
    </Section>
  );
}

function ResultView({ r }: { r: CommitResult }) {
  return (
    <div className="rounded-md border border-slate-200 bg-slate-50 p-3 text-sm">
      <p className="font-semibold">{r.dry_run ? "Dry-run preview (nothing saved)" : "Commit result"}</p>
      <dl className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
        {(["created", "merged", "skipped", "rejected"] as const).map((k) => (
          <div key={k}>
            <dt className="text-slate-600">{humanize(k)}</dt>
            <dd className="text-xl font-semibold tabular-nums">{r[k]}</dd>
          </div>
        ))}
      </dl>
      {!!r.errors?.length && (
        <ul className="mt-2 list-disc pl-5 text-red-800">
          {r.errors.map((e, i) => (
            <li key={i}>
              Row {e.row_number}: {e.message}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function CommitPanel({ job }: { job: ImportJob }) {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const [includeClean, setIncludeClean] = useState(false);
  // The task launched from this page (dry run or commit) and when, so its outcome can be derived
  // from the polled job without effects.
  const [launched, setLaunched] = useState<{ task: "dry_run" | "commit"; at: string } | null>(null);

  const finishedAfterLaunch =
    !!launched && !job.task && !!job.task_finished_at && job.task_finished_at >= launched.at;
  const outcome = finishedAfterLaunch ? (job.task_result as CommitResult | null) : null;
  const preview = launched?.task === "dry_run" && outcome?.dry_run ? outcome : null;
  const result = launched?.task === "commit" && outcome && !outcome.dry_run ? outcome : null;
  const running = !!job.task;

  const commit = useMutation({
    mutationFn: (dryRun: boolean) =>
      api.post<ImportJob>(`/admin/imports/${job.id}/commit`, { dry_run: dryRun, include_clean_pending: includeClean, async: true }),
    onSuccess: (j, dryRun) => {
      setLaunched({ task: dryRun ? "dry_run" : "commit", at: j.task_started_at ?? new Date().toISOString() });
      qc.setQueryData(["admin", "import", job.id], j);
      void qc.invalidateQueries({ queryKey: ["admin", "imports"] });
      if (!dryRun) toast.success("Commit started. It runs on the server; you can leave this page.");
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const rollback = useMutation({
    mutationFn: () => api.post<ImportJob>(`/admin/imports/${job.id}/rollback`),
    onSuccess: (j) => {
      qc.setQueryData(["admin", "import", job.id], j);
      void qc.invalidateQueries({ queryKey: ["admin", "import-rows", job.id] });
      toast.success("Import rolled back.");
    },
    onError: (e) => toast.error(errorMessage(e)),
  });

  return (
    <Section title="Commit" id="commit-h">
      {job.status === "staged" ? (
        <>
          <p className="text-sm text-slate-700">
            Commits rows marked accepted or merged. Created companies stay private (never auto-published). The declared source must allow
            internal use.
          </p>
          {job.source && !job.source.allowed_internal_use && (
            <p className="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-900">
              The declared source does not allow internal use, so the API will refuse to commit. Update the source&apos;s documented permissions first.
            </p>
          )}
          <label className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              className="mt-1"
              checked={includeClean}
              onChange={(e) => {
                setIncludeClean(e.target.checked);
                setLaunched(null);
              }}
            />
            <span>Also create pending rows that have no errors and no duplicate candidates</span>
          </label>
          <div className="flex flex-wrap gap-2">
            <button type="button" className="btn-secondary" disabled={commit.isPending || running} onClick={() => commit.mutate(true)}>
              {job.task === "dry_run" ? "Previewing…" : "Preview (dry run)"}
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={commit.isPending || running || !preview}
              title={!preview ? "Run a dry-run preview first" : undefined}
              onClick={() => window.confirm("Commit this import? Companies will be created as private records.") && commit.mutate(false)}
            >
              Commit import
            </button>
          </div>
          {!preview && !running && <p className="hint">Run a preview first to see what will happen.</p>}
          {preview && <ResultView r={preview} />}
        </>
      ) : (
        <p className="text-sm text-slate-700">This job is {humanize(job.status).toLowerCase()}.</p>
      )}
      {result && <ResultView r={result} />}
      {job.status === "committed" && (
        <button
          type="button"
          className="btn-danger"
          disabled={rollback.isPending}
          onClick={() => window.confirm("Roll back this import? Companies it created will be removed and merges reverted where possible.") && rollback.mutate()}
        >
          Roll back import
        </button>
      )}
    </Section>
  );
}

const TASK_TEXT = {
  process: "Checking rows: normalising values and looking for duplicates",
  dry_run: "Running the dry-run preview",
  commit: "Committing rows",
} as const;

function TaskBanner({ job }: { job: ImportJob }) {
  if (job.task) {
    return (
      <div role="status" aria-live="polite" className="flex items-center gap-3 rounded-md border border-accent-200 bg-accent-50 px-4 py-3 text-sm">
        <span aria-hidden="true" className="size-4 shrink-0 animate-spin rounded-full border-2 border-accent-600 border-t-transparent" />
        <span>
          <strong className="font-semibold">{TASK_TEXT[job.task]}…</strong> {job.task_started_at && <>Started {formatDateTime(job.task_started_at)}.</>} This runs on
          the server: you can leave this page and come back. Large files against a remote database can take several minutes.
        </span>
      </div>
    );
  }
  if (job.task_error) {
    return (
      <p role="alert" className="rounded-md border border-red-300 bg-red-50 px-4 py-3 text-sm text-red-900">
        <strong className="font-semibold">The last operation did not complete:</strong> {job.task_error}
      </p>
    );
  }
  return null;
}

export default function ImportDetailPage() {
  const { id } = useParams<{ id: string }>();
  const api = useApi();
  const q = useQuery({
    queryKey: ["admin", "import", id],
    queryFn: () => api.get<ImportJob>(`/admin/imports/${id}`),
    enabled: !!id,
    // Poll while a background task runs.
    refetchInterval: (query) => (query.state.data?.task ? 2000 : false),
  });
  if (q.isLoading) return <LoadingState />;
  if (q.error) return <ErrorState error={q.error} onRetry={() => q.refetch()} />;
  const job = q.data;
  if (!job) return null;
  return (
    <div className="space-y-6">
      <p className="text-sm">
        <Link href="/admin/imports" className="link">
          ← All imports
        </Link>
      </p>
      <PageHeader
        title={job.filename}
        description={`Uploaded ${formatDateTime(job.created_at)}${job.committed_at ? ` · committed ${formatDateTime(job.committed_at)}` : ""}`}
        actions={
          <a href={`/api/v1/admin/imports/${job.id}/report.csv`} className="btn-secondary" download>
            Download report.csv
          </a>
        }
      />
      <TaskBanner job={job} />
      <Section title="Summary" id="sum-h">
        <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3 lg:grid-cols-6">
          <div>
            <dt className="text-slate-600">Status</dt>
            <dd>
              <StatusBadge status={job.status} />
            </dd>
          </div>
          {(
            [
              ["Total rows", job.total_rows],
              ["Valid", job.valid_rows],
              ["Errors", job.error_rows],
              ["Duplicates", job.duplicate_rows],
              ["Committed", job.committed_rows],
            ] as const
          ).map(([label, n]) => (
            <div key={label}>
              <dt className="text-slate-600">{label}</dt>
              <dd className="text-xl font-semibold tabular-nums">{n}</dd>
            </div>
          ))}
        </dl>
        {job.source && (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-slate-600">Declared source:</span> <span className="font-medium">{job.source.provider}</span> ({humanize(job.source.source_type)})
            <span>
              internal <BoolBadge value={job.source.allowed_internal_use} />
            </span>
            <span>
              public <BoolBadge value={job.source.allowed_public_display} />
            </span>
            <span>
              commercial <BoolBadge value={job.source.allowed_commercial_api} />
            </span>
          </div>
        )}
      </Section>
      <MappingEditor key={JSON.stringify(job.column_mapping ?? {})} job={job} />
      <RowsTable job={job} />
      <CommitPanel job={job} />
    </div>
  );
}
