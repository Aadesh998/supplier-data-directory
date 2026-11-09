import Link from "next/link";
import { SITE_NAME } from "@/lib/constants";

const BROWSE = [
  { href: "/industries", label: "All industries" },
  { href: "/locations", label: "Manufacturers by state" },
  { href: "/industries/tiles", label: "Tile manufacturers" },
  { href: "/industries/garment-manufacturing", label: "Garment manufacturers" },
  { href: "/industries/cnc-machined-components", label: "CNC machining" },
  { href: "/industries/auto-components", label: "Auto components" },
  { href: "/industries/namkeen-and-snacks", label: "Namkeen manufacturers" },
  { href: "/industries/pharmaceuticals", label: "Pharma manufacturers" },
  { href: "/locations/gujarat", label: "Manufacturers in Gujarat" },
  { href: "/locations/maharashtra", label: "Manufacturers in Maharashtra" },
  { href: "/locations/tamil-nadu", label: "Manufacturers in Tamil Nadu" },
  { href: "/register-supplier", label: "List your business free" },
];

const LEGAL = [
  { href: "/about", label: "About" },
  { href: "/contact", label: "Contact" },
  { href: "/privacy", label: "Privacy notice" },
  { href: "/data-policy", label: "Data policy" },
  { href: "/terms", label: "Terms" },
  { href: "/request-correction", label: "Request a correction or removal" },
];

export function SiteFooter() {
  return (
    <footer className="mt-16 border-t border-slate-200 bg-slate-100">
      <div className="container-page grid gap-8 py-10 md:grid-cols-[2fr_2fr_2fr]">
        <div className="space-y-2 text-sm text-slate-700">
          <p className="font-semibold text-slate-900">{SITE_NAME}</p>
          <p>
            A B2B directory of Indian manufacturers, contract manufacturers, job-work units and exporters. Listings are published only after
            manual review and with the business&apos;s permission or a documented right to display. A listing does not mean the business has
            been verified unless the profile says so.
          </p>
          <p className="text-xs text-slate-600">Beta · peoplvance.org</p>
        </div>
        <nav aria-label="Browse">
          <p className="mb-2 text-sm font-semibold text-slate-900">Browse</p>
          <ul className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            {BROWSE.map((l) => (
              <li key={l.href}>
                <Link href={l.href} className="link">
                  {l.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        <nav aria-label="Legal and help">
          <p className="mb-2 text-sm font-semibold text-slate-900">Company</p>
          <ul className="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            {LEGAL.map((l) => (
              <li key={l.href}>
                <Link href={l.href} className="link">
                  {l.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <div className="border-t border-slate-100 py-4 text-center text-xs text-slate-600">
        &copy; {new Date().getFullYear()} {SITE_NAME}. Not affiliated with any government body.
      </div>
    </footer>
  );
}
