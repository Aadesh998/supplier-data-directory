import type { CertificationStatus, FacilityType, PrivacyRequestType, VerificationStatus } from "./types";

export const FACILITY_TYPES: { value: FacilityType; label: string }[] = [
  { value: "factory", label: "Factory / plant" },
  { value: "office", label: "Office" },
  { value: "registered_office", label: "Registered office" },
  { value: "warehouse", label: "Warehouse" },
  { value: "showroom", label: "Showroom" },
  { value: "workshop", label: "Workshop" },
  { value: "laboratory", label: "Laboratory" },
  { value: "other", label: "Other" },
];

export function facilityLabel(value: string | undefined | null): string {
  return FACILITY_TYPES.find((f) => f.value === value)?.label ?? humanize(value ?? "");
}

export const VERIFICATION_LABELS: Record<VerificationStatus, string> = {
  unverified: "Not yet verified by the directory team",
  partially_verified: "Partially verified by the directory team",
  verified: "Verified by the directory team",
  stale: "Verification out of date",
};

export function verificationLabel(status: string | undefined): string {
  return (status && VERIFICATION_LABELS[status as VerificationStatus]) || "Not yet verified by the directory team";
}

export const CERTIFICATION_LABELS: Record<CertificationStatus, string> = {
  claimed: "Claimed, not verified",
  unverified: "Claimed, not verified",
  verified: "Verified by directory team",
  expired: "Expired",
  revoked: "Revoked",
  rejected: "Could not be verified",
};

export function certificationLabel(status: string | undefined): string {
  return (status && CERTIFICATION_LABELS[status as CertificationStatus]) || "Claimed, not verified";
}

export function capabilityLabel(status: string | undefined): string {
  return status === "verified" ? "Verified by directory team" : "Claimed, not verified";
}

export const PRIVACY_REQUEST_TYPES: { value: PrivacyRequestType; label: string }[] = [
  { value: "correction", label: "Correct information in a profile" },
  { value: "removal", label: "Remove a profile or information" },
  { value: "withdraw_public_listing", label: "Withdraw permission for public listing" },
  { value: "withdraw_commercial_api", label: "Withdraw permission for commercial/API distribution" },
  { value: "access", label: "Access the data held about me or my business" },
  { value: "other", label: "Something else" },
];

export function humanize(value: string): string {
  if (!value) return "";
  const s = value.replace(/_/g, " ");
  return s.charAt(0).toUpperCase() + s.slice(1);
}

export function formatDate(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString("en-IN", { year: "numeric", month: "short", day: "numeric", timeZone: "Asia/Kolkata" });
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) return "—";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("en-IN", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Asia/Kolkata",
  });
}
