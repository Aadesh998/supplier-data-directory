import Link from "next/link";
import { ApiUnavailableNotice } from "@/components/site/Notices";
import { JsonLd } from "@/components/site/JsonLd";
import { LockIcon } from "@/components/site/Locked";
import { SITE_DESCRIPTION, SITE_NAME } from "@/lib/constants";
import { absoluteUrl } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";
import { getCategoryTree } from "@/lib/server-api";

export async function generateMetadata() {
  return {
    ...pageMetadata({
      title: `${SITE_NAME}: Find Manufacturers & Suppliers in India`,
      description:
        "Find verified manufacturers, contract manufacturers, job-work units and exporters across India. Search by product, industry cluster and state. Free supplier listing.",
      path: "/",
    }),
    // The home title is complete on its own; skip the "| Peoplvance" template.
    title: { absolute: `${SITE_NAME}: Find Manufacturers & Suppliers in India` },
  };
}

const POPULAR = [
  { label: "Vitrified tiles", href: "/industries/vitrified-tiles" },
  { label: "CNC machining", href: "/industries/cnc-machined-components" },
  { label: "Garments", href: "/industries/garment-manufacturing" },
  { label: "Namkeen & snacks", href: "/industries/namkeen-and-snacks" },
  { label: "Auto components", href: "/industries/auto-components" },
  { label: "Corrugated boxes", href: "/industries/corrugated-boxes" },
  { label: "Rugs & carpets", href: "/industries/rugs-and-carpets" },
  { label: "Job work", href: "/search?q=job+work" },
];

/** Well-known Indian manufacturing clusters. Facts are general public knowledge, kept deliberately conservative. */
const CLUSTERS = [
  { town: "Morbi", state: "Gujarat", stateSlug: "gujarat", known: "Ceramic and vitrified tiles, sanitaryware, wall clocks", href: "/industries/tiles" },
  { town: "Tiruppur", state: "Tamil Nadu", stateSlug: "tamil-nadu", known: "Knitwear, T-shirts and cotton garments for export", href: "/industries/garment-manufacturing" },
  { town: "Ludhiana", state: "Punjab", stateSlug: "punjab", known: "Hosiery, woollens, bicycle parts, hand tools and fasteners", href: "/industries/fasteners" },
  { town: "Rajkot", state: "Gujarat", stateSlug: "gujarat", known: "Engineering goods, diesel engines, pumps and castings", href: "/industries/foundries-and-castings" },
  { town: "Pune & Chakan", state: "Maharashtra", stateSlug: "maharashtra", known: "Auto components, CNC machining and precision engineering", href: "/industries/auto-components" },
  { town: "Coimbatore", state: "Tamil Nadu", stateSlug: "tamil-nadu", known: "Pumps, motors, textile machinery and foundries", href: "/industries/pumps-and-motors" },
  { town: "Bhadohi & Mirzapur", state: "Uttar Pradesh", stateSlug: "uttar-pradesh", known: "Hand-knotted and hand-tufted carpets and rugs", href: "/industries/rugs-and-carpets" },
  { town: "Moradabad", state: "Uttar Pradesh", stateSlug: "uttar-pradesh", known: "Brassware, metal handicrafts and home décor", href: "/industries/brass-and-copper" },
  { town: "Jamnagar", state: "Gujarat", stateSlug: "gujarat", known: "Brass parts, fittings and turned components", href: "/industries/brass-and-copper" },
  { town: "Firozabad", state: "Uttar Pradesh", stateSlug: "uttar-pradesh", known: "Glassware and glass bangles", href: "/industries/glass" },
  { town: "Panipat", state: "Haryana", stateSlug: "haryana", known: "Home textiles, blankets and recycled yarn", href: "/industries/home-textiles" },
  { town: "Baddi", state: "Himachal Pradesh", stateSlug: "himachal-pradesh", known: "Pharmaceutical formulations and contract manufacturing", href: "/industries/pharmaceuticals" },
];

const PUBLIC_VS_FULL = [
  { item: "Company name, city and state", open: true },
  { item: "Industry categories and business roles", open: true },
  { item: "Key products and capabilities", open: true },
  { item: "Certification types and verification status", open: true },
  { item: "Phone, email and other contact points", open: false },
  { item: "Website and registered legal name", open: false },
  { item: "Exact locality and facility details", open: false },
  { item: "Full product catalogue and certificate issuers", open: false },
];

