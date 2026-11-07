import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = { title: "Page not found", robots: { index: false, follow: false } };

export default function RootNotFound() {
  return (
    <main id="main" className="mx-auto max-w-2xl space-y-4 px-4 py-16">
      <h1 className="text-3xl font-bold">Page not found</h1>
      <p className="text-slate-700">This page doesn&apos;t exist.</p>
      <Link href="/" className="btn-secondary">
        Go to the homepage
      </Link>
    </main>
  );
}
