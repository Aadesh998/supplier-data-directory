import Link from "next/link";
import { Breadcrumbs } from "@/components/site/Breadcrumbs";
import { RegistrationForm } from "@/components/register/RegistrationForm";
import { pageMetadata } from "@/lib/seo";
import { getCategoryTree, getRoles, getStates } from "@/lib/server-api";

export async function generateMetadata() {
  return pageMetadata({
    title: "Register your business",
    description:
      "Add a free basic profile for your manufacturing or industrial supply business. Every submission is reviewed manually and nothing is published without your permission.",
    path: "/register-supplier",
  });
}

export default async function RegisterSupplierPage() {
  const [roles, tree, states] = await Promise.all([getRoles(), getCategoryTree(), getStates()]);
  return (
    <div className="container-page grid gap-8 py-8 lg:grid-cols-[2fr_1fr]">
      <div className="space-y-6">
        <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: "Register your business", path: "/register-supplier" }]} />
        <header className="space-y-2">
          <h1 className="text-3xl font-bold">Register your business</h1>
          <p className="text-slate-700">
            It takes about 10 minutes. You&apos;ll need your business details, the categories you work in, and at least one facility
            location.
          </p>
        </header>
        <RegistrationForm
          initialOptions={{
            roles: roles.ok ? roles.data.data : [],
            categoryTree: tree.ok ? tree.data.data : [],
            states: states.ok ? states.data.data : [],
          }}
        />
      </div>
      <aside className="space-y-4 text-sm lg:pt-16">
        <section className="card space-y-2">
          <h2 className="text-base font-semibold">Why register?</h2>
          <ul className="list-disc space-y-1 pl-5 text-slate-700">
            <li>A free basic profile — no paid placements.</li>
            <li>Be discoverable by buyers when the directory launches.</li>
            <li>Easy corrections whenever your details change.</li>
            <li>Possibly, sourcing enquiries in future.</li>
          </ul>
          <p className="text-slate-700">We can&apos;t promise leads, rankings or verification.</p>
        </section>
        <section className="card space-y-2">
          <h2 className="text-base font-semibold">What happens to your data</h2>
          <ul className="list-disc space-y-1 pl-5 text-slate-700">
            <li>It is stored privately and reviewed by a person.</li>
            <li>Nothing is public unless you allow a public listing and we approve it.</li>
            <li>Your personal details are never published.</li>
            <li>Commercial/API use only with your separate, optional permission.</li>
            <li>You can correct, withdraw or ask for removal any time.</li>
          </ul>
          <p>
            <Link href="/data-policy" className="link">
              Read the data policy
            </Link>{" "}
            ·{" "}
            <Link href="/privacy" className="link">
              Privacy notice
            </Link>
          </p>
        </section>
      </aside>
    </div>
  );
}
