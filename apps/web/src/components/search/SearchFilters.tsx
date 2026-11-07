"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { buildSearchQuery, type SearchFilterValues } from "@/lib/search";

export type Option = { value: string; label: string };

type Props = {
  initial: SearchFilterValues;
  categories: Option[];
  states: Option[];
  roles: Option[];
};

export function SearchFilters({ initial, categories, states, roles }: Props) {
  const router = useRouter();
  const [values, setValues] = useState<SearchFilterValues>({
    q: initial.q ?? "",
    category: initial.category ?? "",
    state: initial.state ?? "",
    city: initial.city ?? "",
    role: initial.role ?? "",
  });

  const set = (key: keyof SearchFilterValues) => (e: { target: { value: string } }) => setValues((v) => ({ ...v, [key]: e.target.value }));

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const qs = buildSearchQuery(values);
    router.push(qs ? `/search?${qs}` : "/search");
  }

  function onReset() {
    setValues({ q: "", category: "", state: "", city: "", role: "" });
    router.push("/search");
  }

  const hasFilters = Object.values(values).some((v) => v && v.trim());

  return (
    <form role="search" aria-label="Search suppliers" onSubmit={onSubmit} className="card grid gap-4 md:grid-cols-6">
      <div className="md:col-span-6">
        <label htmlFor="sf-q" className="label">
          Keyword
        </label>
        <input id="sf-q" type="search" className="input" value={values.q} onChange={set("q")} maxLength={100} placeholder="Company name, product or process" />
      </div>
      <div className="md:col-span-2">
        <label htmlFor="sf-category" className="label">
          Category
        </label>
        <select id="sf-category" className="input" value={values.category} onChange={set("category")}>
          <option value="">Any category</option>
          {categories.map((c) => (
            <option key={c.value} value={c.value}>
              {c.label}
            </option>
          ))}
        </select>
      </div>
      <div className="md:col-span-2">
        <label htmlFor="sf-state" className="label">
          State
        </label>
        <select id="sf-state" className="input" value={values.state} onChange={set("state")}>
          <option value="">Any state</option>
          {states.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
      </div>
      <div className="md:col-span-2">
        <label htmlFor="sf-city" className="label">
          City
        </label>
        <input id="sf-city" className="input" value={values.city} onChange={set("city")} maxLength={100} placeholder="e.g. Rajkot" />
      </div>
      <div className="md:col-span-3">
        <label htmlFor="sf-role" className="label">
          Business role
        </label>
        <select id="sf-role" className="input" value={values.role} onChange={set("role")}>
          <option value="">Any role</option>
          {roles.map((r) => (
            <option key={r.value} value={r.value}>
              {r.label}
            </option>
          ))}
        </select>
      </div>
      <div className="flex items-end gap-2 md:col-span-3 md:justify-end">
        {hasFilters && (
          <button type="button" className="btn-ghost" onClick={onReset}>
            Clear filters
          </button>
        )}
        <button type="submit" className="btn-primary">
          Search
        </button>
      </div>
    </form>
  );
}
