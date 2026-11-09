// Package importer parses, maps and normalises CSV rows for the staging pipeline.
// It has no database dependency so every transformation is unit-testable.
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bharatmfg/directory/internal/normalize"
)

// MaxRows bounds a single import.
const MaxRows = 20000

// Field is a mappable target field.
type Field struct {
	Field       string `json:"field"`
	Label       string `json:"label"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

// Fields lists mapping targets, in UI order.
var Fields = []Field{
	{"display_name", "Company name", true, "Trading or display name."},
	{"legal_name", "Legal name", false, "Registered legal name, if different."},
	{"gstin", "GSTIN", false, "15-character GST number. Kept private; used for duplicate checks and to fill a missing state."},
	{"description", "Description", false, "Short factual description."},
	{"website_url", "Website", false, "http(s) address; invalid values are dropped with a warning."},
	{"establishment_year", "Year established", false, "Four-digit year."},
	{"roles", "Business roles", false, "Separated by comma/semicolon/pipe, e.g. 'manufacturer; exporter'."},
	{"categories", "Categories", false, "Category names or slugs, separated by comma/semicolon/pipe."},
	{"products", "Products", false, "Product names separated by semicolon or pipe."},
	{"facility_type", "Facility type", false, "factory, office, warehouse... (default factory)."},
	{"address_line1", "Address", false, "Street address."},
	{"locality", "Locality / area", false, ""},
	{"city", "City / town", false, ""},
	{"district", "District", false, ""},
	{"state", "State / UT", false, "Name, common alias or ISO code."},
	{"postal_code", "PIN code", false, "6-digit Indian PIN."},
	{"business_email", "Email", false, "Stored privately. Role addresses (info@, sales@) are kept as business email; others as personal (encrypted)."},
	{"business_phone", "Phone", false, "Stored privately and encrypted. Never published from imports."},
}

var synonyms = map[string][]string{
	"display_name":       {"company name", "company", "name", "business name", "firm name", "firm", "supplier name", "supplier", "display name", "trade name"},
	"legal_name":         {"legal name", "registered name", "legal entity", "registered company name"},
	"gstin":              {"gstin", "gst", "gst number", "gst no", "gstin number", "gst in", "gstno", "gstnumber"},
	"description":        {"description", "about", "profile", "business description", "details"},
	"website_url":        {"website", "web site", "url", "web", "website url", "site"},
	"establishment_year": {"year established", "established", "year of establishment", "establishment year", "since", "founded", "year"},
	"roles":              {"business type", "type", "role", "roles", "nature of business", "business role", "company type"},
	"categories":         {"category", "categories", "industry", "sector", "segment", "sub category", "subcategory"},
	"products":           {"products", "product", "items", "product list", "products manufactured"},
	"facility_type":      {"facility type", "unit type", "premises type"},
	"address_line1":      {"address", "street address", "address line 1", "address1", "full address"},
	"locality":           {"locality", "area", "industrial area", "estate", "sector area"},
	"city":               {"city", "town", "city town", "location"},
	"district":           {"district", "dist"},
	"state":              {"state", "state name", "province", "region", "ut"},
	"postal_code":        {"pincode", "pin code", "pin", "zip", "postal code", "zip code", "postcode"},
	"business_email":     {"email", "e mail", "email id", "email address", "mail"},
	"business_phone":     {"phone", "mobile", "contact number", "phone number", "telephone", "mobile number", "contact no", "tel"},
}

func headerKey(h string) string {
	return strings.TrimSpace(strings.Join(strings.FieldsFunc(strings.ToLower(h), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}), " "))
}

// AutoMap guesses a mapping target -> header from header names.
func AutoMap(headers []string) map[string]string {
	m := map[string]string{}
	used := map[string]bool{}
	for _, f := range Fields {
		for _, syn := range synonyms[f.Field] {
			for _, h := range headers {
				if !used[h] && headerKey(h) == syn {
					m[f.Field] = h
					used[h] = true
					break
				}
			}
			if _, ok := m[f.Field]; ok {
				break
			}
		}
	}
	return m
}

// ValidateMapping checks targets exist, headers exist, and the required field is mapped.
func ValidateMapping(m map[string]string, headers []string) error {
	hs := map[string]bool{}
	for _, h := range headers {
		hs[h] = true
	}
	known := map[string]bool{}
	for _, f := range Fields {
		known[f.Field] = true
	}
	var problems []string
	for target, header := range m {
		if !known[target] {
			problems = append(problems, fmt.Sprintf("unknown target field %q", target))
		}
		if header != "" && !hs[header] {
			problems = append(problems, fmt.Sprintf("column %q not found in file", header))
		}
	}
	if m["display_name"] == "" {
		problems = append(problems, "the company name (display_name) column must be mapped")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Parsed is a parsed CSV file.
type Parsed struct {
	Headers []string
	Rows    []map[string]string // keyed by header; row index i is CSV line i+2
}

// Parse reads a CSV (comma, semicolon or tab delimited; UTF-8 with optional BOM).
func Parse(data []byte) (Parsed, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if !utf8.Valid(data) {
		return Parsed{}, errors.New("file is not valid UTF-8; please export the spreadsheet as 'CSV UTF-8'")
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return Parsed{}, errors.New("file is empty")
	}
	firstLine := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		firstLine = data[:i]
	}
	delim := ','
	best := bytes.Count(firstLine, []byte{','})
	for _, d := range []rune{';', '\t'} {
		if n := bytes.Count(firstLine, []byte(string(d))); n > best {
			best, delim = n, d
		}
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.ReuseRecord = false
	header, err := r.Read()
	if err != nil {
		return Parsed{}, fmt.Errorf("could not read header row: %w", err)
	}
	p := Parsed{}
	seen := map[string]int{}
	for i, h := range header {
		h = normalize.Whitespace(h)
		if h == "" {
			h = fmt.Sprintf("column_%d", i+1)
		}
		seen[h]++
		if seen[h] > 1 {
			h = fmt.Sprintf("%s_%d", h, seen[h])
		}
		p.Headers = append(p.Headers, h)
	}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Parsed{}, fmt.Errorf("malformed CSV near line %d: %w", len(p.Rows)+2, err)
		}
		row := map[string]string{}
		empty := true
		for i, v := range rec {
			if i >= len(p.Headers) {
				if strings.TrimSpace(v) != "" {
					row[fmt.Sprintf("extra_column_%d", i+1)] = v
				}
				continue
			}
			if strings.TrimSpace(v) != "" {
				empty = false
			}
			row[p.Headers[i]] = v
		}
		if empty {
			continue
		}
		p.Rows = append(p.Rows, row)
		if len(p.Rows) > MaxRows {
			return Parsed{}, fmt.Errorf("file has more than %d data rows; split it into smaller files", MaxRows)
		}
	}
	if len(p.Rows) == 0 {
		return Parsed{}, errors.New("file has a header but no data rows")
	}
	return p, nil
}

// Lookups provides reference data for mapping free text to codes.
type Lookups struct {
	Roles      map[string]bool   // role codes
	Categories map[string]string // lowercased name or slug -> slug
}

var roleAliases = map[string]string{
	"manufacturing": "manufacturer", "mfr": "manufacturer", "mfg": "manufacturer", "maker": "manufacturer", "producer": "manufacturer",
	"export": "exporter", "exporters": "exporter", "import": "importer", "importers": "importer",
	"trader": "wholesaler", "trading": "wholesaler", "wholesale": "wholesaler", "dealer": "distributor", "distribution": "distributor",
	"stockist": "distributor", "agent": "sourcing_agent", "sourcing": "sourcing_agent", "buying agent": "sourcing_agent",
	"contract manufacturing": "contract_manufacturer", "job work": "job_work_provider", "jobwork": "job_work_provider",
	"service provider": "industrial_service_provider", "testing": "testing_lab", "lab": "testing_lab", "oem supplier": "oem",
}

// Issue is a validation error or warning on a field.
type Issue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Facility is the normalised facility part of a row.
type Facility struct {
	FacilityType string `json:"facility_type"`
	AddressLine1 string `json:"address_line1,omitempty"`
	Locality     string `json:"locality,omitempty"`
	City         string `json:"city,omitempty"`
	District     string `json:"district,omitempty"`
	StateCode    string `json:"state_code,omitempty"`
	PostalCode   string `json:"postal_code,omitempty"`
}

// Normalized is a cleaned row ready to commit.
type Normalized struct {
	DisplayName       string   `json:"display_name,omitempty"`
	NormalizedName    string   `json:"normalized_name,omitempty"`
	LegalName         string   `json:"legal_name,omitempty"`
	GSTIN             string   `json:"gstin,omitempty"`
	Description       string   `json:"description,omitempty"`
	WebsiteURL        string   `json:"website_url,omitempty"`
	WebsiteDomain     string   `json:"website_domain,omitempty"`
	EstablishmentYear int      `json:"establishment_year,omitempty"`
	Roles             []string `json:"roles,omitempty"`
	CategorySlugs     []string `json:"category_slugs,omitempty"`
	Products          []string `json:"products,omitempty"`
	Facility          Facility `json:"facility"`
	BusinessEmails    []string `json:"business_emails,omitempty"`
	PersonalEmails    []string `json:"personal_emails,omitempty"`
	Phones            []string `json:"phones,omitempty"`
}

// Result is the outcome of normalising one row.
type Result struct {
	Normalized Normalized
	Errors     []Issue
	Warnings   []Issue
	Log        []string
}

func splitList(s string, seps string) []string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return strings.ContainsRune(seps, r) })
	var out []string
	seen := map[string]bool{}
	for _, p := range parts {
		p = normalize.Whitespace(p)
		if p != "" && !seen[strings.ToLower(p)] {
			seen[strings.ToLower(p)] = true
			out = append(out, p)
		}
	}
	return out
}

var roleEmailPrefixes = []string{"info", "sales", "contact", "enquiry", "enquiries", "inquiry", "export", "exports", "office",
	"admin", "support", "hello", "marketing", "mail", "business", "orders", "purchase", "accounts", "care", "customercare"}

// IsRoleEmail reports whether an email is a role/department address rather than a person's.
func IsRoleEmail(email string) bool {
	local := strings.SplitN(email, "@", 2)[0]
	for _, p := range roleEmailPrefixes {
		if local == p {
			return true
		}
	}
	return false
}

// NormalizeRow maps and cleans one raw row. It never guesses missing data.
func NormalizeRow(raw map[string]string, mapping map[string]string, lk Lookups) Result {
	res := Result{}
	get := func(field string) string {
		h := mapping[field]
		if h == "" {
			return ""
		}
		return raw[h]
	}
	logf := func(format string, args ...any) { res.Log = append(res.Log, fmt.Sprintf(format, args...)) }
	warn := func(field, msg string) { res.Warnings = append(res.Warnings, Issue{field, msg}) }
	n := &res.Normalized

	if v := normalize.Whitespace(get("display_name")); v != "" {
		n.DisplayName = v
		n.NormalizedName = normalize.CompanyName(v)
		if v != get("display_name") {
			logf("display_name: whitespace normalised")
		}
		if len(v) > 300 {
			res.Errors = append(res.Errors, Issue{"display_name", "is longer than 300 characters"})
		}
	} else {
		res.Errors = append(res.Errors, Issue{"display_name", "is required"})
	}
	n.LegalName = normalize.Whitespace(get("legal_name"))
	if raw := get("gstin"); strings.TrimSpace(raw) != "" {
		if g, ok := normalize.GSTIN(raw); ok {
			n.GSTIN = g
		} else {
			warn("gstin", fmt.Sprintf("%q is not a valid 15-character GSTIN and was dropped", strings.TrimSpace(raw)))
		}
	}
	n.Description = strings.TrimSpace(get("description"))
	if len(n.Description) > 5000 {
		n.Description = n.Description[:5000]
		warn("description", "truncated to 5000 characters")
	}
	if raw := get("website_url"); strings.TrimSpace(raw) != "" {
		if u, d, ok := normalize.WebsiteURL(raw); ok {
			n.WebsiteURL, n.WebsiteDomain = u, d
			if u != strings.TrimSpace(raw) {
				logf("website_url: %q -> %q", strings.TrimSpace(raw), u)
			}
		} else {
			warn("website_url", fmt.Sprintf("%q is not a valid public website and was dropped", normalize.Whitespace(raw)))
		}
	}
	if raw := normalize.Whitespace(get("establishment_year")); raw != "" {
		if y, err := strconv.Atoi(raw); err == nil && y >= 1800 && y <= 2100 {
			n.EstablishmentYear = y
		} else {
			warn("establishment_year", fmt.Sprintf("%q is not a valid year and was dropped", raw))
		}
	}
	for _, r := range splitList(get("roles"), ",;|/") {
		key := strings.ToLower(r)
		code := strings.ReplaceAll(key, " ", "_")
		if alias, ok := roleAliases[key]; ok {
			code = alias
		}
		code = strings.TrimSuffix(code, "s")
		if !lk.Roles[code] && lk.Roles[code+"s"] {
			code += "s"
		}
		if lk.Roles[code] {
			if !contains(n.Roles, code) {
				n.Roles = append(n.Roles, code)
			}
			if code != key {
				logf("roles: %q -> %s", r, code)
			}
		} else {
			warn("roles", fmt.Sprintf("unrecognised role %q was ignored", r))
		}
	}
	for _, c := range splitList(get("categories"), ",;|") {
		key := strings.ToLower(c)
		slug, ok := lk.Categories[key]
		if !ok {
			slug, ok = lk.Categories[normalize.Slug(c)]
		}
		if ok {
			if !contains(n.CategorySlugs, slug) {
				n.CategorySlugs = append(n.CategorySlugs, slug)
			}
		} else {
			warn("categories", fmt.Sprintf("no matching category for %q; map it manually after import", c))
		}
	}
	n.Products = splitList(get("products"), ";|\n")
	if len(n.Products) > 30 {
		n.Products = n.Products[:30]
		warn("products", "only the first 30 products were kept")
	}
	f := &n.Facility
	f.FacilityType = "factory"
	if ft := strings.ToLower(normalize.Whitespace(get("facility_type"))); ft != "" {
		switch ft {
		case "factory", "office", "registered_office", "warehouse", "showroom", "workshop", "laboratory", "other":
			f.FacilityType = ft
		case "plant", "unit", "works", "manufacturing unit":
			f.FacilityType = "factory"
			logf("facility_type: %q -> factory", ft)
		case "head office", "corporate office", "ho":
			f.FacilityType = "office"
			logf("facility_type: %q -> office", ft)
		default:
			f.FacilityType = "other"
			warn("facility_type", fmt.Sprintf("unrecognised facility type %q stored as 'other'", ft))
		}
	}
	f.AddressLine1 = normalize.Whitespace(get("address_line1"))
	f.Locality = normalize.Title(get("locality"))
	f.City = normalize.Title(get("city"))
	f.District = normalize.Title(get("district"))
	if s := normalize.Whitespace(get("state")); s != "" {
		if code, ok := normalize.StateCode(s); ok {
			f.StateCode = code
			if code != s {
				logf("state: %q -> %s", s, code)
			}
		} else {
			warn("state", fmt.Sprintf("unrecognised state %q was dropped", s))
		}
	}
	if f.StateCode == "" && n.GSTIN != "" {
		if code, ok := normalize.StateFromGSTIN(n.GSTIN); ok {
			f.StateCode = code
			logf("state: %s inferred from GSTIN prefix", code)
		}
	}
	if p := normalize.Whitespace(get("postal_code")); p != "" {
		if pin, ok := normalize.PostalCode(p); ok {
			f.PostalCode = pin
		} else {
			warn("postal_code", fmt.Sprintf("%q is not a valid 6-digit PIN and was dropped", p))
		}
	}
	for _, e := range splitList(get("business_email"), ",;| ") {
		if email, ok := normalize.Email(e); ok {
			if IsRoleEmail(email) {
				n.BusinessEmails = append(n.BusinessEmails, email)
			} else {
				n.PersonalEmails = append(n.PersonalEmails, email)
				logf("email: treated as personal (stored encrypted, never public)")
			}
		} else {
			warn("business_email", "an invalid email address was dropped")
		}
	}
	for _, ph := range splitList(get("business_phone"), ",;|/") {
		if p, ok := normalize.Phone(ph); ok {
			n.Phones = append(n.Phones, p)
		} else {
			warn("business_phone", "a phone number without an unambiguous STD/country code was dropped (not guessed)")
		}
	}
	return res
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
