import Link from "next/link";
import { SITE_NAME } from "@/lib/constants";

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
    <footer className="mt-16 border-t border-slate-200 bg-white">
      <div className="container-page grid gap-6 py-10 md:grid-cols-[2fr_3fr]">
        <div className="space-y-2 text-sm text-slate-700">
          <p className="font-semibold text-slate-900">{SITE_NAME}</p>
          <p>
            <strong className="font-semibold">Beta.</strong> This directory is in early testing. Listings are published only after manual
            review and with the business&apos;s permission or a documented right to display. A listing does not mean the business has been
            verified unless the profile says so.
          </p>
        </div>
        <nav aria-label="Legal and help">
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
        &copy; {new Date().getFullYear()} {SITE_NAME} (working name). Not affiliated with any government body.
      </div>
    </footer>
  );
}
