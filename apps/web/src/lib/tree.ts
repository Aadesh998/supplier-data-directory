import type { CategoryNode } from "./types";

export type FlatCategory = { node: CategoryNode; path: string[]; depth: number };

/** Flatten a category tree depth-first, keeping ancestor names for labels. */
export function flattenTree(nodes: CategoryNode[], parents: string[] = []): FlatCategory[] {
  const out: FlatCategory[] = [];
  for (const node of nodes) {
    out.push({ node, path: parents, depth: parents.length });
    if (node.children?.length) out.push(...flattenTree(node.children, [...parents, node.name]));
  }
  return out;
}
