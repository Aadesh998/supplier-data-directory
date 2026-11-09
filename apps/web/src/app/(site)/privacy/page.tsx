import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { PRIVACY_NOTICE_VERSION } from "@/lib/constants";
import { privacyEmail } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({
    title: "Privacy notice",
    description: "How Peoplvance collects, uses, stores and protects information, and how to exercise your choices.",
    path: "/privacy",
  });
}

export default function PrivacyPage() {
  const email = privacyEmail();
  return (
    <StaticPage title="Privacy notice" path="/privacy" draft lead={`Version ${PRIVACY_NOTICE_VERSION}. This notice explains what we collect, why, and the choices you have.`}>
      <h2>Who we are</h2>
      <p>
        Peoplvance (peoplvance.org) is operated by an independent founder during a beta period. It is not yet a
        registered company. Contact for privacy matters: <a href={`mailto:${email}`}>{email}</a>.
      </p>

      <h2>What we collect</h2>
      <h3>Business information</h3>
      <p>
        When you register a business we collect the business details you enter: names, description, website, year of establishment,
        business roles, categories, products, facility locations and optional business contact points (such as a sales email or switchboard
        number).
      </p>
      <h3>Submitter details</h3>
      <p>
        We also collect the name, email address and optionally the designation and phone number of the person submitting the form. These
        details are <strong>stored privately and never published</strong>. We use them only to review the submission, contact you about it,
        and handle your requests.
      </p>
      <h3>Technical information</h3>
      <p>
        To protect the form from spam we record basic technical information such as the time taken to fill the form and request metadata
        (for example IP address) in security logs for a limited period.
      </p>

      <h2>How we use it now</h2>
      <ul>
        <li>To store your submission privately and review it manually.</li>
        <li>To contact you about your submission, corrections or requests.</li>
        <li>To detect spam and abuse.</li>
      </ul>
      <p>Nothing you submit is made public unless you give separate permission for a public listing and a reviewer approves the profile.</p>

      <h2>Your separate permissions</h2>
      <p>The registration form asks for three separate things. None is pre-selected:</p>
      <ol>
        <li>
          <strong>Acknowledgement of this notice (required).</strong> Confirms you have read how we process the submitted information so we
          can store and review it.
        </li>
        <li>
          <strong>Public listing (optional).</strong> Allows us, after review, to publish a public profile with the business fields described
          in the <Link href="/data-policy">data policy</Link>.
        </li>
        <li>
          <strong>Future commercial/API distribution (optional).</strong> Allows us, in future, to include the listed business fields (name,
          categories, products, city/state and business contacts) in a paid data service or API for other businesses. We do not sell data
          today. Submitter details are never included.
        </li>
      </ol>
      <p>
        You can withdraw an optional permission at any time using the <Link href="/request-correction">request form</Link>. Withdrawal does
        not affect what was done before it, but we will stop the related use going forward.
      </p>

      <h2>Sharing</h2>
      <p>
        We do not sell personal data. We use service providers (such as hosting and email delivery) who process data on our behalf. We may
        disclose information when required by law.
      </p>

      <h2>Retention</h2>
      <p>
        We keep submissions while they are under review and, for approved businesses, while the profile is maintained. Rejected or spam
        submissions are deleted or anonymised after a limited period. Audit records of permissions and changes are kept as long as needed to
        demonstrate how data was handled. See the <Link href="/data-policy">data policy</Link> for details.
      </p>

      <h2>Your choices and requests</h2>
      <p>
        You can ask to access, correct or remove information, or withdraw permissions, through the{" "}
        <Link href="/request-correction">correction and removal form</Link> or by email. We will respond as soon as we reasonably can.
      </p>

      <h2>Security</h2>
      <p>
        Data is stored on access-controlled systems, transmitted over encrypted connections, and only reviewers who need it can see submitter
        details. Access to personal details is logged.
      </p>

      <h2>Changes</h2>
      <p>
        If we change this notice we will publish a new version with a new version number. Submissions record the version that was shown at
        the time.
      </p>
    </StaticPage>
  );
}
