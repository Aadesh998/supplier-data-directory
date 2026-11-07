import Link from "next/link";
import { notFound, permanentRedirect } from "next/navigation";
import { cache } from "react";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { JsonLd } from "@/components/site/JsonLd";
import { ApiUnavailableNotice, SyntheticBanner } from "@/components/site/Notices";
import { ProfileRemoved } from "@/components/site/ProfileRemoved";
import { isSynthetic } from "@/lib/constants";
import { absoluteUrl } from "@/lib/env";
import { capabilityLabel, certificationLabel, facilityLabel, formatDate, humanize, verificationLabel } from "@/lib/labels";
import { pageMetadata } from "@/lib/seo";
import { getPublicCompany, getRoles } from "@/lib/server-api";
import type { PublicCompany } from "@/lib/types";

type Props = { params: Promise<{ slug: string }> };

const lookup = cache((slug: string) => getPublicCompany(slug));

export async function generateMetadata({ params }: Props) {
  const { slug } = await params;
  const res = await lookup(slug);
  if (res.kind !== "ok") {
    return { title: res.kind === "gone" ? "Profile removed" : "Supplier profile", robots: { index: false, follow: false } };
  }
  const c = res.company;
  const where = [c.city, c.state_name].filter(Boolean).join(", ");
  const desc =
    c.description?.slice(0, 160) ||
    `${c.display_name}${where ? `, ${where}` : ""} — supplier profile in the Bharat Manufacturing Directory.`;
  return pageMetadata({
    title: where ? `${c.display_name} — ${where}` : c.display_name,
    description: desc,
    path: `/suppliers/${c.slug}`,
    ogType: "profile",
    // Synthetic demo records are never offered to search engines.
    noindex: isSynthetic(c.display_name),
  });
}

function organizationJsonLd(c: PublicCompany) {
  const locality = c.city ?? c.facilities?.find((f) => f.city)?.city;
  const region = c.state_name ?? c.facilities?.find((f) => f.state_name)?.state_name;
  // Public responses never include a street address or PIN, so a full postal
  // address is never available: we therefore always use Organization, not
  // LocalBusiness, and include only fields that are actually present.
  const data: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": "Organization",
    name: c.display_name,
    mainEntityOfPage: absoluteUrl(`/suppliers/${c.slug}`),
  };
  if (c.legal_name) data.legalName = c.legal_name;
  if (c.website_url) data.url = c.website_url;
  if (c.description) data.description = c.description;
  if (c.establishment_year) data.foundingDate = String(c.establishment_year);
  if (locality || region) {
    const address: Record<string, string> = { "@type": "PostalAddress", addressCountry: "IN" };
    if (locality) address.addressLocality = locality;
    if (region) address.addressRegion = region;
    data.address = address;
  }
  return data;
}

