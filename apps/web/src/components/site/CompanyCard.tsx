import Link from "next/link";
import { isSynthetic } from "@/lib/constants";
import { humanize, verificationLabel } from "@/lib/labels";
import type { PublicCompanySummary } from "@/lib/types";

export function CompanyCard({ company, roleNames }: { company: PublicCompanySummary; roleNames?: Record<string, string> }) {
  const location = [company.city, company.state_name ?? company.state_code].filter(Boolean).join(", ");
  return (
    <article className="card flex h-full flex-col gap-2">
      <h3 className="text-base font-semibold">
        <Link href={`/suppliers/${company.slug}`} className="text-slate-950 hover:text-accent-700 hover:underline">
          {company.display_name}
        </Link>
      </h3>
      {isSynthetic(company.display_name) && (
        <p className="text-xs font-semibold text-amber-900">Synthetic demo record — not a real business</p>
      )}
      {location && <p className="text-sm text-slate-700">{location}</p>}
      {company.description && <p className="line-clamp-3 text-sm text-slate-700">{company.description}</p>}
      {!!company.roles?.length && (
        <ul className="flex flex-wrap gap-1" aria-label="Business roles">
          {company.roles.map((r) => (
            <li key={r} className="badge border-slate-200 bg-slate-50 text-slate-700">
              {roleNames?.[r] ?? humanize(r)}
            </li>
          ))}
        </ul>
      )}
      {!!company.categories?.length && (
        <p className="text-xs text-slate-600">{company.categories.map((c) => c.name).join(" · ")}</p>
      )}
      <p className="mt-auto pt-2 text-xs text-slate-600">{verificationLabel(company.verification_status)}</p>
    </article>
  );
}

export function CompanyGrid({ companies, roleNames }: { companies: PublicCompanySummary[]; roleNames?: Record<string, string> }) {
  return (
    <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {companies.map((c) => (
        <li key={c.slug}>
          <CompanyCard company={c} roleNames={roleNames} />
        </li>
      ))}
    </ul>
  );
}
