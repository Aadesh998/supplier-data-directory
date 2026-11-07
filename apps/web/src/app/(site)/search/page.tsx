import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { CompanyGrid } from "@/components/site/CompanyCard";
import { ApiUnavailableNotice, EmptyNotice } from "@/components/site/Notices";
import { firstParam, Pagination, parsePage } from "@/components/site/Pagination";
import { SearchFilters } from "@/components/search/SearchFilters";
import { DEFAULT_PAGE_SIZE } from "@/lib/constants";
import { noindexMetadata } from "@/lib/seo";
import { getCategoryTree, getRoles, getStates, searchCompanies } from "@/lib/server-api";
import { flattenTree } from "@/lib/tree";

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

// Search result permutations are never indexed.
export const metadata = noindexMetadata("Search suppliers", "Search published supplier profiles.");

export default async function SearchPage({ searchParams }: Props) {
  const sp = await searchParams;
  const filters = {
    q: firstParam(sp.q),
    category: firstParam(sp.category),
    state: firstParam(sp.state),
    city: firstParam(sp.city),
    role: firstParam(sp.role),
  };
  const page = parsePage(sp.page);
  const [results, tree, states, roles] = await Promise.all([
    searchCompanies({ ...filters, page, limit: DEFAULT_PAGE_SIZE }),
    getCategoryTree(),
    getStates(),
    getRoles(),
  ]);
  const categoryOptions = tree.ok
    ? flattenTree(tree.data.data).map(({ node, depth }) => ({ value: node.slug, label: `${"  ".repeat(depth)}${node.name}` }))
    : [];
  const stateOptions = states.ok ? states.data.data.map((s) => ({ value: s.slug, label: s.name })) : [];
  const roleOptions = roles.ok ? roles.data.data.map((r) => ({ value: r.code, label: r.name })) : [];
  const roleNames = roles.ok ? Object.fromEntries(roles.data.data.map((r) => [r.code, r.name])) : undefined;

  return (
    <div className="container-page space-y-6 py-8">
      <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: "Search", path: "/search" }]} />
      <h1 className="text-3xl font-bold">Search suppliers</h1>
      <SearchFilters initial={filters} categories={categoryOptions} states={stateOptions} roles={roleOptions} />
      <section aria-labelledby="results-h" aria-live="polite" className="space-y-4">
        <h2 id="results-h" className="sr-only">
          Results
        </h2>
        {!results.ok ? (
          <ApiUnavailableNotice what="search results" />
        ) : results.data.data.length === 0 ? (
          <EmptyNotice>No published profiles match these filters. Try fewer filters or a broader keyword.</EmptyNotice>
        ) : (
          <>
            <CompanyGrid companies={results.data.data} roleNames={roleNames} />
            <Pagination page={results.data.page} basePath="/search" params={filters} />
          </>
        )}
      </section>
    </div>
  );
}
