import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { SITE_NAME } from "@/lib/constants";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({
    title: "About",
    description: `What ${SITE_NAME} is, who runs it, and how listings are reviewed and published.`,
    path: "/about",
  });
}

export default function AboutPage() {
  return (
    <StaticPage title="About the directory" path="/about" lead={`${SITE_NAME} is an early-stage, India-focused directory of manufacturers and industrial suppliers.`}>
      <h2>What we are building</h2>
      <p>
        Buyers, sourcing agents and other manufacturers often struggle to find accurate, up-to-date information about Indian suppliers —
        especially small and medium manufacturers, job-work providers and contract manufacturers. We are building a carefully reviewed
        directory that describes what each business makes, which processes it offers, and where it operates.
      </p>
      <h2>Current status: beta</h2>
      <p>
        The directory is in beta. It is run by an independent founder, is not yet a registered company, and does not sell data. The number
        of published profiles is still small, and we would rather publish fewer, accurate profiles than many unreviewed ones.
      </p>
      <h2>How listings work</h2>
      <ul>
        <li>Businesses can submit their own details through the <Link href="/register-supplier">registration form</Link>.</li>
        <li>Every submission and imported record is reviewed by a person before anything is published.</li>
        <li>A profile is published only with the business&apos;s permission or a documented right to display the information.</li>
        <li>
          Profiles show where information came from and whether the directory team has verified it. <strong>Publishing does not imply
          verification.</strong>
        </li>
        <li>We do not publish personal contact details, ratings, reviews or invented statistics.</li>
      </ul>
      <h2>What we do not do</h2>
      <p>
        We do not scrape map services or copy listings from other directories, and we do not send unsolicited bulk marketing. Read our{" "}
        <Link href="/data-policy">data policy</Link> for details.
      </p>
    </StaticPage>
  );
}
