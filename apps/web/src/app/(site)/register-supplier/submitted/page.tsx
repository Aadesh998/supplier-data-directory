import Link from "next/link";
import { firstParam } from "@/components/site/Pagination";
import { noindexMetadata } from "@/lib/seo";

export const metadata = noindexMetadata("Submission received");

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

export default async function SubmittedPage({ searchParams }: Props) {
  const ref = firstParam((await searchParams).ref);
  return (
    <div className="container-page max-w-2xl space-y-6 py-12">
      <div className="rounded-md border border-emerald-300 bg-emerald-50 px-4 py-3 text-emerald-950" role="status">
        <h1 className="text-2xl font-bold text-emerald-950">Thank you — your submission has been received</h1>
        {ref && <p className="mt-1 text-sm">Reference: <span className="font-mono">{ref}</span></p>}
      </div>
      <section className="space-y-3">
        <h2 className="text-xl font-semibold">What happens next</h2>
        <ol className="list-decimal space-y-2 pl-6 text-slate-800">
          <li>A person will review your submission. We may email you if something needs clarifying.</li>
          <li>Nothing is published before approval. If you did not give public listing permission, your details stay private.</li>
          <li>If you allowed a public listing and the submission is approved, a profile page may be published. It will be labelled as submitted by the business; it is not a verification.</li>
        </ol>
        <p className="text-slate-700">
          We don&apos;t promise enquiries, search rankings or verification, and listing is free.
        </p>
      </section>
      <section className="space-y-2">
        <h2 className="text-xl font-semibold">Need to change something?</h2>
        <p className="text-slate-700">
          You can ask us to correct your details, withdraw a permission or remove your submission at any time using the{" "}
          <Link href="/request-correction" className="link">
            correction and removal form
          </Link>
          {ref ? " (mention the reference above)" : ""}.
        </p>
      </section>
      <p>
        <Link href="/" className="btn-secondary">
          Back to the homepage
        </Link>
      </p>
    </div>
  );
}
