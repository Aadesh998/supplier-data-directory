import { LoginForm } from "@/components/admin/LoginForm";
import { noindexMetadata } from "@/lib/seo";

export const metadata = noindexMetadata("Admin sign in");

export default function LoginPage() {
  return (
    <div className="container-page max-w-md py-12">
      <h1 className="mb-2 text-2xl font-bold">Admin sign in</h1>
      <p className="mb-6 text-sm text-slate-700">For directory staff only. Suppliers do not need an account.</p>
      <LoginForm />
    </div>
  );
}
