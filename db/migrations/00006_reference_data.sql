-- +goose Up
-- Reference data that every environment needs. No NIC codes are assigned here: they must be
-- verified against the official MoSPI NIC publication before being recorded.

INSERT INTO indian_states (code, name, slug, region_type) VALUES
  ('AP', 'Andhra Pradesh', 'andhra-pradesh', 'state'),
  ('AR', 'Arunachal Pradesh', 'arunachal-pradesh', 'state'),
  ('AS', 'Assam', 'assam', 'state'),
  ('BR', 'Bihar', 'bihar', 'state'),
  ('CG', 'Chhattisgarh', 'chhattisgarh', 'state'),
  ('GA', 'Goa', 'goa', 'state'),
  ('GJ', 'Gujarat', 'gujarat', 'state'),
  ('HR', 'Haryana', 'haryana', 'state'),
  ('HP', 'Himachal Pradesh', 'himachal-pradesh', 'state'),
  ('JH', 'Jharkhand', 'jharkhand', 'state'),
  ('KA', 'Karnataka', 'karnataka', 'state'),
  ('KL', 'Kerala', 'kerala', 'state'),
  ('MP', 'Madhya Pradesh', 'madhya-pradesh', 'state'),
  ('MH', 'Maharashtra', 'maharashtra', 'state'),
  ('MN', 'Manipur', 'manipur', 'state'),
  ('ML', 'Meghalaya', 'meghalaya', 'state'),
  ('MZ', 'Mizoram', 'mizoram', 'state'),
  ('NL', 'Nagaland', 'nagaland', 'state'),
  ('OD', 'Odisha', 'odisha', 'state'),
  ('PB', 'Punjab', 'punjab', 'state'),
  ('RJ', 'Rajasthan', 'rajasthan', 'state'),
  ('SK', 'Sikkim', 'sikkim', 'state'),
  ('TN', 'Tamil Nadu', 'tamil-nadu', 'state'),
  ('TS', 'Telangana', 'telangana', 'state'),
  ('TR', 'Tripura', 'tripura', 'state'),
  ('UP', 'Uttar Pradesh', 'uttar-pradesh', 'state'),
  ('UK', 'Uttarakhand', 'uttarakhand', 'state'),
  ('WB', 'West Bengal', 'west-bengal', 'state'),
  ('AN', 'Andaman and Nicobar Islands', 'andaman-and-nicobar-islands', 'union_territory'),
  ('CH', 'Chandigarh', 'chandigarh', 'union_territory'),
  ('DH', 'Dadra and Nagar Haveli and Daman and Diu', 'dadra-and-nagar-haveli-and-daman-and-diu', 'union_territory'),
  ('DL', 'Delhi', 'delhi', 'union_territory'),
  ('JK', 'Jammu and Kashmir', 'jammu-and-kashmir', 'union_territory'),
  ('LA', 'Ladakh', 'ladakh', 'union_territory'),
  ('LD', 'Lakshadweep', 'lakshadweep', 'union_territory'),
  ('PY', 'Puducherry', 'puducherry', 'union_territory');

INSERT INTO business_roles (code, name, description, sort_order) VALUES
  ('manufacturer', 'Manufacturer', 'Produces goods in its own facility.', 10),
  ('contract_manufacturer', 'Contract manufacturer', 'Manufactures to another company''s specification and brand.', 20),
  ('oem', 'OEM', 'Original equipment manufacturer supplying parts or products sold under another brand.', 30),
  ('odm', 'ODM', 'Designs and manufactures products that other companies rebrand.', 40),
  ('raw_material_supplier', 'Raw material supplier', 'Supplies raw or semi-processed materials.', 50),
  ('component_supplier', 'Component supplier', 'Supplies parts and sub-assemblies.', 60),
  ('job_work_provider', 'Job work provider', 'Performs processing on materials supplied by the customer.', 70),
  ('sourcing_agent', 'Sourcing agent', 'Finds and manages suppliers on behalf of buyers.', 80),
  ('buying_house', 'Buying house', 'Sources, inspects and consolidates orders for overseas or domestic buyers.', 90),
  ('exporter', 'Exporter', 'Sells goods to buyers outside India.', 100),
  ('importer', 'Importer', 'Brings goods into India.', 110),
  ('wholesaler', 'Wholesaler', 'Sells in bulk to retailers or businesses.', 120),
  ('distributor', 'Distributor', 'Distributes products of one or more manufacturers in a territory.', 130),
  ('testing_lab', 'Testing lab', 'Tests materials or products against standards.', 140),
  ('industrial_service_provider', 'Industrial service provider', 'Provides maintenance, inspection, tooling or other industrial services.', 150);

