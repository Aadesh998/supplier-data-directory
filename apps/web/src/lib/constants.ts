export const SITE_NAME = "Peoplvance";
export const SITE_DOMAIN = "peoplvance.org";
export const SITE_TAGLINE = "Find verified manufacturers, suppliers and job-work units across India";
export const SITE_DESCRIPTION =
  "Peoplvance is a B2B directory of Indian manufacturers, contract manufacturers, job-work units, exporters and wholesalers. Search by product, industry cluster and state. Every profile is reviewed by a person and published with the business's permission.";
export const PRIVACY_NOTICE_VERSION = "2026-11-v1";
export const DEFAULT_PAGE_SIZE = 20;
export const SYNTHETIC_MARKER = "(Synthetic)";

export function isSynthetic(name: string | undefined | null): boolean {
  return !!name && name.includes(SYNTHETIC_MARKER);
}
