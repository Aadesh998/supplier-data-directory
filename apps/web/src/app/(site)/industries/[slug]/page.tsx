import Link from "next/link";
import { notFound } from "next/navigation";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { CompanyGrid } from "@/components/site/CompanyCard";
import { ApiUnavailableNotice, EmptyNotice } from "@/components/site/Notices";
import { Pagination, parsePage } from "@/components/site/Pagination";
import { DEFAULT_PAGE_SIZE } from "@/lib/constants";
import { pagedPath, pageMetadata } from "@/lib/seo";
import { getCategory, getRoles, searchCompanies } from "@/lib/server-api";

type Props = { params: Promise<{ slug: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> };

const THIN_THRESHOLD = 3;

export async function generateMetadata({ params, searchParams }: Props) {
  const { slug } = await params;
  const page = parsePage((await searchParams).page);
  const res = await getCategory(slug);
  if (!res.ok) {
    return { title: "Industry", robots: { index: false, follow: false } };
  }
  const { category, children } = res.data;
  const count = category.published_company_count ?? 0;
  const thin = count < THIN_THRESHOLD && children.length === 0;
  return pageMetadata({
    title:
      page > 1
        ? `${category.name} Manufacturers & Suppliers in India (page ${page})`
        : `${category.name} Manufacturers & Suppliers in India`,
    description: (
      `${count > 0 ? `${count} reviewed` : "Reviewed"} ${category.name.toLowerCase()} manufacturers, suppliers and job-work units in India. ` +
      (category.description ?? "Compare products, capabilities, certifications and locations by state.")
    ).slice(0, 158),
    path: pagedPath(`/industries/${category.slug}`, page),
    noindex: thin,
  });
}

export default async function IndustryPage({ params, searchParams }: Props) {
  const { slug } = await params;
  const page = parsePage((await searchParams).page);
  const res = await getCategory(slug);
  if (!res.ok) {
    if (res.status === 404) notFound();
    return (
      <div className="container-page py-8">
        <ApiUnavailableNotice what="this industry" />
      </div>
    );
  }
  const { category, ancestors, children } = res.data;
  const [companies, roles] = await Promise.all([
    searchCompanies({ category: category.slug, page, limit: DEFAULT_PAGE_SIZE }),
    getRoles(),
  ]);
  const roleNames = roles.ok ? Object.fromEntries(roles.data.data.map((r) => [r.code, r.name])) : undefined;

  const crumbs = [
    { name: "Home", path: "/" },
    { name: "Industries", path: "/industries" },
    ...ancestors.map((a) => ({ name: a.name, path: `/industries/${a.slug}` })),
    { name: category.name, path: `/industries/${category.slug}` },
  ];

  return (
    <div className="container-page space-y-8 py-8">
      <Breadcrumbs items={crumbs} />
      <header className="space-y-3">
        <h1 className="text-3xl font-bold">{category.name}</h1>
        {category.description ? (
          <p className="max-w-prose text-slate-700">{category.description}</p>
        ) : (
          <p className="max-w-prose text-slate-700">
            Businesses listed under {category.name.toLowerCase()} in India. Profiles appear here only after review and with permission to
            publish.
          </p>
        )}
      </header>

      {children.length > 0 && (
        <section aria-labelledby="sub-heading" className="space-y-3">
          <h2 id="sub-heading" className="text-xl font-semibold">
            Categories within {category.name}
          </h2>
          <ul className="flex flex-wrap gap-2">
            {children.map((c) => (
              <li key={c.id}>
                <Link href={`/industries/${c.slug}`} className="badge border-slate-300 bg-white px-3 py-1 text-sm text-slate-800 hover:border-accent-600">
                  {c.name}
                  {c.published_company_count ? <span className="ml-1 text-slate-600">({c.published_company_count})</span> : null}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      <section aria-labelledby="list-heading" className="space-y-4">
        <h2 id="list-heading" className="text-xl font-semibold">
          Published profiles
        </h2>
        {!companies.ok ? (
          <ApiUnavailableNotice what="profiles" />
        ) : companies.data.data.length === 0 ? (
          <EmptyNotice>
            No published profiles in this category yet.{" "}
            <Link href="/register-supplier" className="link">
              Register your business
            </Link>{" "}
            if it belongs here.
          </EmptyNotice>
        ) : (
          <>
            <CompanyGrid companies={companies.data.data} roleNames={roleNames} />
            <Pagination page={companies.data.page} basePath={`/industries/${category.slug}`} />
          </>
        )}
      </section>
    </div>
  );
}