-- Category tree. Top level first, children reference parents by slug.
INSERT INTO categories (name, slug, category_type, description, sort_order) VALUES
  ('Food and beverages', 'food-and-beverages', 'industry', 'Processing and packaging of food products and drinks, from traditional snacks and spices to dairy, bakery and edible oils.', 10),
  ('Textiles and apparel', 'textiles-and-apparel', 'industry', 'Fibre-to-finished-product value chain: yarn, fabric, dyeing and finishing, garments, home textiles, rugs and carpets.', 20),
  ('Building and construction materials', 'building-and-construction-materials', 'industry', 'Materials used in construction: tiles, bricks, blocks, cement, precast concrete, sanitaryware, stone, glass and pipes.', 30),
  ('Metals and fabrication', 'metals-and-fabrication', 'industry', 'Ferrous and non-ferrous metal production, casting, sheet metal work, fabrication and fasteners.', 40),
  ('Industrial machinery and components', 'industrial-machinery-and-components', 'industry', 'Machined parts, power transmission components, pumps, motors, conveyors, compressors, tools, dies and moulds.', 50),
  ('Plastics, rubber and packaging', 'plastics-rubber-and-packaging', 'industry', 'Moulded and extruded plastics, rubber products, and packaging materials such as flexible packaging, corrugated boxes and labels.', 60),
  ('Electrical and electronics', 'electrical-and-electronics', 'industry', 'Transformers, wiring, PCBs, control panels, sensors and electronic assembly.', 70),
  ('Automotive and transport', 'automotive-and-transport', 'industry', 'Components for road vehicles, railways and aerospace.', 80),
  ('Chemicals, pharma, cosmetics and medical devices', 'chemicals-pharma-cosmetics-medical', 'industry', 'Industrial and specialty chemicals, pharmaceutical formulations, cosmetics and medical devices. These sectors are heavily regulated; licences should be checked directly with the supplier and regulator.', 90),
  ('Furniture, wood and paper', 'furniture-wood-and-paper', 'industry', 'Furniture, wood products, paper and printing.', 100),
  ('Leather, footwear and handicrafts', 'leather-footwear-and-handicrafts', 'industry', 'Leather goods, footwear, handicrafts, jewellery and toys.', 110),
  ('Renewables and recycling', 'renewables-and-recycling', 'industry', 'Renewable energy equipment and material recycling.', 120),
  ('Industrial services', 'industrial-services', 'service', 'Testing, certification support, quality inspection, job work, maintenance, tooling and logistics support for manufacturers.', 130),
  ('Sourcing and trading', 'sourcing-and-trading', 'trade', 'Buying houses, sourcing agents, exporters, importers, wholesalers and distributors that connect buyers with manufacturers.', 140);

