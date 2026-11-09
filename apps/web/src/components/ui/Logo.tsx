import { SITE_NAME } from "@/lib/constants";

const P_PATH = "M18 46V18a4 4 0 0 1 4-4h12a13 13 0 0 1 0 26h-8v6a4 4 0 0 1-8 0zM28.5 27a5.5 5.5 0 1 0 11 0a5.5 5.5 0 1 0-11 0z";

/** The Peoplvance mark: a "P" on a blue tile, with a location dot. Fixed brand colours (same in light and dark), matching public/icon.svg. */
export function LogoMark({ className = "size-8" }: { className?: string }) {
  return (
    <svg viewBox="0 0 64 64" aria-hidden="true" className={`shrink-0 ${className}`}>
      <rect width="64" height="64" rx="14" fill="#1d4ed8" />
      <path d={P_PATH} fillRule="evenodd" fill="#ffffff" />
      <circle cx="45" cy="46" r="4" fill="#93c5fd" />
    </svg>
  );
}

/** Mark + wordmark. The name is real text so it uses the app font and stays accessible. */
export function Logo({ className = "" }: { className?: string }) {
  return (
    <span className={`inline-flex items-center gap-2 ${className}`}>
      <LogoMark />
      <span className="font-semibold tracking-tight">{SITE_NAME}</span>
    </span>
  );
}
