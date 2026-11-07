# ADR 0004: Field-level provenance and source permissions

**Status:** accepted · 2026-11-07

## Context
The business model is licensing data. Its value depends on proving, field by field, that we may display or redistribute it. Records merge data from multiple sources with different rights, and suppliers can withdraw permission.

## Decision
- `data_sources` carries three independent rights (internal, public display, commercial API), plus retention and restrictions. All default to false.
- `field_provenance` links every stored field to its source. Views (`public_/commercial_/internal_field_permissions`) answer "may this field be used for X".
- Public responses, exports and publish checks compose output field by field from those views.
- Company-level withdrawal timestamps override sources for one company.
- Merges move child records with their original `source_id` and copy provenance for filled fields.

## Consequences
- Every write path must record provenance. The store layer does this, and tests assert it.
- Public queries are more complex. We measured and indexed them (ADR 0001).
- Revoking a source cascades: affected companies are unpublished automatically.
