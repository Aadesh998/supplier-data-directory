import type { CategoryNode, Role, State } from "@/lib/types";

export const roles: Role[] = [
  { code: "manufacturer", name: "Manufacturer", description: "Makes products in its own facility" },
  { code: "exporter", name: "Exporter" },
];

export const categoryTree: CategoryNode[] = [
  {
    id: "11111111-1111-1111-1111-111111111111",
    parent_id: null,
    name: "Food and beverages",
    slug: "food-and-beverages",
    category_type: "industry",
    children: [
      {
        id: "22222222-2222-2222-2222-222222222222",
        parent_id: "11111111-1111-1111-1111-111111111111",
        name: "Namkeen",
        slug: "namkeen",
        category_type: "product_category",
        children: [],
      },
    ],
  },
];

export const states: State[] = [
  { code: "GJ", name: "Gujarat", slug: "gujarat", region_type: "state" },
  { code: "MH", name: "Maharashtra", slug: "maharashtra", region_type: "state" },
];

export function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}
