"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { useApi, useAuth } from "@/components/admin/AuthProvider";
import { PageHeader } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { ApiError, errorMessage } from "@/lib/api-error";
import { saveBlob } from "@/lib/admin/client";
import { humanize } from "@/lib/labels";

const SCOPES = [
  { value: "internal", label: "Internal", help: "All fields permitted for internal use. For private analysis only — do not share." },
  { value: "public", label: "Public", help: "Only fields whose sources allow public display." },
  { value: "commercial", label: "Commercial", help: "Only fields whose sources allow commercial/API distribution. Owner only." },
] as const;

export default function ExportsPage() {
  const api = useApi();
  const { user } = useAuth();
  const toast = useToast();
  const [scope, setScope] = useState<(typeof SCOPES)[number]["value"]>("internal");
  const [pubStatus, setPubStatus] = useState("");
  const [error, setError] = useState<string | null>(null);
  const m = useMutation({
    mutationFn: () => api.download("POST", "/admin/exports", { scope, ...(pubStatus ? { publication_status: pubStatus } : {}) }),
    onSuccess: (blob) => {
      const stamp = new Date().toISOString().slice(0, 10);
      saveBlob(blob, `bharatmfg-${scope}-export-${stamp}.csv`);
      toast.success("Export downloaded. The export was recorded in the audit log.");
    },
    onError: (e) => {
      setError(e instanceof ApiError && e.status === 403 ? "Your role is not allowed to run this export scope." : errorMessage(e));
    },
  });
  return (
    <div className="max-w-2xl space-y-4">
      <PageHeader title="Exports" description="Permission-filtered CSV exports. Every export is audit-logged." />
      <form
        className="card space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          m.mutate();
        }}
      >
        <fieldset className="space-y-2">
          <legend className="label">Scope</legend>
          {SCOPES.map((s) => (
            <label key={s.value} className="flex items-start gap-2 rounded-md border border-slate-200 p-3 text-sm">
              <input type="radio" name="scope" className="mt-1" checked={scope === s.value} onChange={() => setScope(s.value)} />
              <span>
                <span className="font-medium">{s.label}</span>
                <span className="block text-slate-700">{s.help}</span>
              </span>
            </label>
          ))}
        </fieldset>
        {scope === "commercial" && user?.role !== "owner" && <p className="text-sm text-amber-900">Commercial exports are owner-only; the API will refuse this request.</p>}
        <div>
          <label htmlFor="ex-pub" className="label">
            Publication status (optional)
          </label>
          <select id="ex-pub" className="input" value={pubStatus} onChange={(e) => setPubStatus(e.target.value)}>
            <option value="">Any</option>
            {["approved_private", "published", "archived"].map((s) => (
              <option key={s} value={s}>
                {humanize(s)}
              </option>
            ))}
          </select>
        </div>
        <div aria-live="assertive" role="status">
          {error && <p className="rounded-md border border-red-300 bg-red-50 px-3 py-2 text-sm text-red-900">{error}</p>}
        </div>
        <button type="submit" className="btn-primary" disabled={m.isPending}>
          {m.isPending ? "Preparing…" : "Download CSV"}
        </button>
      </form>
    </div>
  );
}
