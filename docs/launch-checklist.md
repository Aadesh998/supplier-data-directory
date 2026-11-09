# Launch checklist

## Data and rights
- [ ] Audit the source and permitted uses of every one of the existing 1,200 records. Record each as a `data_sources` row with only the rights you can evidence.
- [ ] Choose 1–2 pilot clusters (e.g. Morbi tiles, Panipat carpets) and focus on near-complete, verified coverage there.
- [ ] Map categories to official NIC 2025 codes only after checking the MoSPI publication.
- [ ] For any open dataset (MCA, Udyam/MSME on data.gov.in, OpenStreetMap), read its licence and record attribution requirements.

## Legal (qualified Indian counsel)
- [ ] Review `/privacy`, `/terms` and `/data-policy` (currently marked "Draft — pending legal review"), and the three registration authorisations.
- [ ] Confirm DPDP Act / Rules obligations: notice, lawful basis, grievance contact, retention, breach process.
- [ ] Draft a supplier data licence and an API customer agreement before any commercial use.
- [ ] Decide on business registration and GST when you start selling.

## Product
- [ ] Brand and domain chosen. "Peoplvance" is a working name, not a trademark claim.
- [ ] Dedicated business email and privacy contact (`PRIVACY_EMAIL`).
- [ ] SMTP configured; test every email template via Mailpit, then in production.
- [ ] Remove or never load the synthetic seed in production (it is private by default; `seed --demo` is refused in production).
- [ ] Write original industry descriptions for the pilot categories (Admin → Taxonomy).
- [ ] At least 20–50 real, permitted, published profiles in the pilot niche before enabling indexing.

## Security and operations
- [ ] `DATA_ENCRYPTION_KEY` generated and stored in a password manager.
- [ ] Production behind Caddy (never expose the API or web container ports directly).
- [ ] Backups scheduled, off-site copy configured, restore drill passed.
- [ ] Uptime monitoring and alerting on `/healthz`.
- [ ] Owner account uses a long unique password. Plan TOTP 2FA before adding more admins.
- [ ] Google Places adapter remains disabled unless you have read the current terms and set billing alerts.

## Go public
- [ ] `NEXT_PUBLIC_ALLOW_INDEXING=true`, redeploy web.
- [ ] Submit the sitemap to Google Search Console and Bing Webmaster Tools.
- [ ] Interview 10–20 procurement / B2B software buyers. Aim for one paid pilot (served with the commercial export) before building M8.
- [ ] Track: verification coverage, duplicate rate, supplier permission rate, form conversion, buyer pilot interest.
