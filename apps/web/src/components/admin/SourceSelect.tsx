"use client";

import { useSources } from "@/lib/admin/hooks";
import { humanize } from "@/lib/labels";
import type { DataSource } from "@/lib/types";

export function sourceLabel(s: DataSource): string {
  const flags = [s.allowed_internal_use && "internal", s.allowed_public_display && "public", s.allowed_commercial_api && "commercial"].filter(Boolean);
  return `${s.provider} (${humanize(s.source_type)}) — ${flags.length ? flags.join(", ") : "no permissions"}`;
}

/** Required source picker for any admin data edit (every field needs provenance). */
export function SourceSelect({
  id,
  value,
  onChange,
  required = true,
  label = "Source for this change",
}: {
  id: string;
  value: string;
  onChange: (v: string) => void;
  required?: boolean;
  label?: string;
}) {
  const { data, isLoading, error } = useSources();
  return (
    <div>
      <label htmlFor={id} className="label">
        {label}
        {required && <span className="ml-1 text-red-700">*</span>}
      </label>
      <select id={id} className="input" value={value} onChange={(e) => onChange(e.target.value)} aria-required={required} disabled={isLoading}>
        <option value="">{isLoading ? "Loading sources…" : "Choose the source of this information…"}</option>
        {data?.data.map((s) => (
          <option key={s.id} value={s.id}>
            {sourceLabel(s)}
          </option>
        ))}
      </select>
      {error ? <p className="field-error">Could not load sources.</p> : <p className="hint">Every edit is recorded with its source so field permissions can be enforced.</p>}
    </div>
  );
}
