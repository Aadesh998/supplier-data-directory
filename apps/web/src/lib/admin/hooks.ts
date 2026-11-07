"use client";

import { useQuery } from "@tanstack/react-query";
import { useApi } from "@/components/admin/AuthProvider";
import type { Category, DataList, DataSource, Paginated, Role, State } from "../types";

export function useSources() {
  const api = useApi();
  return useQuery({
    queryKey: ["admin", "sources", "all"],
    queryFn: () => api.get<Paginated<DataSource>>("/admin/sources", { page: 1, limit: 100 }),
  });
}

export function useRoles() {
  const api = useApi();
  return useQuery({ queryKey: ["public", "roles"], queryFn: () => api.get<DataList<Role>>("/roles"), staleTime: 10 * 60_000 });
}

export function useStates() {
  const api = useApi();
  return useQuery({ queryKey: ["public", "states"], queryFn: () => api.get<DataList<State>>("/states"), staleTime: 10 * 60_000 });
}

export function useAdminCategories() {
  const api = useApi();
  return useQuery({ queryKey: ["admin", "categories"], queryFn: () => api.get<DataList<Category>>("/admin/categories") });
}

/** "Parent › Child" labels for a flat category list. */
export function categoryPathLabels(categories: Category[]): Record<string, string> {
  const byId = new Map(categories.map((c) => [c.id, c]));
  const labels: Record<string, string> = {};
  for (const c of categories) {
    const parts = [c.name];
    let parent = c.parent_id ? byId.get(c.parent_id) : undefined;
    let guard = 0;
    while (parent && guard++ < 20) {
      parts.unshift(parent.name);
      parent = parent.parent_id ? byId.get(parent.parent_id) : undefined;
    }
    labels[c.id] = parts.join(" › ");
  }
  return labels;
}
