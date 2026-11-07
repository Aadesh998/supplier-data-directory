# How the schema fits different kinds of supplier

Industry, role, category, product, process/capability and facility are separate concepts, so one company can be several things at once without a flat "industry" enum. Category-specific specs go in `products.attributes` (JSONB). Everything identity-related stays in normal tables.

| Example | Roles | Categories (tree path) | Products + `attributes` | Capabilities | Facilities |
|---|---|---|---|---|---|
| **Hand-knotted rug maker, Bhadohi** | manufacturer, exporter | Textiles and apparel › Rugs and carpets › Hand-knotted rugs | "Wool rug" `{"knots_per_sq_inch": 120, "pile": "cut", "composition": "100% New Zealand wool", "sizes": ["5x8","8x10"]}` | hand-knotting, design-development, private-label | factory (Bhadohi, UP) |
| **Namkeen manufacturer, Bikaner** | manufacturer, wholesaler | Food and beverages › Namkeen and savoury snacks | "Bikaneri bhujia" `{"pack_sizes_g": [200,400,1000], "shelf_life_days": 120, "allergens": ["gram flour"]}` | frying-and-roasting, nitrogen-flush-packing | factory; FSSAI licence as a `certifications` row (status `claimed` until checked) |
| **Vitrified tile plant, Morbi** | manufacturer, exporter | Building and construction materials › Tiles › Vitrified tiles | "GVT 600×1200" `{"thickness_mm": 9, "water_absorption_pct": 0.5, "finish": "glossy"}` | tile-pressing-and-kiln, digital-glazing | factory + warehouse |
| **Fly-ash brick maker** | manufacturer, distributor | … › Bricks › Fly-ash bricks | "Fly-ash brick" `{"size_mm": "230x110x75", "compressive_strength_mpa": 7.5}` | fly-ash-brick-press | factory |
| **CNC job shop, Coimbatore** | manufacturer, job_work_provider, component_supplier | Industrial machinery and components › CNC machined components (+ Auto components) | "Turned shaft" `{"materials": ["EN8","SS304"], "tolerance_mm": 0.01, "max_diameter_mm": 300}` | cnc-turning, cnc-milling, cmm-inspection | factory |
| **Sourcing house, Delhi** (no factory) | sourcing_agent, buying_house, exporter | Sourcing and trading › Sourcing agents (+ the categories it sources: Handicrafts, Home textiles) | none, or represented product lines | third-party-inspection, export-documentation | office only, so buyers can tell intermediaries from factory owners |

Notes:
- A category can hold companies in any role. "Rugs" lists both the weaver and the sourcing house, and the role filter separates them.
- Search by category includes descendants: `?category=tiles` finds ceramic, vitrified and porcelain makers.
- `nic_code` is left empty until mapped against the official NIC 2008/2025 publication. No codes are guessed.
- Verification is per company (`verification_status`) and per claim (capabilities and certifications have their own status). A published rug exporter can still be `unverified`, and the public page says so.
- The synthetic seed (`db/seeds/001_synthetic_companies.sql`) has 20 examples across these six types, all labelled "(Synthetic)" with `.example` domains.
