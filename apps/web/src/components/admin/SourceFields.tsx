"use client";

import { humanize } from "@/lib/labels";
import type { SourceType } from "@/lib/types";

export const SOURCE_TYPES: SourceType[] = [
  "self_submission",
  "authorized_import",
  "official_site",
  "licensed_feed",
  "permitted_api_lookup",
  "manual_verification",
];

export type SourceFormValues = {
  source_type: string;
  provider: string;
  source_url: string;
  license_reference: string;
  notes: string;
  retention_expires_at: string;
  allowed_internal_use: boolean;
  allowed_public_display: boolean;
  allowed_commercial_api: boolean;
};

export const emptySource = (): SourceFormValues => ({
  source_type: "authorized_import",
  provider: "",
  source_url: "",
  license_reference: "",
  notes: "",
  retention_expires_at: "",
  // Restrictive defaults: nothing is permitted until evidenced.
  allowed_internal_use: false,
  allowed_public_display: false,
  allowed_commercial_api: false,
});

export function sourceInputFromForm(v: SourceFormValues) {
  const out: Record<string, unknown> = {
    source_type: v.source_type,
    provider: v.provider.trim(),
    allowed_internal_use: v.allowed_internal_use,
    allowed_public_display: v.allowed_public_display,
    allowed_commercial_api: v.allowed_commercial_api,
  };
  if (v.source_url.trim()) out.source_url = v.source_url.trim();
  if (v.license_reference.trim()) out.license_reference = v.license_reference.trim();
  if (v.notes.trim()) out.notes = v.notes.trim();
  if (v.retention_expires_at) out.retention_expires_at = new Date(`${v.retention_expires_at}T00:00:00Z`).toISOString();
  return out;
}

export function SourceFields({ value, onChange, idPrefix, showPermissionHelp = true }: { value: SourceFormValues; onChange: (v: SourceFormValues) => void; idPrefix: string; showPermissionHelp?: boolean }) {
  const set = (k: keyof SourceFormValues) => (e: { target: { value: string } }) => onChange({ ...value, [k]: e.target.value });
  const flag = (k: "allowed_internal_use" | "allowed_public_display" | "allowed_commercial_api") => (e: { target: { checked: boolean } }) =>
    onChange({ ...value, [k]: e.target.checked });
  return (
    <div className="space-y-3">
      <div className="grid gap-3 md:grid-cols-2">
        <div>
          <label htmlFor={`${idPrefix}-type`} className="label">
            Source type <span className="text-red-700">*</span>
          </label>
          <select id={`${idPrefix}-type`} className="input" value={value.source_type} onChange={set("source_type")}>
            {SOURCE_TYPES.map((t) => (
              <option key={t} value={t}>
                {humanize(t)}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor={`${idPrefix}-provider`} className="label">
            Provider <span className="text-red-700">*</span>
          </label>
          <input id={`${idPrefix}-provider`} className="input" value={value.provider} onChange={set("provider")} placeholder="Who supplied the data" />
        </div>
        <div>
          <label htmlFor={`${idPrefix}-url`} className="label">
            Source URL
          </label>
          <input id={`${idPrefix}-url`} className="input" value={value.source_url} onChange={set("source_url")} />
        </div>
        <div>
          <label htmlFor={`${idPrefix}-license`} className="label">
            Licence / permission reference
          </label>
          <input id={`${idPrefix}-license`} className="input" value={value.license_reference} onChange={set("license_reference")} placeholder="Contract, email or licence ID" />
        </div>
        <div>
          <label htmlFor={`${idPrefix}-retention`} className="label">
            Retention expires
          </label>
          <input id={`${idPrefix}-retention`} type="date" className="input" value={value.retention_expires_at} onChange={set("retention_expires_at")} />
        </div>
        <div>
          <label htmlFor={`${idPrefix}-notes`} className="label">
            Notes
          </label>
          <input id={`${idPrefix}-notes`} className="input" value={value.notes} onChange={set("notes")} />
        </div>
      </div>
      <fieldset className="space-y-2 rounded-md border border-slate-200 p-3">
        <legend className="px-1 text-sm font-semibold">Documented permissions (all off by default)</legend>
        <label className="flex items-start gap-2 text-sm">
          <input type="checkbox" className="mt-1" checked={value.allowed_internal_use} onChange={flag("allowed_internal_use")} />
          <span>
            <strong>Internal use</strong> — we may store and review this data privately. Required before an import can be committed.
          </span>
        </label>
        <label className="flex items-start gap-2 text-sm">
          <input type="checkbox" className="mt-1" checked={value.allowed_public_display} onChange={flag("allowed_public_display")} />
          <span>
            <strong>Public display</strong> — fields from this source may be shown on public profiles.
          </span>
        </label>
        <label className="flex items-start gap-2 text-sm">
          <input type="checkbox" className="mt-1" checked={value.allowed_commercial_api} onChange={flag("allowed_commercial_api")} />
          <span>
            <strong>Commercial/API distribution</strong> — fields may be included in commercial exports or a future API.
          </span>
        </label>
        {showPermissionHelp && (
          <p className="text-xs text-amber-900">
            Only tick a permission you can evidence (licence, contract or written consent). If reuse rights are unknown, leave them unticked —
            the data stays private.
          </p>
        )}
      </fieldset>
    </div>
  );
}
