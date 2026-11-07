import Link from "next/link";
import { ApiUnavailableNotice } from "@/components/site/Notices";
import { JsonLd } from "@/components/site/JsonLd";
import { SITE_NAME } from "@/lib/constants";
import { absoluteUrl } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";
import { getCategoryTree } from "@/lib/server-api";

export async function generateMetadata() {
  return pageMetadata({
    title: `${SITE_NAME} — find Indian manufacturers and industrial suppliers`,
    description:
      "A beta directory of Indian manufacturers, contract manufacturers, job-work providers and sourcing businesses. Profiles are published only after manual review and with permission.",
    path: "/",
  });
}

export default async function HomePage() {
  const tree = await getCategoryTree();
  const topLevel = tree.ok ? tree.data.data : [];
  const listedTotal = topLevel.reduce((sum, c) => sum + (c.published_company_count ?? 0), 0);

  return (
    <>
      <JsonLd
        data={[
          { "@context": "https://schema.org", "@type": "WebSite", name: SITE_NAME, url: absoluteUrl("/") },
          {
            "@context": "https://schema.org",
            "@type": "Organization",
            name: SITE_NAME,
            url: absoluteUrl("/"),
            description: "An India-focused directory of manufacturers and industrial suppliers, currently in beta.",
          },
        ]}
      />
      <section className="border-b border-slate-200 bg-white">
        <div className="container-page grid gap-8 py-12 md:grid-cols-[3fr_2fr] md:py-16">
          <div className="space-y-5">
            <p className="inline-flex rounded-full border border-amber-300 bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-900">
              Beta — we are building this directory carefully, one reviewed profile at a time
            </p>
            <h1 className="text-3xl font-bold tracking-tight sm:text-4xl">Find manufacturers and industrial suppliers across India</h1>
            <p className="max-w-prose text-lg text-slate-700">
              {SITE_NAME} lists manufacturers, contract manufacturers, job-work providers, exporters and sourcing businesses. Every public
              profile is reviewed by a person and published only with the business&apos;s permission or a documented right to display it.
            </p>
            <form action="/search" method="get" role="search" className="flex max-w-xl flex-col gap-2 sm:flex-row">
              <label htmlFor="home-q" className="sr-only">
                Search suppliers
              </label>
              <input
                id="home-q"
                name="q"
                type="search"
                className="input flex-1 py-3 text-base"
                placeholder="e.g. CNC machining, namkeen, vitrified tiles"
                maxLength={100}
              />
              <button type="submit" className="btn-primary py-3">
                Search
              </button>
            </form>
          </div>
          <aside className="card space-y-3 self-start bg-slate-50">
            <h2 className="text-lg font-semibold">Run a manufacturing business?</h2>
            <p className="text-sm text-slate-700">
              Add a free basic profile. We review every submission manually, nothing is made public without your permission, and you can
              ask us to correct or remove your details at any time.
            </p>
            <Link href="/register-supplier" className="btn-primary w-full">
              Register your business
            </Link>
            <p className="text-xs text-slate-600">
              Read <Link href="/data-policy" className="link">how we use submitted data</Link>.
            </p>
          </aside>
        </div>
      </section>

      <section className="container-page py-12" aria-labelledby="browse-heading">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-2">
          <h2 id="browse-heading" className="text-2xl font-semibold">
            Browse by industry
          </h2>
          {tree.ok && listedTotal > 0 && (
            <p className="text-sm text-slate-700">Category listing counts reflect currently published profiles.</p>
          )}
        </div>
        {!tree.ok ? (
          <ApiUnavailableNotice what="industry categories" />
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {topLevel.map((c) => (
              <li key={c.id}>
                <Link href={`/industries/${c.slug}`} className="card block h-full hover:border-accent-200 hover:bg-accent-50">
                  <span className="font-semibold text-slate-950">{c.name}</span>
                  {typeof c.published_company_count === "number" && c.published_company_count > 0 && (
                    <span className="ml-2 text-sm text-slate-600">
                      {c.published_company_count} listed
                    </span>
                  )}
                  {c.description && <span className="mt-1 block line-clamp-2 text-sm text-slate-700">{c.description}</span>}
                </Link>
              </li>
            ))}
          </ul>
        )}
        <p className="mt-4 text-sm">
          <Link href="/industries" className="link">
            See all industries
          </Link>{" "}
          ·{" "}
          <Link href="/locations" className="link">
            Browse by state
          </Link>
        </p>
      </section>

      <section className="border-t border-slate-200 bg-white" aria-labelledby="how-heading">
        <div className="container-page grid gap-6 py-12 md:grid-cols-3">
          <h2 id="how-heading" className="sr-only">
            How the directory works
          </h2>
          <div>
            <h3 className="font-semibold">Reviewed before publishing</h3>
            <p className="mt-1 text-sm text-slate-700">
              Submissions and imported records stay private until a reviewer approves them and publication is permitted.
            </p>
          </div>
          <div>
            <h3 className="font-semibold">Honest labels</h3>
            <p className="mt-1 text-sm text-slate-700">
              Profiles show where information came from and whether the directory team has verified it. Publishing a profile does not mean it
              has been verified.
            </p>
          </div>
          <div>
            <h3 className="font-semibold">Easy corrections</h3>
            <p className="mt-1 text-sm text-slate-700">
              Every profile links to a <Link href="/request-correction" className="link">correction and removal form</Link>.
            </p>
          </div>
        </div>
      </section>
    </>
  );
}
