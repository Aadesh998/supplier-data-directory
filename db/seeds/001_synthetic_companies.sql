-- SYNTHETIC TEST DATA. These are NOT real suppliers. Names carry "(Synthetic)", websites use the
-- reserved .example TLD, and no contact details are included. Records are approved_private and
-- their source does not permit public display, so they never appear on public pages unless the
-- development-only demo seed (002) is applied.
DO $$
DECLARE
  src uuid;
  cid uuid;
  fid uuid;
  pid uuid;
  r record;
  role text;
  cat text;
  cap text;
  prod text;
BEGIN
  IF EXISTS (SELECT 1 FROM data_sources WHERE provider = 'synthetic-seed') THEN
    RAISE NOTICE 'synthetic seed already applied';
    RETURN;
  END IF;

  INSERT INTO data_sources (source_type, provider, license_reference, allowed_internal_use, notes, restrictions)
  VALUES ('manual_verification', 'synthetic-seed', 'internal-test-fixture', true,
          'Synthetic records generated for development and automated tests. Not real businesses.',
          '{"synthetic": true}'::jsonb)
  RETURNING id INTO src;

  FOR r IN SELECT * FROM (VALUES
    ('Kalin Weaves (Synthetic)', 'Kalin Weaves Private Limited (Synthetic)', 'Synthetic example of a hand-knotted rug manufacturer.', 'UP', 'Bhadohi', 1998, ARRAY['manufacturer','exporter'], ARRAY['hand-knotted-rugs'], ARRAY['hand-knotting','design-development'], ARRAY['Hand-knotted wool rug','Silk blend rug']),
    ('Tuftline Rugs (Synthetic)', 'Tuftline Rugs LLP (Synthetic)', 'Synthetic example of a hand-tufted rug maker supplying private label buyers.', 'UP', 'Mirzapur', 2006, ARRAY['manufacturer','contract_manufacturer'], ARRAY['hand-tufted-rugs'], ARRAY['hand-tufting','private-label'], ARRAY['Hand-tufted wool rug']),
    ('Dhurrie Loom House (Synthetic)', NULL, 'Synthetic example of a flat-weave dhurrie workshop.', 'RJ', 'Jaipur', 2011, ARRAY['manufacturer'], ARRAY['flat-weave-rugs-and-dhurries'], ARRAY['design-development'], ARRAY['Cotton dhurrie','Jute kilim']),
    ('Panipat Floor Textiles (Synthetic)', NULL, 'Synthetic example of a rug and home textile exporter.', 'HR', 'Panipat', 2001, ARRAY['manufacturer','exporter'], ARRAY['rugs-and-carpets','home-textiles'], ARRAY['export-documentation'], ARRAY['Recycled PET rug','Cushion covers']),
    ('Bikaneri Bhujia Works (Synthetic)', 'Bikaneri Bhujia Works Private Limited (Synthetic)', 'Synthetic example of a traditional namkeen manufacturer.', 'RJ', 'Bikaner', 1985, ARRAY['manufacturer','wholesaler'], ARRAY['namkeen-and-snacks'], ARRAY['frying-and-roasting','nitrogen-flush-packing'], ARRAY['Bikaneri bhujia','Moong dal']),
    ('Indore Sev Company (Synthetic)', NULL, 'Synthetic example of a sev and mixture producer.', 'MP', 'Indore', 1994, ARRAY['manufacturer'], ARRAY['namkeen-and-snacks'], ARRAY['frying-and-roasting'], ARRAY['Ratlami sev','Khatta meetha mixture']),
    ('Crunch Contract Foods (Synthetic)', 'Crunch Contract Foods LLP (Synthetic)', 'Synthetic example of a contract snack manufacturer for private labels.', 'GJ', 'Rajkot', 2015, ARRAY['contract_manufacturer','manufacturer'], ARRAY['namkeen-and-snacks','packaged-foods'], ARRAY['private-label','nitrogen-flush-packing'], ARRAY['Private label namkeen']),
    ('Masala Trail Snacks (Synthetic)', NULL, 'Synthetic example of a regional snack and spice blender.', 'MH', 'Pune', 2009, ARRAY['manufacturer','distributor'], ARRAY['namkeen-and-snacks','spices-and-masalas'], ARRAY['frying-and-roasting'], ARRAY['Chakli','Chivda masala']),
    ('Morvi Vitrified (Synthetic)', 'Morvi Vitrified Tiles Private Limited (Synthetic)', 'Synthetic example of a vitrified tile manufacturer.', 'GJ', 'Morbi', 2004, ARRAY['manufacturer','exporter'], ARRAY['vitrified-tiles'], ARRAY['tile-pressing-and-kiln','digital-glazing'], ARRAY['Glazed vitrified tile 600x1200','Double charge tile']),
    ('Ceramica Wall Tiles (Synthetic)', NULL, 'Synthetic example of a ceramic wall tile plant.', 'GJ', 'Morbi', 2010, ARRAY['manufacturer'], ARRAY['ceramic-tiles'], ARRAY['tile-pressing-and-kiln'], ARRAY['Ceramic wall tile 300x450']),
    ('Porcelain Slab Works (Synthetic)', NULL, 'Synthetic example of a porcelain slab producer.', 'RJ', 'Kishangarh', 2018, ARRAY['manufacturer','oem'], ARRAY['porcelain-tiles'], ARRAY['tile-pressing-and-kiln','digital-glazing'], ARRAY['Porcelain slab 800x1600']),
    ('Ganga Clay Bricks (Synthetic)', NULL, 'Synthetic example of a clay brick kiln.', 'BR', 'Patna', 1990, ARRAY['manufacturer'], ARRAY['clay-bricks'], ARRAY['brick-kiln'], ARRAY['Red clay brick']),
    ('Ashblock Industries (Synthetic)', 'Ashblock Industries Private Limited (Synthetic)', 'Synthetic example of a fly-ash brick and block maker.', 'TN', 'Coimbatore', 2012, ARRAY['manufacturer','distributor'], ARRAY['fly-ash-bricks','aac-and-concrete-blocks'], ARRAY['fly-ash-brick-press'], ARRAY['Fly-ash brick','Solid concrete block']),
    ('Deccan Brick Co (Synthetic)', NULL, 'Synthetic example of a brick supplier.', 'TS', 'Hyderabad', 2003, ARRAY['manufacturer','wholesaler'], ARRAY['bricks'], ARRAY['brick-kiln'], ARRAY['Wire-cut brick']),
    ('Precision Turn Components (Synthetic)', 'Precision Turn Components Private Limited (Synthetic)', 'Synthetic example of a CNC turned components job shop.', 'TN', 'Coimbatore', 2008, ARRAY['manufacturer','job_work_provider','component_supplier'], ARRAY['cnc-machined-components'], ARRAY['cnc-turning','cnc-milling','cmm-inspection'], ARRAY['Turned shaft','Precision bush']),
    ('Pune Five Axis (Synthetic)', NULL, 'Synthetic example of a 5-axis machining specialist.', 'MH', 'Pune', 2016, ARRAY['manufacturer','component_supplier'], ARRAY['cnc-machined-components','aerospace-components'], ARRAY['cnc-milling','cmm-inspection','in-house-lab'], ARRAY['Aluminium housing','Impeller']),
    ('Rajkot Machine Parts (Synthetic)', NULL, 'Synthetic example of a machined auto component supplier.', 'GJ', 'Rajkot', 1999, ARRAY['oem','component_supplier','job_work_provider'], ARRAY['cnc-machined-components','auto-components'], ARRAY['cnc-turning','heat-treatment','surface-finishing'], ARRAY['Brake drum machining','Flange']),
    ('Delhi Sourcing Partners (Synthetic)', 'Delhi Sourcing Partners LLP (Synthetic)', 'Synthetic example of a sourcing agent for home textiles and handicrafts.', 'DL', 'New Delhi', 2007, ARRAY['sourcing_agent','exporter'], ARRAY['sourcing-agents','handicrafts'], ARRAY['third-party-inspection','export-documentation'], ARRAY[]::text[]),
    ('Tiruppur Buying House (Synthetic)', NULL, 'Synthetic example of a garment buying house.', 'TN', 'Tiruppur', 2002, ARRAY['buying_house','sourcing_agent'], ARRAY['buying-houses','garment-manufacturing'], ARRAY['third-party-inspection'], ARRAY[]::text[]),
    ('Mumbai Industrial Traders (Synthetic)', NULL, 'Synthetic example of an industrial components distributor and importer.', 'MH', 'Mumbai', 1996, ARRAY['distributor','importer','wholesaler'], ARRAY['wholesalers-and-distributors','bearings'], ARRAY[]::text[], ARRAY['Imported bearings'])
  ) AS t(display_name, legal_name, description, state_code, city, est_year, roles, cats, caps, prods)
  LOOP
    INSERT INTO companies (display_name, legal_name, slug, normalized_name, description, website_url, website_domain,
                           establishment_year, business_status, publication_status, primary_category_id)
    VALUES (
      r.display_name, r.legal_name,
      trim(both '-' from regexp_replace(lower(r.display_name), '[^a-z0-9]+', '-', 'g')),
      trim(regexp_replace(regexp_replace(lower(r.display_name), '\(synthetic\)|private limited|pvt ltd|llp|co\b', '', 'g'), '[^a-z0-9]+', ' ', 'g')),
      r.description,
      'https://' || trim(both '-' from regexp_replace(lower(r.display_name), '[^a-z0-9]+', '-', 'g')) || '.example',
      trim(both '-' from regexp_replace(lower(r.display_name), '[^a-z0-9]+', '-', 'g')) || '.example',
      r.est_year, 'active', 'approved_private',
      (SELECT id FROM categories WHERE slug = r.cats[1]))
    RETURNING id INTO cid;

    INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status, confidence)
    SELECT cid, 'company', cid, f, src, 'permitted', 1.0
    FROM unnest(ARRAY['display_name','legal_name','description','website_url','establishment_year']) AS f;

    INSERT INTO facilities (company_id, facility_type, city, state_code, is_primary, source_id)
    VALUES (cid, 'factory', r.city, r.state_code, true, src)
    RETURNING id INTO fid;
    INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
    SELECT cid, 'facility', fid, f, src, 'permitted' FROM unnest(ARRAY['city','state_code','facility_type']) AS f;

    FOREACH role IN ARRAY r.roles LOOP
      INSERT INTO company_roles (company_id, role_code, source_id) VALUES (cid, role, src);
    END LOOP;
    FOREACH cat IN ARRAY r.cats LOOP
      INSERT INTO company_categories (company_id, category_id, source_id, confidence)
      SELECT cid, id, src, 1.0 FROM categories WHERE slug = cat;
    END LOOP;
    FOREACH cap IN ARRAY r.caps LOOP
      INSERT INTO company_capabilities (company_id, capability_id, source_id)
      SELECT cid, id, src FROM capabilities WHERE slug = cap;
    END LOOP;
    FOREACH prod IN ARRAY r.prods LOOP
      INSERT INTO products (company_id, category_id, name, normalized_name, source_id)
      VALUES (cid, (SELECT id FROM categories WHERE slug = r.cats[1]), prod, lower(prod), src)
      RETURNING id INTO pid;
      INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
      VALUES (cid, 'product', pid, 'name', src, 'permitted');
    END LOOP;
  END LOOP;
END $$;
