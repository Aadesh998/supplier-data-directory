import Link from "next/link";

export default function NotFound() {
  return (
    <div className="container-page max-w-2xl space-y-4 py-16">
      <h1 className="text-3xl font-bold">Page not found</h1>
      <p className="text-slate-700">
        This page doesn&apos;t exist, or the profile is not published. Profiles are only shown after review and with permission.
      </p>
      <p className="flex gap-3">
        <Link href="/search" className="btn-primary">
          Search suppliers
        </Link>
        <Link href="/" className="btn-secondary">
          Homepage
        </Link>
      </p>
    </div>
  );
}
