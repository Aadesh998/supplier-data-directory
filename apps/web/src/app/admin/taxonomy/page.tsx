"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState, type FormEvent } from "react";
import { useApi } from "@/components/admin/AuthProvider";
import { EmptyState, ErrorState, LoadingState, PageHeader } from "@/components/admin/ui";
import { useToast } from "@/components/ui/Toast";
import { ApiError, errorMessage } from "@/lib/api-error";
import { categoryPathLabels, useAdminCategories } from "@/lib/admin/hooks";
import { humanize } from "@/lib/labels";
import type { Category } from "@/lib/types";

const TYPES = ["industry", "product_category", "service", "trade"];

type FormState = { id?: string; parent_id: string; name: string; slug: string; category_type: string; description: string; sort_order: string; active: boolean };

const blank = (parent_id = ""): FormState => ({ parent_id, name: "", slug: "", category_type: "product_category", description: "", sort_order: "0", active: true });

function slugify(s: string) {
  return s
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 80);
}

function TreeList({ nodes, childrenOf, onEdit, onAddChild, labels }: { nodes: Category[]; childrenOf: Map<string, Category[]>; onEdit: (c: Category) => void; onAddChild: (c: Category) => void; labels: Record<string, string> }) {
  return (
    <ul className="space-y-1 border-l border-slate-200 pl-4">
      {nodes.map((c) => {
        const kids = childrenOf.get(c.id) ?? [];
        return (
          <li key={c.id}>
            <div className="flex flex-wrap items-center gap-2 py-1 text-sm">
              <span className={c.active === false ? "text-slate-500 line-through" : "font-medium"}>{c.name}</span>
              <span className="font-mono text-xs text-slate-500">{c.slug}</span>
              <span className="badge border-slate-200 bg-slate-50 text-slate-600">{humanize(c.category_type)}</span>
              {typeof c.published_company_count === "number" && <span className="text-xs text-slate-600">{c.published_company_count} published</span>}
              <button type="button" className="btn-ghost btn-sm" onClick={() => onEdit(c)}>
                Edit<span className="sr-only"> {labels[c.id]}</span>
              </button>
              <button type="button" className="btn-ghost btn-sm" onClick={() => onAddChild(c)}>
                Add child<span className="sr-only"> to {labels[c.id]}</span>
              </button>
            </div>
            {kids.length > 0 && <TreeList nodes={kids} childrenOf={childrenOf} onEdit={onEdit} onAddChild={onAddChild} labels={labels} />}
          </li>
        );
      })}
    </ul>
  );
}

