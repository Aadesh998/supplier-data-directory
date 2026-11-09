package normalize

import (
	"regexp"
	"strings"
)

// States maps ISO 3166-2:IN codes to names (matches the indian_states table).
var States = map[string]string{
	"AP": "Andhra Pradesh", "AR": "Arunachal Pradesh", "AS": "Assam", "BR": "Bihar",
	"CG": "Chhattisgarh", "GA": "Goa", "GJ": "Gujarat", "HR": "Haryana",
	"HP": "Himachal Pradesh", "JH": "Jharkhand", "KA": "Karnataka", "KL": "Kerala",
	"MP": "Madhya Pradesh", "MH": "Maharashtra", "MN": "Manipur", "ML": "Meghalaya",
	"MZ": "Mizoram", "NL": "Nagaland", "OD": "Odisha", "PB": "Punjab", "RJ": "Rajasthan",
	"SK": "Sikkim", "TN": "Tamil Nadu", "TS": "Telangana", "TR": "Tripura",
	"UP": "Uttar Pradesh", "UK": "Uttarakhand", "WB": "West Bengal",
	"AN": "Andaman and Nicobar Islands", "CH": "Chandigarh",
	"DH": "Dadra and Nagar Haveli and Daman and Diu", "DL": "Delhi",
	"JK": "Jammu and Kashmir", "LA": "Ladakh", "LD": "Lakshadweep", "PY": "Puducherry",
}

// Historical names, common abbreviations and pre-2023 ISO codes.
var stateAliases = map[string]string{
	"orissa": "OD", "or": "OD", "pondicherry": "PY", "pondy": "PY", "ut": "UK", "uttaranchal": "UK",
	"ct": "CG", "chattisgarh": "CG", "chhatisgarh": "CG", "tg": "TS", "telengana": "TS",
	"new delhi": "DL", "nct of delhi": "DL", "national capital territory of delhi": "DL",
	"j&k": "JK", "j and k": "JK", "jammu & kashmir": "JK", "jammu kashmir": "JK",
	"andaman": "AN", "andaman & nicobar": "AN", "andaman and nicobar": "AN",
	"daman and diu": "DH", "daman & diu": "DH", "dadra and nagar haveli": "DH", "dnh": "DH", "dd": "DH",
	"tamilnadu": "TN", "t.n.": "TN", "u.p.": "UP", "m.p.": "MP", "a.p.": "AP", "w.b.": "WB",
	"maharastra": "MH", "maharashtra state": "MH", "gujrat": "GJ", "karnatka": "KA",
	"rajastan": "RJ", "panjab": "PB", "bengal": "WB", "west bengal state": "WB",
}

var stateByName = func() map[string]string {
	m := make(map[string]string, len(States)*2)
	for code, name := range States {
		m[strings.ToLower(name)] = code
		m[strings.ToLower(code)] = code
	}
	for alias, code := range stateAliases {
		m[alias] = code
	}
	return m
}()

// StateCode resolves a state name, alias or code to its ISO code. ok=false if unknown.
func StateCode(s string) (string, bool) {
	key := strings.ToLower(Whitespace(s))
	key = strings.TrimPrefix(key, "in-")
	if code, ok := stateByName[key]; ok {
		return code, true
	}
	if code, ok := stateByName[strings.ReplaceAll(key, ".", "")]; ok {
		return code, true
	}
	return "", false
}

// gstStateCodes maps the 2-digit GST state code (first two characters of a GSTIN) to ISO 3166-2:IN.
var gstStateCodes = map[string]string{
	"01": "JK", "02": "HP", "03": "PB", "04": "CH", "05": "UK", "06": "HR", "07": "DL", "08": "RJ", "09": "UP",
	"10": "BR", "11": "SK", "12": "AR", "13": "NL", "14": "MN", "15": "MZ", "16": "TR", "17": "ML", "18": "AS",
	"19": "WB", "20": "JH", "21": "OD", "22": "CG", "23": "MP", "24": "GJ", "25": "DH", "26": "DH", "27": "MH",
	"28": "AP", "29": "KA", "30": "GA", "31": "LD", "32": "KL", "33": "TN", "34": "PY", "35": "AN", "36": "TS",
	"37": "AP", "38": "LA",
}

var gstinPattern = regexp.MustCompile(`^[0-9]{2}[A-Z]{5}[0-9]{4}[A-Z][1-9A-Z]Z[0-9A-Z]$`)

// GSTIN upper-cases and strips spaces/hyphens, then checks the 15-character GSTIN structure and a
// known state prefix. It does not verify the check digit or that the registration is active.
func GSTIN(s string) (string, bool) {
	g := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", ".", "").Replace(strings.TrimSpace(s)))
	if !gstinPattern.MatchString(g) {
		return "", false
	}
	if _, ok := gstStateCodes[g[:2]]; !ok {
		return "", false
	}
	return g, true
}

// StateFromGSTIN returns the ISO state code of a GSTIN's registration state.
func StateFromGSTIN(gstin string) (string, bool) {
	if len(gstin) < 2 {
		return "", false
	}
	code, ok := gstStateCodes[gstin[:2]]
	return code, ok
}
