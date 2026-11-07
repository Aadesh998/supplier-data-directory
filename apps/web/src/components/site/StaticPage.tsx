import { Breadcrumbs } from "./Breadcrumbs";
import { DraftLegalNotice } from "./Notices";

export function StaticPage({
  title,
  path,
  draft,
  lead,
  children,
}: {
  title: string;
  path: string;
  draft?: boolean;
  lead?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="container-page space-y-6 py-8">
      <Breadcrumbs items={[{ name: "Home", path: "/" }, { name: title, path }]} />
      <header className="max-w-3xl space-y-2">
        <h1 className="text-3xl font-bold">{title}</h1>
        {lead && <p className="text-lg text-slate-700">{lead}</p>}
      </header>
      {draft && <DraftLegalNotice />}
      <div className="prose-page">{children}</div>
    </div>
  );
}