INSERT INTO categories (parent_id, name, slug, category_type, description, sort_order)
SELECT p.id, c.name, c.slug, c.category_type, c.description, c.sort_order
FROM (VALUES
  ('food-and-beverages', 'Namkeen and savoury snacks', 'namkeen-and-snacks', 'product_category', 'Traditional Indian savoury snacks such as bhujia, sev, mixtures and chips, typically fried or roasted and packed for retail or bulk supply.', 10),
  ('food-and-beverages', 'Bakery products', 'bakery-products', 'product_category', 'Breads, rusks, biscuits, cakes and other baked goods.', 20),
  ('food-and-beverages', 'Spices and masalas', 'spices-and-masalas', 'product_category', 'Whole spices, ground spices and blended masalas.', 30),
  ('food-and-beverages', 'Dairy products', 'dairy-products', 'product_category', 'Milk processing, ghee, paneer, butter, curd and other dairy products.', 40),
  ('food-and-beverages', 'Packaged foods', 'packaged-foods', 'product_category', 'Ready-to-eat and ready-to-cook packaged foods.', 50),
  ('food-and-beverages', 'Grain milling', 'grain-milling', 'product_category', 'Flour mills, rice mills, pulses processing and related milling.', 60),
  ('food-and-beverages', 'Edible oils', 'edible-oils', 'product_category', 'Extraction and refining of edible oils.', 70),
  ('food-and-beverages', 'Beverages', 'beverages', 'product_category', 'Packaged drinking water, juices, carbonated drinks, tea and coffee processing.', 80),
  ('textiles-and-apparel', 'Rugs and carpets', 'rugs-and-carpets', 'product_category', 'Hand-knotted, hand-tufted, flat-woven and machine-made rugs and carpets, a long-standing Indian export craft.', 10),
  ('textiles-and-apparel', 'Yarn', 'yarn', 'product_category', 'Spinning of cotton, wool, synthetic and blended yarns.', 20),
  ('textiles-and-apparel', 'Fabric', 'fabric', 'product_category', 'Woven and knitted fabric production.', 30),
  ('textiles-and-apparel', 'Garment manufacturing', 'garment-manufacturing', 'product_category', 'Cut-make-trim and full-package garment production.', 40),
  ('textiles-and-apparel', 'Home textiles', 'home-textiles', 'product_category', 'Bed linen, towels, curtains, cushions and table linen.', 50),
  ('textiles-and-apparel', 'Dyeing and finishing', 'dyeing-and-finishing', 'service', 'Dyeing, printing, washing and finishing of yarn, fabric and garments.', 60),
  ('building-and-construction-materials', 'Tiles', 'tiles', 'product_category', 'Ceramic, vitrified and porcelain tiles for floors and walls.', 10),
  ('building-and-construction-materials', 'Bricks', 'bricks', 'product_category', 'Clay bricks and fly-ash bricks for masonry construction.', 20),
  ('building-and-construction-materials', 'AAC and concrete blocks', 'aac-and-concrete-blocks', 'product_category', 'Autoclaved aerated concrete blocks and solid or hollow concrete blocks.', 30),
  ('building-and-construction-materials', 'Cement', 'cement', 'product_category', 'Portland and blended cements.', 40),
  ('building-and-construction-materials', 'Pavers and precast concrete', 'pavers-and-precast', 'product_category', 'Interlocking pavers, kerb stones and precast concrete elements.', 50),
  ('building-and-construction-materials', 'Sanitaryware', 'sanitaryware', 'product_category', 'Ceramic sanitaryware and bathroom fittings.', 60),
  ('building-and-construction-materials', 'Natural stone', 'natural-stone', 'product_category', 'Granite, marble, sandstone and other quarried stone.', 70),
  ('building-and-construction-materials', 'Glass', 'glass', 'product_category', 'Float, toughened, laminated and architectural glass.', 80),
  ('building-and-construction-materials', 'Pipes', 'pipes', 'product_category', 'PVC, HDPE, ductile iron, steel and concrete pipes.', 90),
  ('metals-and-fabrication', 'Iron and steel', 'iron-and-steel', 'product_category', 'Iron and steel production and rolled products.', 10),
  ('metals-and-fabrication', 'Aluminium', 'aluminium', 'product_category', 'Aluminium extrusion, casting and sheet.', 20),
  ('metals-and-fabrication', 'Brass and copper', 'brass-and-copper', 'product_category', 'Brass parts, copper products and related alloys.', 30),
  ('metals-and-fabrication', 'Foundries and castings', 'foundries-and-castings', 'product_category', 'Sand, investment and die casting of ferrous and non-ferrous metals.', 40),
  ('metals-and-fabrication', 'Sheet metal and fabrication', 'sheet-metal-and-fabrication', 'service', 'Cutting, bending, welding and assembly of sheet and structural metal.', 50),
  ('metals-and-fabrication', 'Fasteners', 'fasteners', 'product_category', 'Bolts, nuts, screws, rivets and washers.', 60),
  ('industrial-machinery-and-components', 'CNC machined components', 'cnc-machined-components', 'product_category', 'Precision parts produced on CNC turning and milling centres to customer drawings.', 10),
  ('industrial-machinery-and-components', 'Gears and shafts', 'gears-and-shafts', 'product_category', 'Gears, gearboxes, shafts and power transmission parts.', 20),
  ('industrial-machinery-and-components', 'Bearings', 'bearings', 'product_category', 'Ball, roller and plain bearings.', 30),
  ('industrial-machinery-and-components', 'Pumps and motors', 'pumps-and-motors', 'product_category', 'Industrial and agricultural pumps and electric motors.', 40),
  ('industrial-machinery-and-components', 'Conveyors and compressors', 'conveyors-and-compressors', 'product_category', 'Material handling conveyors and air compressors.', 50),
  ('industrial-machinery-and-components', 'Tools, dies and moulds', 'tools-dies-and-moulds', 'product_category', 'Cutting tools, press tools, dies and injection moulds.', 60),
  ('plastics-rubber-and-packaging', 'Injection moulding', 'injection-moulding', 'service', 'Injection-moulded plastic parts.', 10),
  ('plastics-rubber-and-packaging', 'Plastic extrusion', 'plastic-extrusion', 'service', 'Extruded profiles, sheets, films and pipes.', 20),
  ('plastics-rubber-and-packaging', 'Rubber products and seals', 'rubber-products-and-seals', 'product_category', 'Moulded rubber parts, seals, gaskets and hoses.', 30),
  ('plastics-rubber-and-packaging', 'Flexible packaging', 'flexible-packaging', 'product_category', 'Printed laminates, pouches and films.', 40),
  ('plastics-rubber-and-packaging', 'Corrugated boxes', 'corrugated-boxes', 'product_category', 'Corrugated board and cartons.', 50),
  ('plastics-rubber-and-packaging', 'Labels', 'labels', 'product_category', 'Printed self-adhesive and wrap-around labels.', 60),
  ('electrical-and-electronics', 'Transformers', 'transformers', 'product_category', 'Distribution, power and instrument transformers.', 10),
  ('electrical-and-electronics', 'Wires and cables', 'wires-and-cables', 'product_category', 'Building wire, power and control cables.', 20),
  ('electrical-and-electronics', 'PCBs and electronic assembly', 'pcbs-and-electronic-assembly', 'service', 'Printed circuit board fabrication and assembly.', 30),
  ('electrical-and-electronics', 'Control panels', 'control-panels', 'product_category', 'Electrical control and distribution panels.', 40),
  ('electrical-and-electronics', 'Sensors', 'sensors', 'product_category', 'Industrial sensors and instrumentation.', 50),
  ('automotive-and-transport', 'Auto components', 'auto-components', 'product_category', 'Parts for two-wheelers, cars and commercial vehicles.', 10),
  ('automotive-and-transport', 'Railway components', 'railway-components', 'product_category', 'Components for rolling stock and track.', 20),
  ('automotive-and-transport', 'Aerospace components', 'aerospace-components', 'product_category', 'Precision components for aerospace and defence.', 30),
  ('chemicals-pharma-cosmetics-medical', 'Chemicals', 'chemicals', 'product_category', 'Industrial and specialty chemicals.', 10),
  ('chemicals-pharma-cosmetics-medical', 'Pharmaceuticals', 'pharmaceuticals', 'product_category', 'Formulations and APIs.', 20),
  ('chemicals-pharma-cosmetics-medical', 'Cosmetics and personal care', 'cosmetics-and-personal-care', 'product_category', 'Cosmetics, toiletries and personal care products.', 30),
  ('chemicals-pharma-cosmetics-medical', 'Medical devices', 'medical-devices', 'product_category', 'Medical devices and consumables.', 40),
  ('furniture-wood-and-paper', 'Furniture', 'furniture', 'product_category', 'Home, office and institutional furniture.', 10),
  ('furniture-wood-and-paper', 'Paper and printing', 'paper-and-printing', 'product_category', 'Paper products and commercial printing.', 20),
  ('leather-footwear-and-handicrafts', 'Leather goods', 'leather-goods', 'product_category', 'Bags, belts, wallets and other leather goods.', 10),
  ('leather-footwear-and-handicrafts', 'Footwear', 'footwear', 'product_category', 'Leather and non-leather footwear.', 20),
  ('leather-footwear-and-handicrafts', 'Handicrafts', 'handicrafts', 'product_category', 'Handmade decor, metalware, woodcraft and textiles.', 30),
  ('leather-footwear-and-handicrafts', 'Jewellery', 'jewellery', 'product_category', 'Gold, silver and fashion jewellery.', 40),
  ('leather-footwear-and-handicrafts', 'Toys', 'toys', 'product_category', 'Wooden, plastic and soft toys.', 50),
  ('renewables-and-recycling', 'Solar equipment', 'solar-equipment', 'product_category', 'Solar modules, mounting structures and balance-of-system components.', 10),
  ('renewables-and-recycling', 'Recycling', 'recycling', 'service', 'Recycling of metals, plastics, paper and e-waste.', 20),
  ('industrial-services', 'Testing and inspection', 'testing-and-inspection', 'service', 'Material testing, product testing and third-party quality inspection.', 10),
  ('industrial-services', 'Job work', 'job-work', 'service', 'Processing of customer-supplied material: machining, heat treatment, plating, painting and similar.', 20),
  ('industrial-services', 'Maintenance and tooling', 'maintenance-and-tooling', 'service', 'Plant maintenance, tool rooms and repair services.', 30),
  ('industrial-services', 'Logistics support', 'logistics-support', 'service', 'Packing, warehousing and freight forwarding support for manufacturers.', 40),
  ('sourcing-and-trading', 'Buying houses', 'buying-houses', 'trade', 'Firms that source, inspect and consolidate orders for buyers.', 10),
  ('sourcing-and-trading', 'Sourcing agents', 'sourcing-agents', 'trade', 'Individuals or firms that identify and manage suppliers for buyers.', 20),
  ('sourcing-and-trading', 'Exporters and importers', 'exporters-and-importers', 'trade', 'Merchant exporters and importers.', 30),
  ('sourcing-and-trading', 'Wholesalers and distributors', 'wholesalers-and-distributors', 'trade', 'Bulk sellers and territory distributors.', 40)
) AS c(parent_slug, name, slug, category_type, description, sort_order)
JOIN categories p ON p.slug = c.parent_slug;

