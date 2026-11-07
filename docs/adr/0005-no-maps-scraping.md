# ADR 0005: No Google Maps scraping; discovery stores place IDs only

**Status:** accepted · 2026-11-07

## Context
Google Maps holds broad business coverage, but its policies prohibit exporting, extracting or scraping its content and storing names or addresses outside the Services. Data obtained that way could not be lawfully resold, which is the product we intend to sell.

## Decision
- No scraper, browser automation, CAPTCHA bypass or proxy rotation in this codebase.
- An optional, disabled-by-default Places API adapter returns transient results to an admin. Only `provider_place_id` may be persisted, as a lead for one-to-one outreach.
- `permitted_api_lookup` sources are constrained in the database so they can never be marked public or commercial.

## Consequences
- Coverage grows through supplier self-registration, imports of data with documented rights, licensed feeds and open data with compatible licences. Growth is slower but the data is sellable.
- See [data-source-policy.md](../data-source-policy.md) for the verified terms and the remaining uncertainty.
