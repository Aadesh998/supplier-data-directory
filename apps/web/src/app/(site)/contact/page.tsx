import Link from "next/link";
import { StaticPage } from "@/components/site/StaticPage";
import { privacyEmail } from "@/lib/env";
import { pageMetadata } from "@/lib/seo";

export async function generateMetadata() {
  return pageMetadata({ title: "Contact", description: "How to contact the Bharat Manufacturing Directory team.", path: "/contact" });
}

export default function ContactPage() {
  const email = privacyEmail();
  return (
    <StaticPage title="Contact" path="/contact">
      <p>We are a small team, so please choose the route that fits your question — it helps us respond faster.</p>
      <ul>
        <li>
          <strong>Correct, update or remove a profile, or withdraw a permission:</strong> use the{" "}
          <Link href="/request-correction">correction and removal form</Link>. Requests are logged and handled by a person.
        </li>
        <li>
          <strong>Add your business:</strong> use the <Link href="/register-supplier">registration form</Link>.
        </li>
        <li>
          <strong>Privacy questions or data requests:</strong> email <a href={`mailto:${email}`}>{email}</a>.
        </li>
      </ul>
      <p>We do not offer phone support during the beta.</p>
    </StaticPage>
  );
}
