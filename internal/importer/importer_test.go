package importer

import (
	"strings"
	"testing"
)

var lk = Lookups{
	Roles:      map[string]bool{"manufacturer": true, "exporter": true, "wholesaler": true, "sourcing_agent": true},
	Categories: map[string]string{"namkeen and savoury snacks": "namkeen-and-snacks", "namkeen-and-snacks": "namkeen-and-snacks", "tiles": "tiles", "rugs-and-carpets": "rugs-and-carpets"},
}

func TestParseDelimitersBOMAndBlankRows(t *testing.T) {
	data := "\xef\xbb\xbfCompany Name;City;State\nKalin Weaves;Bhadohi;UP\n;;\nMorvi Tiles;Morbi;Gujarat\n"
	p, err := Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Headers) != 3 || p.Headers[0] != "Company Name" {
		t.Fatalf("headers = %v", p.Headers)
	}
	if len(p.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (blank row skipped)", len(p.Rows))
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	for name, data := range map[string]string{
		"empty":      "",
		"headerOnly": "a,b\n",
		"latin1":     "name\ncaf\xe9\n",
	} {
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseRaggedRowsAndDuplicateHeaders(t *testing.T) {
	p, err := Parse([]byte("name,phone,phone\nA,1\nB,2,3,extra\n"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Headers[2] != "phone_2" {
		t.Fatalf("headers = %v", p.Headers)
	}
	if p.Rows[1]["extra_column_4"] != "extra" {
		t.Fatalf("extra column not retained: %v", p.Rows[1])
	}
}

func TestAutoMap(t *testing.T) {
	m := AutoMap([]string{"Firm Name", "Pin Code", "Mobile", "E-mail", "Web Site", "Nature of Business", "State"})
	want := map[string]string{"display_name": "Firm Name", "postal_code": "Pin Code", "business_phone": "Mobile",
		"business_email": "E-mail", "website_url": "Web Site", "roles": "Nature of Business", "state": "State"}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("AutoMap[%s] = %q want %q", k, m[k], v)
		}
	}
	if err := ValidateMapping(map[string]string{"city": "City"}, []string{"City"}); err == nil {
		t.Error("mapping without display_name accepted")
	}
	if err := ValidateMapping(map[string]string{"display_name": "Nope"}, []string{"Name"}); err == nil {
		t.Error("mapping to missing column accepted")
	}
}

func TestNormalizeRow(t *testing.T) {
	mapping := map[string]string{"display_name": "name", "website_url": "web", "roles": "type", "categories": "cat",
		"state": "state", "postal_code": "pin", "business_phone": "phone", "business_email": "email", "products": "products",
		"establishment_year": "year", "city": "city"}
	raw := map[string]string{"name": "  Bikaneri   Bhujia Pvt Ltd ", "web": "www.bhujia.example", "type": "Manufacturing; Exporters; Wizard",
		"cat": "Namkeen and Savoury Snacks | Unknown Cat", "state": "Rajasthan", "pin": "334001", "phone": "98290 12345, 2522",
		"email": "sales@bhujia.example; ramesh@bhujia.example", "products": "Bhujia; Moong Dal; bhujia", "year": "19x5", "city": "BIKANER"}
	r := NormalizeRow(raw, mapping, lk)
	n := r.Normalized
	if len(r.Errors) != 0 {
		t.Fatalf("errors: %v", r.Errors)
	}
	checks := map[string][2]string{
		"display_name":    {n.DisplayName, "Bikaneri Bhujia Pvt Ltd"},
		"normalized_name": {n.NormalizedName, "bikaneri bhujia"},
		"website":         {n.WebsiteURL, "https://www.bhujia.example"},
		"domain":          {n.WebsiteDomain, "bhujia.example"},
		"state":           {n.Facility.StateCode, "RJ"},
		"city":            {n.Facility.City, "Bikaner"},
		"roles":           {strings.Join(n.Roles, ","), "manufacturer,exporter"},
		"categories":      {strings.Join(n.CategorySlugs, ","), "namkeen-and-snacks"},
		"products":        {strings.Join(n.Products, ","), "Bhujia,Moong Dal"},
		"phones":          {strings.Join(n.Phones, ","), "+919829012345"},
		"business_emails": {strings.Join(n.BusinessEmails, ","), "sales@bhujia.example"},
		"personal_emails": {strings.Join(n.PersonalEmails, ","), "ramesh@bhujia.example"},
	}
	for name, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
	if n.EstablishmentYear != 0 {
		t.Error("invalid year should be dropped")
	}
	fields := map[string]bool{}
	for _, w := range r.Warnings {
		fields[w.Field] = true
	}
	for _, f := range []string{"roles", "categories", "business_phone", "establishment_year"} {
		if !fields[f] {
			t.Errorf("expected warning for %s, got %v", f, r.Warnings)
		}
	}
}

func TestNormalizeRowMissingName(t *testing.T) {
	r := NormalizeRow(map[string]string{"name": "  "}, map[string]string{"display_name": "name"}, lk)
	if len(r.Errors) != 1 || r.Errors[0].Field != "display_name" {
		t.Fatalf("errors = %v", r.Errors)
	}
}