-- Third level examples.
INSERT INTO categories (parent_id, name, slug, category_type, description, sort_order)
SELECT p.id, c.name, c.slug, 'product_category', c.description, c.sort_order
FROM (VALUES
  ('tiles', 'Ceramic tiles', 'ceramic-tiles', 'Pressed and fired clay-based tiles, usually glazed, for walls and light-traffic floors.', 10),
  ('tiles', 'Vitrified tiles', 'vitrified-tiles', 'Low-porosity tiles fired at high temperature; includes double-charge, GVT and PGVT.', 20),
  ('tiles', 'Porcelain tiles', 'porcelain-tiles', 'Dense, very low water-absorption tiles suited to heavy-traffic and outdoor use.', 30),
  ('bricks', 'Clay bricks', 'clay-bricks', 'Kiln-fired bricks made from clay.', 10),
  ('bricks', 'Fly-ash bricks', 'fly-ash-bricks', 'Bricks made from fly ash, lime/cement and sand, typically cured rather than fired.', 20),
  ('rugs-and-carpets', 'Hand-knotted rugs', 'hand-knotted-rugs', 'Rugs where each knot is tied by hand on a loom.', 10),
  ('rugs-and-carpets', 'Hand-tufted rugs', 'hand-tufted-rugs', 'Rugs made by punching yarn through a backing with a tufting gun.', 20),
  ('rugs-and-carpets', 'Flat-weave rugs and dhurries', 'flat-weave-rugs-and-dhurries', 'Pile-free woven rugs, including dhurries and kilims.', 30)
) AS c(parent_slug, name, slug, description, sort_order)
JOIN categories p ON p.slug = c.parent_slug;

