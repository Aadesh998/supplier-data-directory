import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { PRIVACY_NOTICE_VERSION } from "@/lib/constants";
import { privacyEmail } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({
    title: "Data policy",
    description:
      "Why register, what is required, how submitted data is used, what public listing and future commercial/API distribution mean, and how to withdraw or correct.",
    path: "/data-policy",
  });
}

export default function DataPolicyPage() {
  const email = privacyEmail();
  return (
    <StaticPage title="Data policy for businesses" path="/data-policy" draft lead="A plain-language explanation of how we handle business information.">
      <h2>Why register</h2>
      <ul>
        <li>A free basic profile describing what your business makes and where it operates.</li>
        <li>Discoverability by buyers and sourcing teams once the directory launches publicly.</li>
        <li>Easy corrections: you can ask us to update or remove your details at any time.</li>
        <li>Possibly, sourcing enquiries from buyers in future.</li>
      </ul>
      <p>
        <strong>We cannot promise leads, enquiries, search rankings or verification.</strong> Registering is free, and there is no paid
        listing or &ldquo;premium placement&rdquo;.
      </p>

      <h2>What is required and what is optional</h2>
      <ul>
        <li>
          <strong>Required:</strong> business display name, at least one business role, at least one category, at least one facility (type,
          city and state), the submitter&apos;s name and email, and acknowledgement of the{" "}
          <Link href="/privacy">privacy notice</Link>.
        </li>
        <li>
          <strong>Optional:</strong> legal name, description, website, establishment year, products, full facility address and PIN code,
          business contact points, the submitter&apos;s designation and phone, and the two optional permissions below.
        </li>
      </ul>

      <h2>How your data is used now</h2>
      <p>
        Submissions are stored privately and reviewed by a person. We may contact the submitter to clarify details. Until a profile is
        approved <em>and</em> you have given public listing permission, nothing is shown publicly.
      </p>

      <h2>What a public listing means</h2>
      <p>
        If you give public listing permission and the profile is approved, we may publish a profile page with: business name and legal name,
        description, website, establishment year, roles, categories, products, facility locality/city/district/state, and the business
        contact points you provided. Street addresses, PIN codes and submitter details are not published. Public pages may be indexed by
        search engines.
      </p>

      <h2>What future commercial/API distribution means</h2>
      <p>
        We may in future offer a paid data service or API to other businesses (for example procurement software). If you give this separate,
        optional permission, the following listed business fields could be included: <strong>business name, categories, products,
        city/state and business contact points</strong>. Submitter details are never included. We do not sell data today, and this
        permission is never bundled with any other.
      </p>

      <h2>Withdrawing permission or correcting data</h2>
      <p>
        Use the <Link href="/request-correction">correction and removal form</Link> and choose the request type — for example
        &ldquo;Withdraw permission for public listing&rdquo; — or email <a href={`mailto:${email}`}>{email}</a>. When public listing
        permission is withdrawn we unpublish the profile; removed profiles return a &ldquo;removed&rdquo; page.
      </p>

      <h2>Retention</h2>
      <ul>
        <li>Pending submissions: kept while under review.</li>
        <li>Approved profiles: kept while the business remains listed or until you ask us to remove them.</li>
        <li>Rejected or spam submissions: deleted or anonymised after a limited period.</li>
        <li>Records of permissions, changes and requests: kept as long as needed to show how data was handled.</li>
      </ul>

      <h2>Where data comes from</h2>
      <p>
        Besides self-registration, we may add businesses from sources we have a documented right to use (for example licensed datasets).
        Records from sources with unknown rights stay private. We do not scrape map services or other directories. Each profile shows source
        labels describing where its information came from.
      </p>

      <p className="text-sm text-slate-600">Applies to submissions made under privacy notice version {PRIVACY_NOTICE_VERSION}.</p>
    </StaticPage>
  );
}
