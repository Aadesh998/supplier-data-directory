"use client";

import { ApiError, errorMessage } from "@/lib/api-error";
import { humanize } from "@/lib/labels";
import type { PageInfo } from "@/lib/types";

export function PageHeader({ title, description, actions }: { title: string; description?: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
      <div className="min-w-0 space-y-1">
        <h1 className="truncate text-xl font-semibold tracking-tight">{title}</h1>
        {description && <div className="max-w-3xl text-sm text-muted-foreground">{description}</div>}
      </div>
      {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
    </div>
  );
}

export function LoadingState({ label = "Loading…" }: { label?: string }) {
  return (
    <div role="status" className="flex items-center gap-2 rounded-md border bg-card px-4 py-6 text-sm text-muted-foreground">
      <span aria-hidden="true" className="size-4 animate-spin rounded-full border-2 border-border border-t-primary" />
      {label}
    </div>
  );
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const forbidden = error instanceof ApiError && error.status === 403;
  return (
    <div role="alert" className={`rounded-md border px-4 py-4 text-sm ${forbidden ? "border-amber-300 bg-amber-50 text-amber-950" : "border-red-300 bg-red-50 text-red-950"}`}>
      <p className="font-semibold">{forbidden ? "You don't have permission to view this" : "Something went wrong"}</p>
      <p className="mt-1">{forbidden ? "Your role does not allow this action. Ask an owner if you need access." : errorMessage(error)}</p>
      {error instanceof ApiError && error.requestId && <p className="mt-1 text-xs opacity-80">Request ID: {error.requestId}</p>}
      {onRetry && !forbidden && (
        <button type="button" className="btn-secondary btn-sm mt-3" onClick={onRetry}>
          Try again
        </button>
      )}
    </div>
  );
}

export function EmptyState({ children }: { children: React.ReactNode }) {
  return <div className="rounded-md border border-dashed bg-card px-4 py-8 text-center text-sm text-muted-foreground">{children}</div>;
}

const STATUS_STYLES: Record<string, string> = {
  published: "border-emerald-300 bg-emerald-50 text-emerald-900",
  approved_private: "border-sky-300 bg-sky-50 text-sky-900",
  approved: "border-emerald-300 bg-emerald-50 text-emerald-900",
  verified: "border-emerald-300 bg-emerald-50 text-emerald-900",
  partially_verified: "border-sky-300 bg-sky-50 text-sky-900",
  submitted: "border-amber-300 bg-amber-50 text-amber-900",
  under_review: "border-amber-300 bg-amber-50 text-amber-900",
  received: "border-amber-300 bg-amber-50 text-amber-900",
  in_progress: "border-sky-300 bg-sky-50 text-sky-900",
  staged: "border-amber-300 bg-amber-50 text-amber-900",
  open: "border-amber-300 bg-amber-50 text-amber-900",
  committed: "border-emerald-300 bg-emerald-50 text-emerald-900",
  resolved: "border-emerald-300 bg-emerald-50 text-emerald-900",
  rejected: "border-red-300 bg-red-50 text-red-900",
  spam: "border-red-300 bg-red-50 text-red-900",
  failed: "border-red-300 bg-red-50 text-red-900",
  stale: "border-orange-300 bg-orange-50 text-orange-900",
  needs_changes: "border-orange-300 bg-orange-50 text-orange-900",
  archived: "border-slate-300 bg-slate-100 text-slate-700",
  rolled_back: "border-slate-300 bg-slate-100 text-slate-700",
};

export function StatusBadge({ status }: { status: string | null | undefined }) {
  if (!status) return <span className="text-muted-foreground">—</span>;
  return <span className={`badge ${STATUS_STYLES[status] ?? "border-slate-300 bg-white text-slate-700"}`}>{humanize(status)}</span>;
}

export function BoolBadge({ value, yes = "Yes", no = "No" }: { value: boolean | null | undefined; yes?: string; no?: string }) {
  return value ? (
    <span className="badge border-emerald-300 bg-emerald-50 text-emerald-900">{yes}</span>
  ) : (
    <span className="badge border-slate-300 bg-slate-50 text-slate-700">{no}</span>
  );
}

export function AdminPagination({ page, onPage }: { page: PageInfo | undefined; onPage: (p: number) => void }) {
  if (!page) return null;
  const totalPages = Math.max(1, Math.ceil(page.total / Math.max(1, page.limit)));
  return (
    <nav aria-label="Pagination" className="mt-4 flex items-center justify-between gap-3 text-sm">
      <button type="button" className="btn-secondary btn-sm" disabled={page.page <= 1} onClick={() => onPage(page.page - 1)}>
        Previous
      </button>
      <span className="text-muted-foreground">
        Page {page.page} of {totalPages} · {page.total} total
      </span>
      <button type="button" className="btn-secondary btn-sm" disabled={!page.has_more} onClick={() => onPage(page.page + 1)}>
        Next
      </button>
    </nav>
  );
}

export function Section({ title, children, actions, id }: { title: string; children: React.ReactNode; actions?: React.ReactNode; id?: string }) {
  return (
    <section className="card space-y-4" aria-labelledby={id}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id={id} className="text-base font-semibold tracking-tight">
          {title}
        </h2>
        {actions}
      </div>
      {children}
    </section>
  );
}

export function JsonBlock({ value }: { value: unknown }) {
  return <pre className="max-h-96 overflow-auto rounded-md border bg-muted p-3 font-mono text-xs text-foreground">{JSON.stringify(value, null, 2)}</pre>;
}
