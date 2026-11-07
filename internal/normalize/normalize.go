// Package normalize cleans business data for storage and duplicate detection.
// Functions never guess missing information (e.g. they do not invent country codes).
package normalize

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	spaceRe    = regexp.MustCompile(`\s+`)
	nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)
	// Legal-form suffixes and filler words that should not affect duplicate matching.
	legalSuffixes = []string{
		"private limited", "pvt limited", "pvt ltd", "pvt. ltd", "p ltd", "public limited",
		"limited", "ltd", "llp", "opc", "incorporated", "inc", "corporation", "corp",
		"company", "co", "and sons", "& sons", "enterprises", "industries", "india",
	}
	suffixRe = func() *regexp.Regexp {
		parts := make([]string, len(legalSuffixes))
		for i, s := range legalSuffixes {
			parts[i] = regexp.QuoteMeta(s)
		}
		return regexp.MustCompile(`\b(` + strings.Join(parts, "|") + `)\b\.?`)
	}()
)

// Whitespace trims and collapses internal whitespace. Control characters are removed.
func Whitespace(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func stripAccents(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

// CompanyName returns a matching key: lowercase, accents removed, punctuation and
// legal-form suffixes stripped. "Kalin Weaves Pvt. Ltd." -> "kalin weaves".
func CompanyName(s string) string {
	s = strings.ToLower(stripAccents(Whitespace(s)))
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.ReplaceAll(s, "(synthetic)", " ")
	s = nonAlnumRe.ReplaceAllString(s, " ")
	for i := 0; i < 3; i++ { // suffixes can stack: "x industries pvt ltd"
		trimmed := strings.TrimSpace(suffixRe.ReplaceAllString(s, " "))
		trimmed = spaceRe.ReplaceAllString(trimmed, " ")
		if trimmed == "" {
			break // never strip the whole name away
		}
		s = trimmed
	}
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

// Slug converts text to a URL slug (max 80 chars).
func Slug(s string) string {
	s = strings.ToLower(stripAccents(Whitespace(s)))
	s = strings.ReplaceAll(s, "&", " and ")
	s = strings.Trim(nonAlnumRe.ReplaceAllString(s, "-"), "-")
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	return s
}

// WebsiteURL validates and canonicalises a website. Only http/https URLs with a public-looking
// hostname are accepted. A missing scheme is assumed to be https. Returns ("", "", false) if invalid.
func WebsiteURL(raw string) (canonical, domain string, ok bool) {
	raw = Whitespace(raw)
	if raw == "" {
		return "", "", false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return "", "", false
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if !IsPublicHostname(host) {
		return "", "", false
	}
	if u.Port() != "" {
		return "", "", false // custom ports are almost always mistakes or internal hosts
	}
	u.Host = host
	u.Fragment = ""
	u.RawQuery = ""
	if u.Path == "/" {
		u.Path = ""
	}
	return u.String(), Domain(host), true
}

// IsPublicHostname rejects IP literals, localhost, single-label and internal names.
// This is used to keep stored URLs from pointing at internal infrastructure (SSRF hygiene);
// the API never fetches stored URLs server-side.
func IsPublicHostname(host string) bool {
	if host == "" || len(host) > 253 || net.ParseIP(host) != nil {
		return false
	}
	if !strings.Contains(host, ".") {
		return false
	}
	for _, bad := range []string{"localhost", ".local", ".internal", ".lan", ".home", ".corp", ".localdomain"} {
		if host == strings.TrimPrefix(bad, ".") || strings.HasSuffix(host, bad) {
			return false
		}
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// Domain strips a leading "www." from a hostname.
func Domain(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

// Email lowercases and validates an email address shape. Returns ("", false) if invalid.
func Email(s string) (string, bool) {
	s = strings.ToLower(Whitespace(s))
	at := strings.LastIndex(s, "@")
	if at < 1 || at == len(s)-1 || strings.ContainsAny(s, " ,;<>\"") || len(s) > 254 {
		return "", false
	}
	domain := s[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return "", false
	}
	return s, true
}

// Phone normalises Indian phone numbers. It only adds +91 when the number is unambiguously
// Indian (10-digit mobile starting 6-9, or 0-prefixed STD number). Numbers it cannot classify
// are returned as cleaned digits with ok=false so they can be flagged for review instead of guessed.
func Phone(raw string) (normalized string, ok bool) {
	raw = Whitespace(raw)
	if raw == "" {
		return "", false
	}
	plus := strings.HasPrefix(raw, "+")
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case plus && strings.HasPrefix(d, "91") && len(d) == 12:
		return "+" + d, true
	case plus && len(d) >= 8 && len(d) <= 15:
		return "+" + d, true // international number, already has a country code
	case !plus && strings.HasPrefix(d, "0091") && len(d) == 14:
		return "+" + d[2:], true
	case !plus && strings.HasPrefix(d, "91") && len(d) == 12 && d[2] >= '6':
		return "+" + d, true
	case !plus && len(d) == 10 && d[0] >= '6' && d[0] <= '9':
		return "+91" + d, true
	case !plus && len(d) == 11 && d[0] == '0':
		return "+91" + d[1:], true
	default:
		return d, false
	}
}

// PostalCode validates an Indian PIN code (6 digits, not starting with 0).
func PostalCode(s string) (string, bool) {
	s = strings.ReplaceAll(Whitespace(s), " ", "")
	if len(s) != 6 || s[0] < '1' || s[0] > '9' {
		return s, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return s, false
		}
	}
	return s, true
}

// Title tidies a free-text place name: collapse whitespace and title-case all-caps/all-lower input.
func Title(s string) string {
	s = Whitespace(s)
	if s == "" {
		return s
	}
	if s != strings.ToUpper(s) && s != strings.ToLower(s) {
		return s // already mixed case; respect it
	}
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
