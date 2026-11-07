import type { Metadata, Viewport } from "next";
import "./globals.css";
import { SITE_NAME, SITE_TAGLINE } from "@/lib/constants";
import { indexingAllowed, siteUrl } from "@/lib/env";

// Render on demand so runtime environment (API_INTERNAL_URL, indexing flag,
// site URL) is honoured and no API data is ever baked in at build time.
export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const index = indexingAllowed();
  return {
    metadataBase: new URL(siteUrl()),
    title: { default: SITE_NAME, template: `%s | ${SITE_NAME}` },
    description: SITE_TAGLINE,
    applicationName: SITE_NAME,
    robots: index ? { index: true, follow: true } : { index: false, follow: false },
  };
}

export const viewport: Viewport = {
  themeColor: "#1b4a83",
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-IN">
      <body className="min-h-screen">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-white focus:px-4 focus:py-2 focus:text-accent-800 focus:shadow"
        >
          Skip to main content
        </a>
        {children}
      </body>
    </html>
  );
}