function Section({ title, id, children }: { title: string; id: string; children: React.ReactNode }) {
  return (
    <section aria-labelledby={id} className="card space-y-3">
      <h2 id={id} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

export default async function SupplierPage({ params }: Props) {
  const { slug } = await params;
  const res = await lookup(slug);
  if (res.kind === "redirect") permanentRedirect(`/suppliers/${encodeURIComponent(res.slug)}`);
  if (res.kind === "not_found") notFound();
  if (res.kind === "gone") return <ProfileRemoved />;
  if (res.kind === "error") {
    return (
      <div className="container-page py-8">
        <ApiUnavailableNotice what="this profile" />
      </div>
    );
  }

  const c = res.company;
  const roles = await getRoles();
  const roleNames: Record<string, string> = roles.ok ? Object.fromEntries(roles.data.data.map((r) => [r.code, r.name])) : {};
  const location = [c.city, c.state_name ?? c.state_code].filter(Boolean).join(", ");
  const primaryCategory = c.categories?.[0];
  const correctionHref = `/request-correction?company=${encodeURIComponent(c.slug)}`;

  const crumbs = [
    { name: "Home", path: "/" },
    ...(primaryCategory
      ? [
          { name: "Industries", path: "/industries" },
          { name: primaryCategory.name, path: `/industries/${primaryCategory.slug}` },
        ]
      : [{ name: "Search", path: "/search" }]),
    { name: c.display_name, path: `/suppliers/${c.slug}` },
  ];

  return (
    <div className="container-page space-y-6 py-8">
      <JsonLd data={organizationJsonLd(c)} />
      <Breadcrumbs items={crumbs} />
      {isSynthetic(c.display_name) && <SyntheticBanner />}

      <header className="space-y-3">
        <h1 className="text-3xl font-bold">{c.display_name}</h1>
        {location && <p className="text-lg text-slate-700">{location}</p>}
        {!!c.roles?.length && (
          <ul className="flex flex-wrap gap-2" aria-label="Business roles">
            {c.roles.map((r) => (
              <li key={r} className="badge border-slate-300 bg-white px-3 py-1 text-sm text-slate-800">
                {roleNames[r] ?? humanize(r)}
              </li>
            ))}
          </ul>
        )}
        <dl className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-slate-700">
          <div className="flex gap-1">
            <dt className="font-medium">Listing:</dt>
            <dd>Published after review</dd>
          </div>
          <div className="flex gap-1">
            <dt className="font-medium">Verification:</dt>
            <dd>
              <span
                className={
                  c.verification_status === "verified"
                    ? "badge border-emerald-300 bg-emerald-50 text-emerald-900"
                    : "badge border-slate-300 bg-slate-50 text-slate-800"
                }
              >
                {verificationLabel(c.verification_status)}
              </span>
            </dd>
          </div>
          {c.updated_at && (
            <div className="flex gap-1">
              <dt className="font-medium">Last updated:</dt>
              <dd>
                <time dateTime={c.updated_at}>{formatDate(c.updated_at)}</time>
              </dd>
            </div>
          )}
          {c.last_verified_at && (
            <div className="flex gap-1">
              <dt className="font-medium">Last verified:</dt>
              <dd>
                <time dateTime={c.last_verified_at}>{formatDate(c.last_verified_at)}</time>
              </dd>
            </div>
          )}
        </dl>
        <p className="text-xs text-slate-600">Publishing a profile does not mean the directory team has verified it.</p>
      </header>

      <div className="rounded-md border border-accent-200 bg-accent-50 px-4 py-3 text-sm">
        Is something wrong or out of date, or do you want this listing removed?{" "}
        <Link href={correctionHref} className="link font-semibold">
          Request a correction or removal
        </Link>
      </div>

      <div className="grid gap-6 lg:grid-cols-[2fr_1fr]">
        <div className="space-y-6">
          {c.description && (
            <Section title="About" id="about-h">
              <p className="whitespace-pre-line text-slate-800">{c.description}</p>
            </Section>
          )}

          {!!c.products?.length && (
            <Section title="Products" id="products-h">
              <ul className="divide-y divide-slate-100">
                {c.products.map((p, i) => (
                  <li key={`${p.name}-${i}`} className="py-2">
                    <p className="font-medium">{p.name}</p>
                    {p.description && <p className="text-sm text-slate-700">{p.description}</p>}
                    {p.category && (
                      <p className="text-xs text-slate-600">
                        Category:{" "}
                        <Link href={`/industries/${p.category.slug}`} className="link">
                          {p.category.name}
                        </Link>
                      </p>
                    )}
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {!!c.capabilities?.length && (
            <Section title="Capabilities" id="cap-h">
              <ul className="space-y-2">
                {c.capabilities.map((cap) => (
                  <li key={cap.slug} className="flex flex-wrap items-center justify-between gap-2">
                    <span>{cap.name}</span>
                    <span className="badge border-slate-300 bg-slate-50 text-slate-700">{capabilityLabel(cap.verification_status)}</span>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {!!c.certifications?.length && (
            <Section title="Certifications" id="cert-h">
              <ul className="space-y-2">
                {c.certifications.map((cert, i) => (
                  <li key={`${cert.certification_type}-${i}`} className="flex flex-wrap items-center justify-between gap-2">
                    <span>
                      {cert.certification_type}
                      {cert.issuer && <span className="text-sm text-slate-600"> — issued by {cert.issuer}</span>}
                    </span>
                    <span
                      className={
                        cert.status === "verified"
                          ? "badge border-emerald-300 bg-emerald-50 text-emerald-900"
                          : "badge border-slate-300 bg-slate-50 text-slate-700"
                      }
                    >
                      {certificationLabel(cert.status)}
                    </span>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {!!c.facilities?.length && (
            <Section title="Facilities" id="fac-h">
              <ul className="space-y-2">
                {c.facilities.map((f, i) => (
                  <li key={i}>
                    <span className="font-medium">{facilityLabel(f.facility_type)}</span>
                    <span className="text-slate-700">
                      {" — "}
                      {[f.locality, f.city, f.district, f.state_name ?? f.state_code].filter(Boolean).join(", ")}
                    </span>
                  </li>
                ))}
              </ul>
            </Section>
          )}
        </div>

        <aside className="space-y-6">
          {(c.legal_name || c.website_url || c.establishment_year) && (
            <Section title="Business details" id="facts-h">
              <dl className="space-y-2 text-sm">
                {c.legal_name && (
                  <div>
                    <dt className="text-slate-600">Legal name</dt>
                    <dd className="font-medium">{c.legal_name}</dd>
                  </div>
                )}
                {c.website_url && (
                  <div>
                    <dt className="text-slate-600">Website</dt>
                    <dd>
                      <a href={c.website_url} className="link break-all" rel="nofollow noopener noreferrer" target="_blank">
                        {c.website_url.replace(/^https?:\/\//, "")}
                      </a>
                    </dd>
                  </div>
                )}
                {c.establishment_year && (
                  <div>
                    <dt className="text-slate-600">Established</dt>
                    <dd className="font-medium">{c.establishment_year}</dd>
                  </div>
                )}
              </dl>
            </Section>
          )}

          {!!c.categories?.length && (
            <Section title="Categories" id="cats-h">
              <ul className="flex flex-wrap gap-2">
                {c.categories.map((cat) => (
                  <li key={cat.slug}>
                    <Link href={`/industries/${cat.slug}`} className="badge border-slate-300 bg-white px-3 py-1 text-sm text-slate-800 hover:border-accent-600">
                      {cat.name}
                    </Link>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {!!c.contacts?.length && (
            <Section title="Business contacts" id="contacts-h">
              <ul className="space-y-2 text-sm">
                {c.contacts.map((ct, i) => (
                  <li key={i}>
                    <span className="block text-slate-600">{ct.label ?? humanize(ct.contact_type)}</span>
                    {ct.channel === "email" ? (
                      <a href={`mailto:${ct.value}`} className="link break-all">
                        {ct.value}
                      </a>
                    ) : ct.channel === "phone" ? (
                      <a href={`tel:${ct.value.replace(/[^+\d]/g, "")}`} className="link">
                        {ct.value}
                      </a>
                    ) : (
                      <a href={ct.value} className="link break-all" rel="nofollow noopener noreferrer" target="_blank">
                        {ct.value}
                      </a>
                    )}
                  </li>
                ))}
              </ul>
              <p className="text-xs text-slate-600">Business contact points only. The directory does not publish personal contact details.</p>
            </Section>
          )}

          <Section title="Where this information comes from" id="src-h">
            {c.source_labels?.length ? (
              <ul className="list-disc space-y-1 pl-5 text-sm text-slate-800">
                {c.source_labels.map((s) => (
                  <li key={s}>{s}</li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-slate-700">Source details are not available for this profile.</p>
            )}
            {c.published_at && <p className="text-xs text-slate-600">First published {formatDate(c.published_at)}.</p>}
            <p className="text-sm">
              <Link href={correctionHref} className="link">
                Request a correction or removal
              </Link>
            </p>
          </Section>
        </aside>
      </div>
    </div>
  );
}
