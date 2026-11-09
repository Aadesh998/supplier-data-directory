package normalize

import "testing"

func TestCompanyName(t *testing.T) {
	cases := map[string]string{
		"Kalin Weaves Pvt. Ltd.":            "kalin weaves",
		"KALIN WEAVES PRIVATE LIMITED":      "kalin weaves",
		"  Bikaneri  Bhujia & Sons ":        "bikaneri bhujia",
		"Morvi Vitrified Tiles LLP":         "morvi vitrified tiles",
		"Precision Turn Industries Pvt Ltd": "precision turn",
		"Industries":                        "industries",
		"Ashblock Industries (Synthetic)":   "ashblock",
	}
	for in, want := range cases {
		if got := CompanyName(in); got != want {
			t.Errorf("CompanyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlug(t *testing.T) {
	if got := Slug("Bikaneri Bhujia & Sons (Synthetic)"); got != "bikaneri-bhujia-and-sons-synthetic" {
		t.Errorf("got %q", got)
	}
}

func TestWebsiteURL(t *testing.T) {
	good := map[string][2]string{
		"www.Example.com/":             {"https://www.example.com", "example.com"},
		"http://kalin.co.in/about?x=1": {"http://kalin.co.in/about", "kalin.co.in"},
	}
	for in, want := range good {
		u, d, ok := WebsiteURL(in)
		if !ok || u != want[0] || d != want[1] {
			t.Errorf("WebsiteURL(%q) = %q %q %v", in, u, d, ok)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "http://127.0.0.1", "http://localhost/x", "ftp://a.com",
		"http://metadata.internal", "http://user:pw@a.com", "http://10.0.0.1:8080", "intranet", "http://a.com:8443"} {
		if _, _, ok := WebsiteURL(bad); ok {
			t.Errorf("WebsiteURL(%q) should be rejected", bad)
		}
	}
}

func TestPhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"98765 43210", "+919876543210", true},
		{"+91-98765-43210", "+919876543210", true},
		{"022 2345 6789", "+912223456789", true},
		{"919876543210", "+919876543210", true},
		{"+44 20 7946 0000", "+442079460000", true},
		{"2345678", "2345678", false},        // landline without STD code: do not guess
		{"12345 67890", "1234567890", false}, // not a valid Indian mobile prefix
	}
	for _, c := range cases {
		got, ok := Phone(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Phone(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStateCode(t *testing.T) {
	cases := map[string]string{"Maharashtra": "MH", "orissa": "OD", "IN-TN": "TN", "Pondicherry": "PY", "u.p.": "UP", "J&K": "JK", "tg": "TS"}
	for in, want := range cases {
		if got, ok := StateCode(in); !ok || got != want {
			t.Errorf("StateCode(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if _, ok := StateCode("Atlantis"); ok {
		t.Error("unknown state accepted")
	}
}

func TestEmailAndPin(t *testing.T) {
	if e, ok := Email(" Sales@Kalin.Example "); !ok || e != "sales@kalin.example" {
		t.Errorf("Email = %q %v", e, ok)
	}
	if _, ok := Email("not-an-email"); ok {
		t.Error("bad email accepted")
	}
	if _, ok := PostalCode("011001"); ok {
		t.Error("PIN starting with 0 accepted")
	}
	if p, ok := PostalCode("411 001"); !ok || p != "411001" {
		t.Errorf("PostalCode = %q %v", p, ok)
	}
}

func TestGSTIN(t *testing.T) {
	cases := map[string]string{
		"06ABTFA5899D1ZW":    "06ABTFA5899D1ZW",
		" 06adzps1327c1ze ":  "06ADZPS1327C1ZE",
		"06 ABTFA 5899 D1ZW": "06ABTFA5899D1ZW",
		"066AACCK3866B1ZS":   "", // 16 chars
		"06AN0PA1521FZK":     "", // malformed
		"99ABTFA5899D1ZW":    "", // unknown state
		"":                   "",
	}
	for in, want := range cases {
		got, ok := GSTIN(in)
		if got != want || ok != (want != "") {
			t.Errorf("GSTIN(%q) = %q,%v want %q", in, got, ok, want)
		}
	}
	if s, ok := StateFromGSTIN("09AAAAA0000A1Z5"); !ok || s != "UP" {
		t.Errorf("StateFromGSTIN = %q", s)
	}
}