const FAQ = [
  {
    q: "What is Peoplvance?",
    a: "Peoplvance is a B2B directory of manufacturers and industrial suppliers in India. It covers manufacturers, contract manufacturers (OEM/ODM), job-work units, exporters, wholesalers and sourcing agents, organised by industry, product and state.",
  },
  {
    q: "How is Peoplvance different from IndiaMART or Tradeindia?",
    a: "Peoplvance is not a lead-selling marketplace. Every profile is reviewed by a person before it is published. Every field records where it came from, and a business is listed publicly only with its permission or a documented right to display it. Profiles say plainly whether a business has been verified.",
  },
  {
    q: "Why are contact details hidden on public profiles?",
    a: "To protect suppliers from spam and scraping. Anyone can see what a business makes and where it is based. Contact details, website and exact location are shared with genuine buyers on request, with the supplier's permission.",
  },
  {
    q: "Is listing my factory free?",
    a: "Yes. A basic profile is free. Fill in the registration form; we review it manually and publish it only if you allow public listing. You can ask for corrections or removal at any time.",
  },
  {
    q: "What does 'verified' mean on a profile?",
    a: "Verified means our team has checked the business, for example by a call or a document check, and recorded the date. Published but unverified profiles are labelled clearly. Publishing alone never implies verification.",
  },
  {
    q: "Which industries and states do you cover?",
    a: "All 36 Indian states and union territories, across 14 sectors: food processing, textiles and apparel, building materials, metals and fabrication, industrial machinery, plastics and packaging, electrical and electronics, automotive, chemicals and pharma, furniture and paper, leather and handicrafts, renewables, industrial services, and sourcing and trading.",
  },
];

