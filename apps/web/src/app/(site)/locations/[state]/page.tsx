import Link from "next/link";
import { notFound } from "next/navigation";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { CompanyGrid } from "@/components/site/CompanyCard";
import { ApiUnavailableNotice, EmptyNotice } from "@/components/site/Notices";
import { Pagination, parsePage } from "@/components/site/Pagination";
import { DEFAULT_PAGE_SIZE } from "@/lib/constants";
import { pagedPath, pageMetadata } from "@/lib/seo";
import { getRoles, getStates, searchCompanies } from "@/lib/server-api";

type Props = { params: Promise<{ state: string }>; searchParams: Promise<Record<string, string | string[] | undefined>> };

const THIN_THRESHOLD = 3;

async function findState(slug: string) {
  const res = await getStates();
  if (!res.ok) return { ok: false as const };
  return { ok: true as const, state: res.data.data.find((s) => s.slug === slug) };
}

export async function generateMetadata({ params, searchParams }: Props) {
  const { state: slug } = await params;
  const page = parsePage((await searchParams).page);
  const found = await findState(slug);
  if (!found.ok || !found.state) return { title: "Location", robots: { index: false, follow: false } };
  const s = found.state;
  return pageMetadata({
    title: page > 1 ? `Manufacturers & Suppliers in ${s.name} (page ${page})` : `Manufacturers & Suppliers in ${s.name}`,
    description: `Find reviewed manufacturers, industrial suppliers, exporters and job-work units in ${s.name}. Browse by industry, city and capability on Peoplvance.`,
    path: pagedPath(`/locations/${s.slug}`, page),
    noindex: (s.published_company_count ?? 0) < THIN_THRESHOLD,
  });
}

export default async function StatePage({ params, searchParams }: Props) {
  const { state: slug } = await params;
  const page = parsePage((await searchParams).page);
  const found = await findState(slug);
  if (!found.ok) {
    return (
      <div className="container-page py-8">
        <ApiUnavailableNotice what="this location" />
      </div>
    );
  }
  if (!found.state) notFound();
  const s = found.state;
  const [companies, roles] = await Promise.all([searchCompanies({ state: s.slug, page, limit: DEFAULT_PAGE_SIZE }), getRoles()]);
  const roleNames = roles.ok ? Object.fromEntries(roles.data.data.map((r) => [r.code, r.name])) : undefined;

  return (
    <div className="container-page space-y-8 py-8">
      <Breadcrumbs
        items={[
          { name: "Home", path: "/" },
          { name: "Locations", path: "/locations" },
          { name: s.name, path: `/locations/${s.slug}` },
        ]}
      />
      <header className="space-y-2">
        <h1 className="text-3xl font-bold">Manufacturers and suppliers in {s.name}</h1>
        <p className="max-w-prose text-slate-700">
          Businesses with a published profile and at least one listed facility in {s.name}. Use{" "}
          <Link href={`/search?state=${encodeURIComponent(s.slug)}`} className="link">
            search
          </Link>{" "}
          to filter by category, city or business role.
        </p>
      </header>
      {!companies.ok ? (
        <ApiUnavailableNotice what="profiles" />
      ) : companies.data.data.length === 0 ? (
        <EmptyNotice>No published profiles in {s.name} yet.</EmptyNotice>
      ) : (
        <>
          <CompanyGrid companies={companies.data.data} roleNames={roleNames} />
          <Pagination page={companies.data.page} basePath={`/locations/${s.slug}`} />
        </>
      )}
    </div>
  );
}
