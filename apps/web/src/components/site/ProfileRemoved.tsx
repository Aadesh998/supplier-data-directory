import Link from "next/link";

export function ProfileRemoved() {
  return (
    <div className="container-page max-w-2xl space-y-4 py-16">
      <h1 className="text-3xl font-bold">This profile has been removed</h1>
      <p className="text-slate-700">
        The business profile that used to be at this address is no longer published in Peoplvance. This can happen
        when a business withdraws permission, asks for removal, or the listing no longer meets our publication rules.
      </p>
      <p className="text-slate-700">
        If you represent the business and believe this is a mistake, please{" "}
        <Link href="/request-correction" className="link">
          contact us through the correction form
        </Link>
        .
      </p>
      <p>
        <Link href="/search" className="btn-secondary">
          Search other suppliers
        </Link>
      </p>
    </div>
  );
}
