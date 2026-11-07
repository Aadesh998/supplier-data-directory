# Supplier-facing copy

All copy is a draft for legal review. It must not promise leads, rankings or verification.

## Where the copy lives

| Copy | File |
|---|---|
| Registration form: steps, help text, the three permission checkboxes and their explanations | `apps/web/src/components/register/RegistrationForm.tsx` |
| Why register / what happens next | `apps/web/src/app/(site)/register-supplier/page.tsx`, `…/register-supplier/submitted/page.tsx` |
| Privacy notice (version `2026-11-v1`), terms, data policy | `apps/web/src/app/(site)/privacy`, `terms`, `data-policy` |
| Correction / removal form | `apps/web/src/components/forms/CorrectionRequestForm.tsx` |
| Emails | `internal/notify/templates/*.txt` |

**Changing the privacy notice:** bump `PRIVACY_NOTICE_VERSION` in the API environment and `PRIVACY_NOTICE_VERSION` in `apps/web/src/lib/constants.ts` together. The API rejects submissions made against an old version, so every stored consent maps to the exact text the supplier saw.

## The three authorisations (never pre-ticked, never bundled)

1. **Required:** "I have read the privacy notice and understand how this information will be processed", with "We will store your submission privately and review it manually" and links to the privacy notice (version 2026-11-v1) and data policy. This acknowledges the notice rather than asking for blanket consent. It is recorded as `storage_consent_at`.
2. **Optional:** permission to publish a basic public profile, after review, showing the business fields provided. Recorded as `public_listing_permission_at`.
3. **Optional:** permission for future commercial/API distribution of the listed business fields (name, categories, products, city/state, business contacts) to approved business customers. Recorded as `future_commercial_api_permission_at`.

Each can be withdrawn later through `/request-correction`. The supplier's personal details (name, email, phone) are never published or distributed.

## Email templates (`internal/notify/templates/`)

| Template | Sent when |
|---|---|
| `submission_received.txt` | Immediately after registration. Restates the choices, says nothing is public before review, links corrections. |
| `admin_new_submission.txt` | To `ADMIN_NOTIFY_EMAIL`. No personal details in the email. |
| `submission_approved.txt` | After approval. Explains private vs public and that publishing ≠ verification. |
| `submission_rejected.txt` | Rejected or needs changes, with the reason. Spam gets no email. |
| `privacy_request_received.txt` | Acknowledges a correction or removal request |
| `supplier_invitation.txt` | **Template for one-to-one, personal outreach only.** It is not sent by the system. Use it only where you have a lawful basis to contact that business. Never use it for bulk sends. |
