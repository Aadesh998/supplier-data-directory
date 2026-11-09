import { LogoMark } from "@/components/ui/Logo";
import Link from "next/link";
import { SITE_NAME } from "@/lib/constants";
import { ThemeToggle } from "@/components/ui/ThemeToggle";
import { MobileNav } from "./MobileNav";

export const NAV_LINKS = [
  { href: "/industries", label: "Industries" },
  { href: "/locations", label: "Locations" },
  { href: "/search", label: "Search" },
];

export function SiteHeader() {
  return (
    <header className="relative border-b bg-background/95">
      <div className="container-page flex h-16 items-center justify-between gap-4">
        <Link href="/" className="flex items-center gap-2 font-semibold text-slate-950 no-underline">
          <LogoMark />
          <span className="leading-tight">
            {SITE_NAME}
            <span className="ml-2 rounded border border-amber-300 bg-amber-50 px-1.5 py-0.5 align-middle text-[10px] font-semibold tracking-wide text-amber-900 uppercase">
              Beta
            </span>
          </span>
        </Link>
        <nav aria-label="Main" className="hidden items-center gap-1 md:flex">
          {NAV_LINKS.map((l) => (
            <Link key={l.href} href={l.href} className="rounded-md px-3 py-2 text-sm font-medium text-muted-foreground no-underline hover:bg-accent hover:text-accent-foreground">
              {l.label}
            </Link>
          ))}
          <ThemeToggle className="ml-1" />
          <Link href="/register-supplier" className="btn-primary ml-1">
            Register your business
          </Link>
        </nav>
        <div className="flex items-center gap-1 md:hidden">
          <ThemeToggle />
          <MobileNav links={[...NAV_LINKS, { href: "/register-supplier", label: "Register your business" }]} />
        </div>
      </div>
    </header>
  );
}
