import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({ title: "Terms of use", description: "Terms for using the Bharat Manufacturing Directory during its beta.", path: "/terms" });
}

export default function TermsPage() {
  return (
    <StaticPage title="Terms of use" path="/terms" draft>
      <h2>Beta service</h2>
      <p>
        The directory is provided as a beta service, &ldquo;as is&rdquo;. We work to keep information accurate but cannot guarantee that
        every profile is complete, current or correct. Always carry out your own checks before doing business with any supplier.
      </p>
      <h2>No endorsement or verification unless stated</h2>
      <p>
        A published profile is not an endorsement. A profile is only described as verified where the directory team has recorded a
        verification, and then only for the information and date shown.
      </p>
      <h2>Submitting information</h2>
      <p>
        If you submit a business, you confirm that you are authorised to provide its details and that they are accurate to the best of your
        knowledge. Do not submit information about other people without a lawful basis.
      </p>
      <h2>Acceptable use</h2>
      <ul>
        <li>Do not scrape, bulk-download or republish directory content.</li>
        <li>Do not submit spam, misleading information or other people&apos;s personal data.</li>
        <li>Do not attempt to access the admin console or interfere with the service.</li>
      </ul>
      <h2>Corrections and removal</h2>
      <p>
        Use the <Link href="/request-correction">correction and removal form</Link> to report errors or ask for removal.
      </p>
      <h2>Changes</h2>
      <p>We may update these terms as the service develops. The current version is always on this page.</p>
    </StaticPage>
  );
}