export default async function HomePage() {
  const tree = await getCategoryTree();
  const topLevel = tree.ok ? tree.data.data : [];
  const listedTotal = topLevel.reduce((sum, c) => sum + (c.published_company_count ?? 0), 0);
  const categoryCount = topLevel.reduce((sum, c) => sum + 1 + (c.children?.length ?? 0), 0);

  return (
    <>
      <JsonLd
        data={[
          {
            "@context": "https://schema.org",
            "@type": "WebSite",
            name: SITE_NAME,
            alternateName: "peoplvance.org",
            url: absoluteUrl("/"),
            inLanguage: "en-IN",
          },
          {
            "@context": "https://schema.org",
            "@type": "Organization",
            name: SITE_NAME,
            url: absoluteUrl("/"),
            logo: absoluteUrl("/icon.svg"),
            description: SITE_DESCRIPTION,
            areaServed: { "@type": "Country", name: "India" },
          },
        ]}
      />

      {/* Hero */}
      <section className="border-b border-slate-200">
        <div className="container-page grid gap-10 py-14 md:grid-cols-[3fr_2fr] md:py-20">
          <div className="space-y-6">
            <p className="eyebrow">India&apos;s reviewed B2B supplier directory</p>
            <h1 className="text-4xl font-bold tracking-tight text-slate-950 sm:text-5xl">
              Find manufacturers and suppliers across India, <span className="text-accent-700">checked by people.</span>
            </h1>
            <p className="max-w-prose text-lg text-slate-700">
              Search manufacturers, contract manufacturers, job-work units and exporters by product, industry cluster and state. Every
              profile is reviewed before it goes live and shows where its information came from.
            </p>
            <form action="/search" method="get" role="search" className="flex max-w-xl flex-col gap-2 sm:flex-row">
              <label htmlFor="home-q" className="sr-only">
                Search manufacturers and suppliers
              </label>
              <input
                id="home-q"
                name="q"
                type="search"
                className="input flex-1 py-3 text-base"
                placeholder="Try: vitrified tiles, CNC turning, cotton T-shirts"
                maxLength={100}
              />
              <button type="submit" className="btn-primary py-3">
                Search suppliers
              </button>
            </form>
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <span className="text-slate-600">Popular:</span>
              {POPULAR.map((p) => (
                <Link
                  key={p.href}
                  href={p.href}
                  className="rounded-full border border-slate-300 bg-white px-3 py-1 text-slate-800 hover:border-accent-600 hover:text-accent-800"
                >
                  {p.label}
                </Link>
              ))}
            </div>
          </div>

          <aside className="card space-y-4 self-start">
            <h2 className="text-lg font-semibold">Run a factory or supply business?</h2>
            <p className="text-sm text-slate-700">
              Get found by buyers searching for what you make. A basic profile is free, nothing goes public without your permission, and
              you can correct or remove your details at any time.
            </p>
            <ul className="space-y-1 text-sm text-slate-800">
              <li>✓ Free listing, reviewed within days</li>
              <li>✓ Your contact details stay protected from scrapers</li>
              <li>✓ Show your products, capabilities and certifications</li>
            </ul>
            <Link href="/register-supplier" className="btn-primary w-full">
              List your business free
            </Link>
          </aside>
        </div>
      </section>

      {/* Stats */}
      <section aria-label="Directory coverage" className="border-b border-slate-200 bg-slate-100">
        <dl className="container-page grid grid-cols-2 gap-6 py-8 text-center md:grid-cols-4">
          {[
            { k: "States & UTs covered", v: "36" },
            { k: "Industry sectors", v: tree.ok ? String(topLevel.length) : "14" },
            { k: "Product categories", v: tree.ok && categoryCount ? String(categoryCount) : "90+" },
            { k: "Reviewed profiles", v: listedTotal > 0 ? listedTotal.toLocaleString("en-IN") : "Growing daily" },
          ].map((s) => (
            <div key={s.k}>
              <dt className="text-xs font-medium tracking-wide text-slate-600 uppercase">{s.k}</dt>
              <dd className="mt-1 text-2xl font-bold text-slate-950">{s.v}</dd>
            </div>
          ))}
        </dl>
      </section>

      {/* Industries */}
      <section className="container-page py-14" aria-labelledby="browse-heading">
        <div className="mb-6 flex flex-wrap items-end justify-between gap-2">
          <div>
            <p className="eyebrow">Browse</p>
            <h2 id="browse-heading" className="text-2xl font-bold sm:text-3xl">
              Manufacturers by industry
            </h2>
          </div>
          <Link href="/industries" className="link text-sm font-medium">
            See all industries →
          </Link>
        </div>
        {!tree.ok ? (
          <ApiUnavailableNotice what="industry categories" />
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {topLevel.map((c) => (
              <li key={c.id}>
                <Link href={`/industries/${c.slug}`} className="card block h-full hover:border-accent-300">
                  <span className="font-semibold text-slate-950">{c.name}</span>
                  {typeof c.published_company_count === "number" && c.published_company_count > 0 && (
                    <span className="ml-2 text-sm text-slate-600">{c.published_company_count} listed</span>
                  )}
                  {!!c.children?.length && (
                    <span className="mt-1 block line-clamp-2 text-sm text-slate-700">
                      {c.children
                        .slice(0, 5)
                        .map((ch) => ch.name)
                        .join(" · ")}
                    </span>
                  )}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>

      {/* Clusters */}
      <section className="border-y border-slate-200 bg-white" aria-labelledby="clusters-heading">
        <div className="container-page py-14">
          <p className="eyebrow">Source where the industry is</p>
          <h2 id="clusters-heading" className="text-2xl font-bold sm:text-3xl">
            India&apos;s manufacturing clusters
          </h2>
          <p className="mt-2 max-w-3xl text-slate-700">
            Much of Indian manufacturing is concentrated in specialised towns, where hundreds of factories, job-work units and raw-material
            suppliers sit within a few kilometres of each other. Sourcing from a cluster usually means better prices, faster sampling and
            easier factory visits.
          </p>
          <ul className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {CLUSTERS.map((c) => (
              <li key={c.town} className="rounded-lg border border-slate-200 bg-slate-50 p-4">
                <p className="font-semibold text-slate-950">
                  {c.town}
                  <span className="font-normal text-slate-600">
                    ,{" "}
                    <Link href={`/locations/${c.stateSlug}`} className="link">
                      {c.state}
                    </Link>
                  </span>
                </p>
                <p className="mt-1 text-sm text-slate-700">{c.known}</p>
                <Link href={c.href} className="mt-2 inline-block text-sm font-medium text-accent-700 hover:underline">
                  Find suppliers →
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </section>

      {/* How it works */}
      <section className="container-page grid gap-10 py-14 md:grid-cols-2" aria-labelledby="how-heading">
        <h2 id="how-heading" className="sr-only">
          How Peoplvance works
        </h2>
        <div>
          <p className="eyebrow">For buyers</p>
          <h3 className="mt-1 text-xl font-bold">Shortlist suppliers in minutes</h3>
          <ol className="mt-4 space-y-4">
            {[
              ["Search or browse", "Filter by product, industry, state and business role: manufacturer, OEM, job work or exporter."],
              ["Compare profiles", "See products, capabilities, certifications, verification status and where the information came from."],
              ["Request full details", "Ask for contact details and the full profile. We share them with the supplier's permission."],
            ].map(([t, d], i) => (
              <li key={t} className="flex gap-3">
                <span className="grid size-8 shrink-0 place-items-center rounded-full bg-primary text-sm font-bold text-primary-foreground">{i + 1}</span>
                <span>
                  <span className="block font-semibold">{t}</span>
                  <span className="text-sm text-slate-700">{d}</span>
                </span>
              </li>
            ))}
          </ol>
        </div>
        <div>
          <p className="eyebrow">For suppliers</p>
          <h3 className="mt-1 text-xl font-bold">Get found by serious buyers</h3>
          <ol className="mt-4 space-y-4">
            {[
              ["Register free", "A short 6-step form covering products, capabilities, facilities and certifications."],
              ["We review it", "A person checks every submission. Nothing is published without your separate permission."],
              ["Stay in control", "Your contact details are never shown openly. Ask us to update or remove your profile any time."],
            ].map(([t, d], i) => (
              <li key={t} className="flex gap-3">
                <span className="grid size-8 shrink-0 place-items-center rounded-full bg-foreground text-sm font-bold text-background">{i + 1}</span>
                <span>
                  <span className="block font-semibold">{t}</span>
                  <span className="text-sm text-slate-700">{d}</span>
                </span>
              </li>
            ))}
          </ol>
        </div>
      </section>

      {/* Public vs full */}
      <section className="border-y border-slate-200 bg-slate-100" aria-labelledby="access-heading">
        <div className="container-page grid gap-8 py-14 md:grid-cols-[2fr_3fr] md:items-center">
          <div className="space-y-3">
            <p className="eyebrow">Open profiles, protected contacts</p>
            <h2 id="access-heading" className="text-2xl font-bold sm:text-3xl">
              See what a supplier makes. Unlock how to reach them.
            </h2>
            <p className="text-slate-700">
              Public profiles give you enough to judge fit. The details scrapers and spammers want stay locked until a genuine buyer asks.
            </p>
            <Link href="/contact?topic=access" className="btn-primary">
              <LockIcon className="h-4 w-4" /> Request full supplier details
            </Link>
          </div>
          <div className="table-wrap">
            <table className="table min-w-0">
              <thead>
                <tr>
                  <th scope="col">Information</th>
                  <th scope="col" className="text-center">
                    Public profile
                  </th>
                  <th scope="col" className="text-center">
                    Full profile
                  </th>
                </tr>
              </thead>
              <tbody>
                {PUBLIC_VS_FULL.map((r) => (
                  <tr key={r.item}>
                    <td>{r.item}</td>
                    <td className="text-center">
                      {r.open ? (
                        <span className="font-semibold text-accent-700">✓<span className="sr-only"> included</span></span>
                      ) : (
                        <span className="inline-flex text-slate-500">
                          <LockIcon />
                          <span className="sr-only">locked</span>
                        </span>
                      )}
                    </td>
                    <td className="text-center font-semibold text-accent-700">
                      ✓<span className="sr-only"> included</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* Trust */}
      <section className="container-page py-14" aria-labelledby="trust-heading">
        <p className="eyebrow">Why Peoplvance</p>
        <h2 id="trust-heading" className="text-2xl font-bold sm:text-3xl">
          A directory you can trust
        </h2>
        <div className="mt-8 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {[
            ["Reviewed before publishing", "Submissions and imported records stay private until a reviewer approves them."],
            ["Source on every field", "Each detail records where it came from and what it may be used for."],
            ["Honest verification labels", "Profiles show whether, and when, our team verified the business. Published never means verified."],
            ["Easy corrections", "Every profile links to a correction and removal form, handled by a person."],
          ].map(([t, d]) => (
            <div key={t} className="border-t-2 border-accent-700 pt-4">
              <h3 className="font-semibold">{t}</h3>
              <p className="mt-1 text-sm text-slate-700">{d}</p>
            </div>
          ))}
        </div>
      </section>

      {/* FAQ: visible content only (no FAQPage markup; Google retired FAQ rich results). */}
      <section className="border-t border-slate-200 bg-white" aria-labelledby="faq-heading">
        <div className="container-page max-w-3xl py-14">
          <p className="eyebrow">Questions</p>
          <h2 id="faq-heading" className="text-2xl font-bold sm:text-3xl">
            Frequently asked questions
          </h2>
          <div className="mt-6 divide-y divide-slate-200 border-y border-slate-200">
            {FAQ.map((f) => (
              <details key={f.q} className="group py-4">
                <summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-semibold text-slate-950">
                  {f.q}
                  <span aria-hidden="true" className="text-accent-700 transition-transform group-open:rotate-45">
                    +
                  </span>
                </summary>
                <p className="mt-2 text-slate-700">{f.a}</p>
              </details>
            ))}
          </div>
        </div>
      </section>

      {/* Final CTA */}
      <section className="bg-zinc-950 text-zinc-50 dark:border-y dark:bg-card">
        <div className="container-page flex flex-col items-start justify-between gap-6 py-12 md:flex-row md:items-center">
          <div>
            <h2 className="text-2xl font-bold tracking-tight text-zinc-50">Are you a manufacturer, job-work unit or exporter?</h2>
            <p className="mt-1 text-zinc-300">Add your business to Peoplvance free and reach buyers looking for what you make.</p>
          </div>
          <div className="flex gap-3">
            <Link href="/register-supplier" className="btn bg-zinc-50 text-zinc-950 hover:bg-zinc-200">
              List your business free
            </Link>
            <Link href="/search" className="btn border border-zinc-600 text-zinc-50 hover:bg-zinc-800">
              Find suppliers
            </Link>
          </div>
        </div>
      </section>
    </>
  );
}