INSERT INTO capabilities (slug, name, capability_type, description) VALUES
  ('cnc-turning', 'CNC turning', 'process', 'Turning on CNC lathes.'),
  ('cnc-milling', 'CNC milling', 'process', 'Milling on CNC machining centres (3/4/5-axis).'),
  ('heat-treatment', 'Heat treatment', 'process', 'Hardening, tempering, annealing, case hardening.'),
  ('surface-finishing', 'Surface finishing', 'process', 'Plating, anodising, powder coating, painting.'),
  ('welding', 'Welding', 'process', 'MIG, TIG, arc and spot welding.'),
  ('injection-moulding-process', 'Injection moulding', 'process', 'Plastic injection moulding.'),
  ('hand-knotting', 'Hand knotting', 'process', 'Hand-knotted rug weaving.'),
  ('hand-tufting', 'Hand tufting', 'process', 'Hand-tufted rug production.'),
  ('frying-and-roasting', 'Frying and roasting', 'process', 'Continuous or batch frying/roasting for snacks.'),
  ('nitrogen-flush-packing', 'Nitrogen-flush packing', 'process', 'Modified atmosphere packing for snacks.'),
  ('tile-pressing-and-kiln', 'Tile pressing and kiln firing', 'process', 'Hydraulic pressing and roller kiln firing of tiles.'),
  ('digital-glazing', 'Digital glaze printing', 'process', 'Inkjet decoration of tiles.'),
  ('brick-kiln', 'Brick kiln firing', 'process', 'Clamp, zig-zag or tunnel kiln firing of clay bricks.'),
  ('fly-ash-brick-press', 'Fly-ash brick pressing', 'equipment', 'Hydraulic or mechanical fly-ash brick presses.'),
  ('cmm-inspection', 'CMM inspection', 'quality', 'Coordinate measuring machine inspection.'),
  ('in-house-lab', 'In-house testing lab', 'quality', 'On-site laboratory for incoming and final inspection.'),
  ('third-party-inspection', 'Third-party inspection coordination', 'service', 'Arranging independent pre-shipment inspection.'),
  ('export-documentation', 'Export documentation', 'service', 'Handling export paperwork and shipping documents.'),
  ('private-label', 'Private label production', 'service', 'Manufacturing under the buyer''s brand.'),
  ('design-development', 'Design and sampling', 'service', 'Developing designs and samples for buyers.');

-- +goose Down
DELETE FROM capabilities;
DELETE FROM categories WHERE parent_id IS NOT NULL AND parent_id IN (SELECT id FROM categories WHERE parent_id IS NOT NULL);
DELETE FROM categories WHERE parent_id IS NOT NULL;
DELETE FROM categories;
DELETE FROM business_roles;
DELETE FROM indian_states;
