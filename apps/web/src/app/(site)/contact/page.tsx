import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { SITE_NAME } from "@/lib/constants";
import { contactEmail, privacyEmail } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({
    title: "Contact",
    description: `Contact the ${SITE_NAME} team: request a full supplier profile, list your factory, or correct a listing.`,
    path: "/contact",
  });
}

type Props = { searchParams: Promise<{ topic?: string; company?: string }> };

const SLUG = /^[a-z0-9-]{1,160}$/;

export default async function ContactPage({ searchParams }: Props) {
  const { topic, company } = await searchParams;
  const email = privacyEmail();
  const sales = contactEmail();
  const slug = company && SLUG.test(company) ? company : undefined;
  const subject = slug ? `Full profile request: ${slug}` : "Full supplier profile request";
  const body = [
    "Hello,",
    "",
    slug ? `I would like the full profile and contact details for: /suppliers/${slug}` : "I would like full supplier details for:",
    "",
    "My name:",
    "Company:",
    "What I want to source (product, quantity, location):",
    "",
    "Thank you.",
  ].join("\n");
  const accessMail = `mailto:${sales}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;

  return (
    <StaticPage title="Contact" path="/contact">
      {topic === "access" && (
        <div className="not-prose rounded-lg border border-accent-200 bg-accent-50 p-4">
          <h2 className="text-lg font-semibold">Request a full supplier profile</h2>
          <p className="mt-1 text-sm text-slate-800">
            Public profiles show what a supplier makes and where it is based. Contact details, website, legal name and exact location are
            shared with genuine buyers on request, with the supplier&apos;s permission. Tell us what you want to source and we will
            reply by email.
          </p>
          <a href={accessMail} className="btn-primary mt-3">
            Email a request{slug ? " for this supplier" : ""}
          </a>
        </div>
      )}
      <p>We are a small team, so please choose the route that fits your question. It helps us reply faster.</p>
      <ul>
        <li>
          <strong>Buyers: get full supplier details or sourcing help.</strong> Email <a href={accessMail}>{sales}</a> with what you need.
        </li>
        <li>
          <strong>Correct, update or remove a profile, or withdraw a permission:</strong> use the{" "}
          <Link href="/request-correction">correction and removal form</Link>. Requests are logged and handled by a person.
        </li>
        <li>
          <strong>List your factory or business for free:</strong> use the <Link href="/register-supplier">registration form</Link>.
        </li>
        <li>
          <strong>Privacy questions or data requests:</strong> email <a href={`mailto:${email}`}>{email}</a>.
        </li>
      </ul>
      <p>We do not offer phone support during the beta.</p>
    </StaticPage>
  );
}
