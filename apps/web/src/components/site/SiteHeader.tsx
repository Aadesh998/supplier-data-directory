import Link from "next/link";
import { SITE_NAME } from "@/lib/constants";
import { MobileNav } from "./MobileNav";

export const NAV_LINKS = [
  { href: "/industries", label: "Industries" },
  { href: "/locations", label: "Locations" },
  { href: "/search", label: "Search" },
];

export function SiteHeader() {
  return (
    <header className="relative border-b border-slate-200 bg-white">
      <div className="container-page flex h-16 items-center justify-between gap-4">
        <Link href="/" className="flex items-center gap-2 font-semibold text-slate-950 no-underline">
          <span aria-hidden="true" className="grid size-8 place-items-center rounded-md bg-accent-700 text-sm font-bold text-white">
            BM
          </span>
          <span className="leading-tight">
            {SITE_NAME}
            <span className="ml-2 rounded border border-amber-300 bg-amber-50 px-1.5 py-0.5 align-middle text-[10px] font-semibold tracking-wide text-amber-900 uppercase">
              Beta
            </span>
          </span>
        </Link>
        <nav aria-label="Main" className="hidden items-center gap-1 md:flex">
          {NAV_LINKS.map((l) => (
            <Link key={l.href} href={l.href} className="rounded-md px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-100 hover:text-slate-950">
              {l.label}
            </Link>
          ))}
          <Link href="/register-supplier" className="btn-primary ml-2">
            Register your business
          </Link>
        </nav>
        <MobileNav links={[...NAV_LINKS, { href: "/register-supplier", label: "Register your business" }]} />
      </div>
    </header>
  );
}
