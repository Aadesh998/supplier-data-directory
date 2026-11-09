"use client";

import {
  Building2,
  Copy,
  Database,
  Download,
  ExternalLink,
  Inbox,
  LayoutDashboard,
  ListChecks,
  LogOut,
  MapPinned,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  ScrollText,
  Search,
  ShieldCheck,
  Tags,
  Upload,
  Users,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { LogoMark } from "@/components/ui/Logo";
import { ThemeToggle } from "@/components/ui/ThemeToggle";
import { SITE_NAME } from "@/lib/constants";
import { humanize } from "@/lib/labels";
import { useLocalStorage } from "@/lib/use-local-storage";
import { useAuth } from "./AuthProvider";

type NavItem = { href: string; label: string; icon: LucideIcon; ownerOnly?: boolean };

const NAV: { title: string; items: NavItem[] }[] = [
  {
    title: "Main",
    items: [
      { href: "/admin", label: "Dashboard", icon: LayoutDashboard },
      { href: "/admin/submissions", label: "Submissions", icon: Inbox },
      { href: "/admin/companies", label: "Companies", icon: Building2 },
    ],
  },
  {
    title: "Data",
    items: [
      { href: "/admin/imports", label: "Imports", icon: Upload },
      { href: "/admin/duplicates", label: "Duplicates", icon: Copy },
      { href: "/admin/sources", label: "Data sources", icon: Database },
      { href: "/admin/exports", label: "Exports", icon: Download },
    ],
  },
  {
    title: "Discovery",
    items: [
      { href: "/admin/discovery/jobs", label: "Discovery jobs", icon: ListChecks },
      { href: "/admin/discovery/results", label: "Discovery results", icon: MapPinned },
      { href: "/admin/discovery", label: "Places lookup", icon: Search },
    ],
  },
  {
    title: "Admin",
    items: [
      { href: "/admin/taxonomy", label: "Taxonomy", icon: Tags },
      { href: "/admin/privacy-requests", label: "Privacy requests", icon: ShieldCheck },
      { href: "/admin/audit", label: "Audit log", icon: ScrollText },
      { href: "/admin/users", label: "Users", icon: Users, ownerOnly: true },
    ],
  },
];

const ROLE_HINT: Record<string, string> = {
  owner: "Owner — full access, including source permission flags and commercial exports.",
  admin: "Admin — can edit, import, publish and resolve requests. Source permission flags and commercial exports are owner-only.",
  reviewer: "Reviewer — can review and verify. Some actions (publishing, imports, exports) may be refused by the API.",
};

const COLLAPSE_KEY = "admin-sidebar-collapsed";

function initials(name: string) {
  const parts = name.replace(/@.*/, "").split(/[\s._-]+/).filter(Boolean);
  return ((parts[0]?.[0] ?? "?") + (parts[1]?.[0] ?? "")).toUpperCase();
}

export function AdminShell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [collapsedValue, setCollapsedValue] = useLocalStorage(COLLAPSE_KEY);
  const collapsed = collapsedValue === "1";
  const toggleCollapsed = () => setCollapsedValue(collapsed ? null : "1");

  const isActive = (href: string) =>
    href === "/admin" || href === "/admin/discovery" ? pathname === href : !!pathname?.startsWith(href);
  // Collapsed (icon-only) applies on desktop only; the mobile drawer always shows labels.
  const iconOnly = collapsed && !mobileOpen;
  const name = user ? user.display_name || user.email : "";

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background">
      {mobileOpen && <div className="fixed inset-0 z-30 bg-black/50 lg:hidden" aria-hidden="true" onClick={() => setMobileOpen(false)} />}

      <aside
        id="admin-nav"
        aria-label="Admin"
        className={`fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width,transform] duration-200 lg:static lg:translate-x-0 ${
          mobileOpen ? "translate-x-0" : "-translate-x-full"
        } ${iconOnly ? "lg:w-[72px]" : "lg:w-64"}`}
      >
        {/* Header: brand tile + name */}
        <div className={`flex h-14 shrink-0 items-center gap-2 px-3 ${iconOnly ? "lg:justify-center lg:px-0" : ""}`}>
          <Link href="/admin" className="flex min-w-0 items-center gap-2 no-underline" title={`${SITE_NAME} admin`}>
            <LogoMark />
            <span className={`min-w-0 leading-tight ${iconOnly ? "lg:hidden" : ""}`}>
              <span className="block truncate text-sm font-semibold">{SITE_NAME}</span>
              <span className="block text-xs text-muted-foreground">Admin · private, not indexed</span>
            </span>
          </Link>
          <button type="button" className="btn-ghost btn-sm ml-auto lg:hidden" aria-label="Close menu" onClick={() => setMobileOpen(false)}>
            <X aria-hidden="true" className="size-4" />
          </button>
        </div>

        {/* Content: grouped nav */}
        <nav className="flex-1 space-y-4 overflow-y-auto px-3 py-2">
          {NAV.map((group) => {
            const items = group.items.filter((n) => !n.ownerOnly || user?.role === "owner");
            if (items.length === 0) return null;
            return (
              <div key={group.title}>
                <p className={`mb-1 px-2 text-xs font-medium text-muted-foreground ${iconOnly ? "lg:sr-only" : ""}`}>{group.title}</p>
                <ul className="space-y-0.5">
                  {items.map((n) => {
                    const active = isActive(n.href);
                    return (
                      <li key={n.href}>
                        <Link
                          href={n.href}
                          aria-current={active ? "page" : undefined}
                          data-active={active}
                          title={iconOnly ? n.label : undefined}
                          onClick={() => setMobileOpen(false)}
                          className={`flex h-9 items-center gap-2 rounded-md px-2 text-sm no-underline transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground ${
                            iconOnly ? "lg:justify-center lg:px-0" : ""
                          }`}
                        >
                          <n.icon aria-hidden="true" className={`size-4 shrink-0 ${active ? "text-sidebar-primary" : ""}`} />
                          <span className={`truncate ${iconOnly ? "lg:sr-only" : ""}`}>{n.label}</span>
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              </div>
            );
          })}
        </nav>

        {/* Footer: theme, collapse, user */}
        <div className="shrink-0 space-y-2 border-t border-sidebar-border p-3">
          <div className={`flex items-center gap-1 ${iconOnly ? "lg:flex-col" : ""}`}>
            <ThemeToggle showLabel={!iconOnly} />
            <button
              type="button"
              className="btn-ghost btn-sm ml-auto hidden lg:inline-flex"
              onClick={toggleCollapsed}
              aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
              title={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {collapsed ? <PanelLeftOpen aria-hidden="true" className="size-4" /> : <PanelLeftClose aria-hidden="true" className="size-4" />}
            </button>
          </div>
          {user && (
            <div className={`flex items-center gap-2 rounded-md p-1 ${iconOnly ? "lg:flex-col" : ""}`} title={ROLE_HINT[user.role]}>
              <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-semibold">
                {initials(name)}
              </span>
              <span className={`min-w-0 flex-1 leading-tight ${iconOnly ? "lg:sr-only" : ""}`}>
                <span className="block truncate text-sm font-medium">{name}</span>
                <span className="block text-xs text-muted-foreground">{humanize(user.role)}</span>
              </span>
              <Link href="/" className="btn-ghost btn-sm px-2" title="Public site" aria-label="Public site">
                <ExternalLink aria-hidden="true" className="size-4" />
              </Link>
              <button type="button" className="btn-ghost btn-sm px-2" onClick={() => void logout()} title="Sign out" aria-label="Sign out">
                <LogOut aria-hidden="true" className="size-4" />
              </button>
            </div>
          )}
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Mobile only: the sidebar is a drawer, so a slim bar holds its trigger. */}
        <div className="flex h-14 shrink-0 items-center gap-2 border-b px-4 lg:hidden">
          <button type="button" className="btn-ghost btn-sm px-2" aria-expanded={mobileOpen} aria-controls="admin-nav" aria-label="Open menu" onClick={() => setMobileOpen(true)}>
            <Menu aria-hidden="true" className="size-5" />
          </button>
          <span className="truncate text-sm font-semibold">{SITE_NAME} admin</span>
        </div>
        {/* `relative` contains absolutely positioned children (e.g. sr-only labels) in this scroll area;
            without it they are placed against the page and stretch it past the viewport. */}
        <main id="main" tabIndex={-1} className="relative min-w-0 flex-1 overflow-y-auto focus:outline-none">
          <div className="mx-auto w-full max-w-7xl space-y-6 p-4 sm:p-6">{children}</div>
        </main>
      </div>
    </div>
  );
}
