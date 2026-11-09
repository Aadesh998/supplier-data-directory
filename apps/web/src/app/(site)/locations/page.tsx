import Link from "next/link";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { ApiUnavailableNotice } from "@/components/site/Notices";
import { pageMetadata } from "@/lib/seo";
import { getStates } from "@/lib/server-api";
import type { State } from "@/lib/types";

export async function generateMetadata() {
  return pageMetadata({
    title: "Manufacturers by State: All 36 States & UTs",
    description: "Find manufacturers and industrial suppliers in every Indian state and union territory, from Gujarat and Maharashtra to Tamil Nadu and Uttar Pradesh.",
    path: "/locations",
  });
}

function StateList({ states }: { states: State[] }) {
  return (
    <ul className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {states.map((s) => {
        const n = s.published_company_count ?? 0;
        return (
          <li key={s.code}>
            {n > 0 ? (
              <Link href={`/locations/${s.slug}`} className="card flex items-center justify-between py-3 hover:border-accent-200 hover:bg-accent-50">
                <span className="font-medium">{s.name}</span>
                <span className="text-sm text-slate-600">{n} listed</span>
              </Link>
            ) : (
              <div className="flex items-center justify-between rounded-lg border border-slate-200 bg-slate-50 px-5 py-3 text-slate-600">
                <span>{s.name}</span>
                <span className="text-xs">No listings yet</span>
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}

export default async function LocationsPage() {
  const res = await getStates();
  const states = res.ok ? res.data.data : [];
  const st = states.filter((s) => s.region_type === "state");
  const ut = states.filter((s) => s.region_type !== "state");
  return (
    <div className="container-page space-y-6 py-8">
      <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: "Locations", path: "/locations" }]} />
      <header className="space-y-2">
        <h1 className="text-3xl font-bold">Suppliers by state</h1>
        <p className="max-w-prose text-slate-700">
          State pages list businesses with a published profile and a facility in that state. States without published profiles are shown
          for completeness but have no page yet.
        </p>
      </header>
      {!res.ok ? (
        <ApiUnavailableNotice what="the list of states" />
      ) : (
        <>
          <section aria-labelledby="states-h" className="space-y-3">
            <h2 id="states-h" className="text-xl font-semibold">
              States
            </h2>
            <StateList states={st} />
          </section>
          {ut.length > 0 && (
            <section aria-labelledby="ut-h" className="space-y-3">
              <h2 id="ut-h" className="text-xl font-semibold">
                Union territories
              </h2>
              <StateList states={ut} />
            </section>
          )}
        </>
      )}
    </div>
  );
}
