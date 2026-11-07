"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { emptySource, SourceFields, sourceInputFromForm } from "@/components/admin/SourceFields";
import { SourceSelect } from "@/components/admin/SourceSelect";
import { AdminPagination, EmptyState, ErrorState, LoadingState, PageHeader, StatusBadge } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { ApiError, errorMessage } from "@/lib/api-error";
import { formatDateTime } from "@/lib/labels";
import type { ImportJob, Paginated } from "@/lib/types";

function UploadForm() {
  const api = useApi();
  const toast = useToast();
  const router = useRouter();
  const qc = useQueryClient();
  const [file, setFile] = useState<File | null>(null);
  const [mode, setMode] = useState<"existing" | "new">("existing");
  const [sourceId, setSourceId] = useState("");
  const [source, setSource] = useState(emptySource());
  const [declared, setDeclared] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const upload = useMutation({
    mutationFn: (fd: FormData) => api.request<ImportJob>("POST", "/admin/imports", { body: fd }),
    onSuccess: (job) => {
      toast.success(`Uploaded ${job.filename}: ${job.total_rows} rows staged.`);
      void qc.invalidateQueries({ queryKey: ["admin", "imports"] });
      router.push(`/admin/imports/${job.id}`);
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) setError("This file has already been imported for this source.");
      else setError(errorMessage(e));
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (!file) return setError("Choose a CSV file.");
    if (file.size > 10 * 1024 * 1024) return setError("The file is larger than 10 MB.");
    if (mode === "existing" && !sourceId) return setError("Choose the data source this file came from.");
    if (mode === "new" && !source.provider.trim()) return setError("Enter the provider for the new source.");
    if (!declared) return setError("Confirm the data-origin declaration.");
    const fd = new FormData();
    fd.append("file", file);
    if (mode === "existing") fd.append("source_id", sourceId);
    else fd.append("source", JSON.stringify(sourceInputFromForm(source)));
    upload.mutate(fd);
  }

  return (
    <form onSubmit={onSubmit} className="card space-y-4" noValidate>
      <h2 className="text-lg font-semibold">Upload a CSV</h2>
      <div aria-live="assertive" role="status">
        {error && <p className="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-900">{error}</p>}
      </div>
      <div>
        <label htmlFor="imp-file" className="label">
          CSV file <span className="text-red-700">*</span>
        </label>
        <input id="imp-file" type="file" accept=".csv,text/csv" className="input" onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
        <p className="hint">Max 10 MB and 20,000 rows. The first row must contain column headers.</p>
      </div>
      <fieldset className="space-y-3">
        <legend className="label">
          Data origin declaration <span className="text-red-700">*</span>
        </legend>
        <div className="flex flex-wrap gap-4 text-sm">
          <label className="flex items-center gap-2">
            <input type="radio" name="origin" checked={mode === "existing"} onChange={() => setMode("existing")} /> Existing source
          </label>
          <label className="flex items-center gap-2">
            <input type="radio" name="origin" checked={mode === "new"} onChange={() => setMode("new")} /> New source
          </label>
        </div>
        {mode === "existing" ? (
          <SourceSelect id="imp-source" value={sourceId} onChange={setSourceId} label="Where did this file come from?" />
        ) : (
          <SourceFields idPrefix="imp-new" value={source} onChange={setSource} />
        )}
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-950">
          Records from a source with unknown reuse rights stay private and staged. They cannot be published or exported commercially until the
          source&apos;s permissions are documented.
        </div>
        <label className="flex items-start gap-2 text-sm">
          <input type="checkbox" className="mt-1" checked={declared} onChange={(e) => setDeclared(e.target.checked)} />
          <span>I confirm the origin and permissions declared above are accurate to the best of my knowledge.</span>
        </label>
      </fieldset>
      <button type="submit" className="btn-primary" disabled={upload.isPending}>
        {upload.isPending ? "Uploading…" : "Upload and stage"}
      </button>
    </form>
  );
}

export default function ImportsPage() {
  const api = useApi();
  const [page, setPage] = useState(1);
  const q = useQuery({
    queryKey: ["admin", "imports", page],
    queryFn: () => api.get<Paginated<ImportJob>>("/admin/imports", { page, limit: 20 }),
    placeholderData: keepPreviousData,
  });
  return (
    <div className="space-y-6">
      <PageHeader title="Imports" description="Upload → map columns → review rows and duplicates → dry-run → commit. Nothing is committed without review." />
      <UploadForm />
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Import jobs</h2>
        {q.isLoading ? (
          <LoadingState />
        ) : q.error ? (
          <ErrorState error={q.error} onRetry={() => q.refetch()} />
        ) : !q.data?.data.length ? (
          <EmptyState>No imports yet.</EmptyState>
        ) : (
          <>
            <div className="table-wrap">
              <table className="table">
                <thead>
                  <tr>
                    <th scope="col">File</th>
                    <th scope="col">Status</th>
                    <th scope="col">Rows</th>
                    <th scope="col">Errors</th>
                    <th scope="col">Duplicates</th>
                    <th scope="col">Committed</th>
                    <th scope="col">Source</th>
                    <th scope="col">Uploaded</th>
                  </tr>
                </thead>
                <tbody>
                  {q.data.data.map((j) => (
                    <tr key={j.id}>
                      <td>
                        <Link href={`/admin/imports/${j.id}`} className="link font-medium">
                          {j.filename}
                        </Link>
                      </td>
                      <td>
                        <StatusBadge status={j.status} />
                      </td>
                      <td className="tabular-nums">{j.total_rows}</td>
                      <td className="tabular-nums">{j.error_rows}</td>
                      <td className="tabular-nums">{j.duplicate_rows}</td>
                      <td className="tabular-nums">{j.committed_rows}</td>
                      <td className="text-xs">{j.source?.provider ?? "—"}</td>
                      <td className="whitespace-nowrap text-xs">{formatDateTime(j.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <AdminPagination page={q.data.page} onPage={setPage} />
          </>
        )}
      </section>
    </div>
  );
}
