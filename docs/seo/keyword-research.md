# Peoplvance keyword research and SEO plan (peoplvance.org)

Researched 9 Nov 2026 with Claude's `/seo` skill and live web search. No Semrush/DataForSEO
account was connected, so **search volumes and difficulty are estimates** inferred from who ranks,
not measured numbers. Connect DataForSEO (`/seo dataforseo`) or Google Search Console after launch to
replace them with real data.

## What the results pages show

| Query type | Who ranks today | What it means for a new domain |
|---|---|---|
| "manufacturers directory India", "suppliers in India" | IndiaMART, Tradeindia, ExportersIndia, IndianYellowPages, Kompass | Head terms are out of reach for 12+ months. Don't target them on the home page alone. |
| "[product] manufacturers in [city]", e.g. "vitrified tiles manufacturers in Morbi" | IndiaMART city pages, niche blogs (thetilesofindia.com), company sites | **Winnable** with real, reviewed listings and useful cluster content. |
| "[service] job work [city]", e.g. "CNC job work Pune" | IndiaMART seller pages, small job shops | Winnable. Job-work units are under-served by quality pages. |
| "contract manufacturers in India [category]" | ensun.io, Clutch, Zetwerk, IndiaMART | Medium. Category hubs with honest verification labels can compete. |
| Cluster/informational, e.g. "Panipat textile industry", "Morbi tile cluster" | Blogs, news, Wikipedia | Winnable with guides. Feeds AI Overviews and links to listings. |

## Keyword map

Intent: C = commercial (find a supplier), I = informational. Difficulty estimate: H/M/L.

| Page (route) | Primary keyword | Secondary | Intent | Est. difficulty |
|---|---|---|---|---|
| `/` | manufacturers and suppliers in India | B2B supplier directory India, verified manufacturers India, find suppliers in India | C | H |
| `/industries` | manufacturers by industry India | industrial suppliers list India | C | M |
| `/industries/tiles`, `/industries/vitrified-tiles` | vitrified tiles manufacturers in India | tile manufacturers Morbi, ceramic tiles suppliers Gujarat | C | M |
| `/industries/rugs-and-carpets` | carpet manufacturers in India | rug manufacturers Panipat, Bhadohi carpet exporters, hand tufted rugs manufacturers | C | M |
| `/industries/yarn` | yarn manufacturers in India | wool yarn suppliers Panipat, acrylic yarn manufacturers | C | M |
| `/industries/home-textiles` | home textile manufacturers in India | Panipat home furnishing manufacturers, blanket manufacturers Panipat | C | M |
| `/industries/dyeing-and-finishing` | yarn dyeing job work | fabric dyeing units Panipat | C | L |
| `/industries/garment-manufacturing` | garment manufacturers in India | T-shirt manufacturers Tiruppur, private label clothing manufacturers India | C | H |
| `/industries/cnc-machined-components` | CNC machining job work | CNC job work Pune, VMC job work, precision machining suppliers India | C | M |
| `/industries/namkeen-and-snacks` | namkeen manufacturers in India | private label namkeen manufacturers, snacks manufacturers Bikaner | C | L–M |
| `/industries/auto-components` | auto component manufacturers in India | auto parts suppliers Pune, Chakan | C | M |
| `/industries/corrugated-boxes` | corrugated box manufacturers | carton box suppliers [city] | C | L |
| `/industries/pharmaceuticals` | pharma contract manufacturing India | third party manufacturing Baddi | C | H |
| `/locations/haryana` | manufacturers in Haryana | Panipat manufacturers, industrial suppliers Haryana | C | M |
| `/locations/gujarat`, `/maharashtra`, `/tamil-nadu`, `/uttar-pradesh` | manufacturers in [state] | [state] industrial suppliers | C | M |
| `/suppliers/[slug]` | [company name] | [company] [city], [company] products | C (navigational) | L |
| Guides (to build) | Panipat textile industry | Panipat handloom cluster, Morbi ceramic cluster, Tiruppur knitwear cluster | I | L–M |

## What was implemented in the code

- Brand: **Peoplvance**. Home title "Peoplvance: Find Manufacturers & Suppliers in India".
- Title pattern for industries: "[Category] Manufacturers & Suppliers in India", and "Manufacturers & Suppliers in [State]" for locations. Supplier pages: "Company · Category · City, State".
- Descriptions are keyword-led, include counts, and stay under 160 characters.
- The landing page is content-rich: clusters section, buyer/supplier how-it-works, free-vs-full profile, trust, FAQ as visible content. There is no FAQPage markup, because Google retired FAQ rich results.
- Structured data: WebSite + Organization (home), Organization (supplier), BreadcrumbList (existing). No street address or contact data in markup.
- Generated 1200×630 Open Graph image, SVG icon, web manifest, `summary_large_image` cards.
- Footer: internal links to the priority industry and state pages.
- Indexing safeguards (existing): `noindex` on industry and state pages with fewer than 3 listings, on synthetic records and on staging. Robots and sitemap only list published, permitted data. Indexing stays off until `NEXT_PUBLIC_ALLOW_INDEXING=true`.

## Next steps, in order

1. **Real listings first.** Thin pages stay `noindex` until they have 3+ published profiles. Ask suppliers from your own network to register, because only self-registered or permission-backed data can be published.
2. **Cluster guides** (1,200–2,000 words each, original): Panipat (home textiles, rugs, recycled yarn), Bhadohi–Mirzapur (carpets), Morbi (tiles), Tiruppur (knitwear). Link each guide to its industry and state pages.
3. **Google Search Console + Bing Webmaster Tools** on day one. Submit `/sitemap.xml`. Then use `/seo google` to get real queries and impressions.
4. **Backlinks:** list Peoplvance on industry association sites and the export promotion councils' member resources (where permitted). Write guest articles for textile and rug trade publications.
5. **Measure:** after 8 weeks, re-run `/seo audit https://peoplvance.org`. Success signals are impressions on "[product] manufacturers in [city]" queries and indexed industry pages.

## Sources (live results checked 9 Nov 2026)

- https://www.indianyellowpages.com/ · https://dir.indiamart.com/ (city and category pages)
- https://ensun.io/search/contract-manufacturing/india · https://clutch.co/in/logistics/manufacturing-companies
- https://www.thetilesofindia.com/tile-companies/top-101-tiles-manufacturers-in-morbi/
- https://www.trade4asia.com/suppliers/cnc-vmc-job-work · https://us.kompass.com/
