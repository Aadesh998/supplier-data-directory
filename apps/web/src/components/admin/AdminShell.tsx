"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { humanize } from "@/lib/labels";
import { useAuth } from "./AuthProvider";

const NAV = [
  { href: "/admin", label: "Dashboard" },
  { href: "/admin/submissions", label: "Submissions" },
  { href: "/admin/companies", label: "Companies" },
  { href: "/admin/imports", label: "Imports" },
  { href: "/admin/duplicates", label: "Duplicates" },
  { href: "/admin/taxonomy", label: "Taxonomy" },
  { href: "/admin/sources", label: "Data sources" },
  { href: "/admin/privacy-requests", label: "Privacy requests" },
  { href: "/admin/audit", label: "Audit log" },
  { href: "/admin/discovery", label: "Discovery" },
  { href: "/admin/exports", label: "Exports" },
];

const ROLE_HINT: Record<string, string> = {
  owner: "Owner — full access, including source permission flags and commercial exports.",
  admin: "Admin — can edit, import, publish and resolve requests. Source permission flags and commercial exports are owner-only.",
  reviewer: "Reviewer — can review and verify. Some actions (publishing, imports, exports) may be refused by the API.",
};

export function AdminShell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const isActive = (href: string) => (href === "/admin" ? pathname === "/admin" : pathname?.startsWith(href));

  return (
    <div className="min-h-screen bg-slate-100">
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white">
        <div className="flex h-14 items-center justify-between gap-3 px-4">
          <div className="flex items-center gap-3">
            <button type="button" className="btn-secondary btn-sm lg:hidden" aria-expanded={open} aria-controls="admin-nav" onClick={() => setOpen((v) => !v)}>
              Menu
            </button>
            <Link href="/admin" className="font-semibold text-slate-950">
              Admin console
            </Link>
            <span className="hidden text-xs text-slate-600 sm:inline">Private — not indexed</span>
          </div>
          <div className="flex items-center gap-3 text-sm">
            {user && (
              <span className="hidden text-slate-700 sm:inline" title={ROLE_HINT[user.role]}>
                {user.display_name || user.email} · <span className="badge border-slate-300 bg-slate-50">{humanize(user.role)}</span>
              </span>
            )}
            <Link href="/" className="btn-ghost btn-sm">
              Public site
            </Link>
            <button type="button" className="btn-secondary btn-sm" onClick={() => void logout()}>
              Sign out
            </button>
          </div>
        </div>
      </header>
      <div className="flex">
        <nav
          id="admin-nav"
          aria-label="Admin"
          className={`${open ? "block" : "hidden"} fixed inset-y-14 left-0 z-20 w-60 overflow-y-auto border-r border-slate-200 bg-white p-3 lg:sticky lg:top-14 lg:block lg:h-[calc(100vh-3.5rem)]`}
        >
          <ul className="space-y-1">
            {NAV.map((n) => (
              <li key={n.href}>
                <Link
                  href={n.href}
                  onClick={() => setOpen(false)}
                  aria-current={isActive(n.href) ? "page" : undefined}
                  className={`block rounded-md px-3 py-2 text-sm ${isActive(n.href) ? "bg-accent-50 font-semibold text-accent-800" : "text-slate-700 hover:bg-slate-100"}`}
                >
                  {n.label}
                </Link>
              </li>
            ))}
          </ul>
          {user && <p className="mt-6 px-3 text-xs text-slate-600">{ROLE_HINT[user.role]}</p>}
        </nav>
        <main id="main" tabIndex={-1} className="min-w-0 flex-1 p-4 sm:p-6">
          {children}
        </main>
      </div>
    </div>
  );
}
