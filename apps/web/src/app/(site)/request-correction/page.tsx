import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { firstParam } from "@/components/site/Pagination";
import { CorrectionRequestForm } from "@/components/forms/CorrectionRequestForm";
import { privacyEmail } from "@/lib/env";
import { noindexMetadata } from "@/lib/seo";

export const metadata = noindexMetadata("Request a correction or removal");

type Props = { searchParams: Promise<Record<string, string | string[] | undefined>> };

export default async function RequestCorrectionPage({ searchParams }: Props) {
  const company = firstParam((await searchParams).company);
  return (
    <div className="container-page max-w-3xl space-y-6 py-8">
      <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: "Request a correction or removal", path: "/request-correction" }]} />
      <header className="space-y-2">
        <h1 className="text-3xl font-bold">Request a correction or removal</h1>
        <p className="text-slate-700">
          Use this form to correct a profile, remove information, withdraw a permission you gave (public listing or commercial/API
          distribution), or ask what data we hold. Every request is read by a person and logged.
        </p>
      </header>
      <CorrectionRequestForm companyReference={company} privacyEmail={privacyEmail()} />
    </div>
  );
}