export default function TaxonomyPage() {
  const api = useApi();
  const toast = useToast();
  const qc = useQueryClient();
  const q = useAdminCategories();
  const [form, setForm] = useState<FormState | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const cats = useMemo(() => q.data?.data ?? [], [q.data]);
  const labels = useMemo(() => categoryPathLabels(cats), [cats]);
  const childrenOf = useMemo(() => {
    const m = new Map<string, Category[]>();
    for (const c of cats) {
      const key = c.parent_id ?? "root";
      m.set(key, [...(m.get(key) ?? []), c]);
    }
    for (const list of m.values()) list.sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0) || a.name.localeCompare(b.name));
    return m;
  }, [cats]);

  const save = useMutation({
    mutationFn: (f: FormState) => {
      if (f.id) {
        const body: Record<string, unknown> = { name: f.name.trim(), description: f.description.trim(), active: f.active, sort_order: Number(f.sort_order) || 0 };
        if (f.parent_id) body.parent_id = f.parent_id;
        return api.patch<Category>(`/admin/categories/${f.id}`, body);
      }
      const body: Record<string, unknown> = { name: f.name.trim(), slug: f.slug.trim(), category_type: f.category_type, sort_order: Number(f.sort_order) || 0 };
      if (f.parent_id) body.parent_id = f.parent_id;
      if (f.description.trim()) body.description = f.description.trim();
      return api.post<Category>("/admin/categories", body);
    },
    onSuccess: (c) => {
      toast.success(`Saved “${c.name}”.`);
      setForm(null);
      void qc.invalidateQueries({ queryKey: ["admin", "categories"] });
    },
    onError: (e) => {
      if (e instanceof ApiError) setErrors(e.fieldErrors);
      toast.error(errorMessage(e));
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!form) return;
    setErrors({});
    if (!form.name.trim() || (!form.id && !form.slug.trim())) {
      setErrors({ name: !form.name.trim() ? "Name is required" : "", slug: !form.slug.trim() ? "Slug is required" : "" });
      return;
    }
    save.mutate(form);
  }

  const set = (k: keyof FormState) => (e: { target: { value: string } }) => setForm((f) => (f ? { ...f, [k]: e.target.value } : f));

  return (
    <div className="space-y-4">
      <PageHeader
        title="Taxonomy"
        description="The category tree used for classification and public industry pages. Descriptions appear on public pages — write original, useful text."
        actions={
          <button type="button" className="btn-primary" onClick={() => setForm(blank())}>
            New top-level category
          </button>
        }
      />
      <div className="grid gap-6 lg:grid-cols-[3fr_2fr]">
        <section className="card overflow-x-auto">
          {q.isLoading ? (
            <LoadingState />
          ) : q.error ? (
            <ErrorState error={q.error} onRetry={() => q.refetch()} />
          ) : cats.length === 0 ? (
            <EmptyState>No categories yet.</EmptyState>
          ) : (
            <TreeList
              nodes={childrenOf.get("root") ?? []}
              childrenOf={childrenOf}
              labels={labels}
              onEdit={(c) =>
                setForm({
                  id: c.id,
                  parent_id: c.parent_id ?? "",
                  name: c.name,
                  slug: c.slug,
                  category_type: c.category_type,
                  description: c.description ?? "",
                  sort_order: String(c.sort_order ?? 0),
                  active: c.active !== false,
                })
              }
              onAddChild={(c) => setForm(blank(c.id))}
            />
          )}
        </section>
        {form && (
          <form onSubmit={onSubmit} className="card space-y-3 self-start lg:sticky lg:top-20" noValidate>
            <h2 className="text-lg font-semibold">{form.id ? "Edit category" : "New category"}</h2>
            <div>
              <label htmlFor="tx-parent" className="label">
                Parent
              </label>
              <select id="tx-parent" className="input" value={form.parent_id} onChange={set("parent_id")}>
                <option value="">— Top level —</option>
                {cats
                  .filter((c) => c.id !== form.id)
                  .sort((a, b) => (labels[a.id] ?? "").localeCompare(labels[b.id] ?? ""))
                  .map((c) => (
                    <option key={c.id} value={c.id}>
                      {labels[c.id]}
                    </option>
                  ))}
              </select>
            </div>
            <div>
              <label htmlFor="tx-name" className="label">
                Name <span className="text-red-700">*</span>
              </label>
              <input
                id="tx-name"
                className="input"
                value={form.name}
                onChange={(e) => setForm((f) => (f ? { ...f, name: e.target.value, slug: f.id ? f.slug : slugify(e.target.value) } : f))}
              />
              {errors.name && <p className="field-error">{errors.name}</p>}
            </div>
            <div>
              <label htmlFor="tx-slug" className="label">
                Slug {!form.id && <span className="text-red-700">*</span>}
              </label>
              <input id="tx-slug" className="input font-mono" value={form.slug} onChange={set("slug")} disabled={!!form.id} />
              {form.id && <p className="hint">Slugs cannot be changed after creation.</p>}
              {errors.slug && <p className="field-error">{errors.slug}</p>}
            </div>
            {!form.id && (
              <div>
                <label htmlFor="tx-type" className="label">
                  Type
                </label>
                <select id="tx-type" className="input" value={form.category_type} onChange={set("category_type")}>
                  {TYPES.map((t) => (
                    <option key={t} value={t}>
                      {humanize(t)}
                    </option>
                  ))}
                </select>
              </div>
            )}
            <div>
              <label htmlFor="tx-desc" className="label">
                Description
              </label>
              <textarea id="tx-desc" rows={4} className="input" value={form.description} onChange={set("description")} />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="tx-sort" className="label">
                  Sort order
                </label>
                <input id="tx-sort" type="number" className="input" value={form.sort_order} onChange={set("sort_order")} />
              </div>
              {form.id && (
                <label className="flex items-center gap-2 self-end text-sm">
                  <input type="checkbox" checked={form.active} onChange={(e) => setForm((f) => (f ? { ...f, active: e.target.checked } : f))} />
                  Active
                </label>
              )}
            </div>
            <div className="flex gap-2">
              <button type="submit" className="btn-primary" disabled={save.isPending}>
                {save.isPending ? "Saving…" : "Save"}
              </button>
              <button type="button" className="btn-ghost" onClick={() => setForm(null)}>
                Cancel
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
