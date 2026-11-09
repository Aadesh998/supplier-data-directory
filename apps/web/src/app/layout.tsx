import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { SITE_DESCRIPTION, SITE_NAME } from "@/lib/constants";
import { indexingAllowed, siteUrl } from "@/lib/env";
import { THEME_INIT_SCRIPT } from "@/lib/theme";

// Fonts are downloaded at build time and served from this app (no request to Google at runtime).
const geistSans = Geist({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-geist-sans", display: "swap" });
const geistMono = Geist_Mono({ subsets: ["latin"], variable: "--font-geist-mono", display: "swap" });

// Render on demand so runtime environment (API_INTERNAL_URL, indexing flag,
// site URL) is honoured and no API data is ever baked in at build time.
export const dynamic = "force-dynamic";

export async function generateMetadata(): Promise<Metadata> {
  const index = indexingAllowed();
  return {
    metadataBase: new URL(siteUrl()),
    title: { default: `${SITE_NAME}: Manufacturers & Suppliers Directory India`, template: `%s | ${SITE_NAME}` },
    description: SITE_DESCRIPTION,
    applicationName: SITE_NAME,
    keywords: [
      "manufacturers in India",
      "suppliers directory India",
      "B2B directory India",
      "contract manufacturers India",
      "job work suppliers",
      "industrial suppliers",
      "exporters India",
    ],
    category: "business",
    formatDetection: { telephone: false, email: false, address: false },
    robots: index ? { index: true, follow: true } : { index: false, follow: false },
  };
}

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#ffffff" },
    { media: "(prefers-color-scheme: dark)", color: "#09090b" },
  ],
  width: "device-width",
  initialScale: 1,
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    // suppressHydrationWarning: the theme script adds the "dark" class before React hydrates.
    <html lang="en-IN" className={`${geistSans.variable} ${geistMono.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
      </head>
      <body className="min-h-screen">
        <a
          href="#main"
          className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-background focus:px-4 focus:py-2 focus:text-foreground focus:shadow"
        >
          Skip to main content
        </a>
        {children}
      </body>
    </html>
  );
}
