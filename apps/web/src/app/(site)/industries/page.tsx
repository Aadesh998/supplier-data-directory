import Link from "next/link";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { ApiUnavailableNotice } from "@/components/site/Notices";
import { pageMetadata } from "@/lib/seo";
import { getCategoryTree } from "@/lib/server-api";
import type { CategoryNode } from "@/lib/types";

export async function generateMetadata() {
  return pageMetadata({
    title: "Browse Industries: Manufacturers by Sector",
    description: "Browse Indian manufacturers and suppliers by industry: food processing, textiles, ceramics and tiles, building materials, CNC machining, chemicals, packaging and more.",
    path: "/industries",
  });
}

function count(c: CategoryNode) {
  return c.published_company_count && c.published_company_count > 0 ? ` (${c.published_company_count})` : "";
}

export default async function IndustriesPage() {
  const tree = await getCategoryTree();
  return (
    <div className="container-page space-y-6 py-8">
      <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: "Industries", path: "/industries" }]} />
      <header className="space-y-2">
        <h1 className="text-3xl font-bold">Industries</h1>
        <p className="max-w-prose text-slate-700">
          Categories are organised as a tree: broad industries contain more specific product and service categories. Numbers in brackets
          are published profiles in that category, including its sub-categories.
        </p>
      </header>
      {!tree.ok ? (
        <ApiUnavailableNotice what="industry categories" />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {tree.data.data.map((top) => (
            <section key={top.id} className="card" aria-labelledby={`cat-${top.slug}`}>
              <h2 id={`cat-${top.slug}`} className="text-lg font-semibold">
                <Link href={`/industries/${top.slug}`} className="hover:text-accent-700 hover:underline">
                  {top.name}
                </Link>
                <span className="text-sm font-normal text-slate-600">{count(top)}</span>
              </h2>
              {top.description && <p className="mt-1 text-sm text-slate-700">{top.description}</p>}
              {!!top.children?.length && (
                <ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm">
                  {top.children.map((child) => (
                    <li key={child.id}>
                      <Link href={`/industries/${child.slug}`} className="link">
                        {child.name}
                      </Link>
                      <span className="text-slate-600">{count(child)}</span>
                    </li>
                  ))}
                </ul>
              )}
            </section>
          ))}
        </div>
      )}
    </div>
  );
}
