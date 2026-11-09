import Link from "next/link";

/** Placeholder bar for a withheld value. The text is decorative only: real values never reach the browser. */
export function LockedValue({ width = "10rem" }: { width?: string }) {
  return (
    <span className="locked-value" style={{ width }} aria-hidden="true">
      hidden value
    </span>
  );
}

export function LockIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true" className={className}>
      <path
        fillRule="evenodd"
        d="M10 1a4.5 4.5 0 0 0-4.5 4.5V8H5a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-6a2 2 0 0 0-2-2h-.5V5.5A4.5 4.5 0 0 0 10 1Zm3 7V5.5a3 3 0 1 0-6 0V8h6Z"
        clipRule="evenodd"
      />
    </svg>
  );
}

export function accessHref(slug: string) {
  return `/contact?topic=access&company=${encodeURIComponent(slug)}`;
}

/** A row for a withheld field: label, blurred placeholder and a lock. */
export function LockedRow({ label, width }: { label: string; width?: string }) {
  return (
    <div>
      <dt className="text-slate-600">{label}</dt>
      <dd className="flex items-center gap-2 text-slate-500">
        <LockIcon />
        <LockedValue width={width} />
        <span className="sr-only">withheld on the public profile</span>
      </dd>
    </div>
  );
}

export function UnlockCta({ slug, children }: { slug: string; children?: React.ReactNode }) {
  return (
    <div className="space-y-2 rounded-md border border-accent-200 bg-accent-50 p-3 text-sm">
      <p className="text-slate-800">
        {children ?? "Contact details, website and exact location are shared with verified buyers on request, with the supplier's permission."}
      </p>
      <Link href={accessHref(slug)} className="btn-primary btn-sm w-full">
        <LockIcon className="h-3.5 w-3.5" /> Request full profile
      </Link>
    </div>
  );
}
