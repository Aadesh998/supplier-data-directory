export function SyntheticBanner() {
  return (
    <div role="note" className="rounded-md border-2 border-amber-400 bg-amber-50 px-4 py-3 text-sm font-semibold text-amber-950">
      Synthetic demo record — not a real business
    </div>
  );
}

export function DraftLegalNotice() {
  return (
    <div role="note" className="mb-6 rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-950">
      <strong>Draft — pending legal review.</strong> This text is a plain-language draft and has not yet been reviewed by a qualified
      lawyer. It may change before public launch.
    </div>
  );
}

export function ApiUnavailableNotice({ what = "this information" }: { what?: string }) {
  return (
    <div role="status" className="rounded-md border border-slate-300 bg-slate-100 px-4 py-3 text-sm text-slate-800">
      We couldn&apos;t load {what} right now. Please try again in a few minutes.
    </div>
  );
}

export function EmptyNotice({ children }: { children: React.ReactNode }) {
  return <div className="rounded-md border border-dashed border-slate-300 bg-white px-4 py-8 text-center text-sm text-slate-700">{children}</div>;
}
