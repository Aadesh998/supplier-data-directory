"use client";

import { useId, useMemo, useState } from "react";
import type { FlatCategory } from "@/lib/tree";

type Props = {
  categories: FlatCategory[];
  value: string[];
  onChange: (value: string[]) => void;
  max: number;
  error?: string;
};

/** Searchable multi-select of categories (sends slugs). Keyboard accessible: plain checkboxes. */
export function CategoryPicker({ categories, value, onChange, max, error }: Props) {
  const id = useId();
  const [query, setQuery] = useState("");
  const bySlug = useMemo(() => new Map(categories.map((c) => [c.node.slug, c])), [categories]);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return categories;
    return categories.filter(({ node, path }) => [...path, node.name].join(" ").toLowerCase().includes(q));
  }, [categories, query]);

  const atMax = value.length >= max;
  const toggle = (slug: string, checked: boolean) => {
    if (checked) {
      if (!value.includes(slug) && value.length < max) onChange([...value, slug]);
    } else onChange(value.filter((v) => v !== slug));
  };

  return (
    <fieldset aria-describedby={`${id}-hint${error ? ` ${id}-error` : ""}`}>
      <legend className="label">
        Categories <span className="text-red-700" aria-hidden="true">*</span>
      </legend>
      <p id={`${id}-hint`} className="hint mb-2">
        Choose 1 to {max} categories that best describe what you make or do. Pick the most specific ones, e.g. &ldquo;Vitrified tiles&rdquo;
        rather than only &ldquo;Building materials&rdquo;. Selected: {value.length} of {max}.
      </p>

      {value.length > 0 && (
        <ul className="mb-3 flex flex-wrap gap-2" aria-label="Selected categories">
          {value.map((slug) => (
            <li key={slug}>
              <button
                type="button"
                className="badge gap-1 border-accent-200 bg-accent-50 px-3 py-1 text-sm text-accent-900 hover:bg-accent-100"
                onClick={() => toggle(slug, false)}
              >
                {bySlug.get(slug)?.node.name ?? slug}
                <span aria-hidden="true">×</span>
                <span className="sr-only">(remove)</span>
              </button>
            </li>
          ))}
        </ul>
      )}

      <label htmlFor={`${id}-search`} className="sr-only">
        Search categories
      </label>
      <input
        id={`${id}-search`}
        type="search"
        className="input mb-2"
        placeholder="Search categories, e.g. namkeen, carpets, CNC"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      <div className="max-h-72 overflow-y-auto rounded-md border border-slate-200">
        {filtered.length === 0 ? (
          <p className="p-3 text-sm text-slate-600">No categories match &ldquo;{query}&rdquo;.</p>
        ) : (
          <ul>
            {filtered.map(({ node, path, depth }) => {
              const checked = value.includes(node.slug);
              return (
                <li key={node.id ?? node.slug}>
                  <label
                    className="flex cursor-pointer items-center gap-2 px-3 py-2 text-sm hover:bg-slate-50"
                    style={{ paddingLeft: query ? undefined : `${0.75 + depth * 1.25}rem` }}
                  >
                    <input
                      type="checkbox"
                      className="size-4"
                      checked={checked}
                      disabled={!checked && atMax}
                      onChange={(e) => toggle(node.slug, e.target.checked)}
                    />
                    <span className={depth === 0 ? "font-medium" : undefined}>
                      {node.name}
                      {query && path.length > 0 && <span className="ml-1 text-xs text-slate-600">in {path.join(" › ")}</span>}
                    </span>
                  </label>
                </li>
              );
            })}
          </ul>
        )}
      </div>
      {atMax && <p className="hint">You have chosen the maximum of {max} categories.</p>}
      {error && (
        <p id={`${id}-error`} className="field-error" role="alert">
          {error}
        </p>
      )}
    </fieldset>
  );
}
