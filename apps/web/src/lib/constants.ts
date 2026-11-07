export const SITE_NAME = "Bharat Manufacturing Directory";
export const SITE_TAGLINE = "An India-focused directory of manufacturers and industrial suppliers (beta)";
export const PRIVACY_NOTICE_VERSION = "2026-11-v1";
export const DEFAULT_PAGE_SIZE = 20;
export const SYNTHETIC_MARKER = "(Synthetic)";

export function isSynthetic(name: string | undefined | null): boolean {
  return !!name && name.includes(SYNTHETIC_MARKER);
}
