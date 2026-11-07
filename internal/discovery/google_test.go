package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGooglePlacesSearchUsesFieldMaskAndKeyHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/places:searchText" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Goog-Api-Key") != "k" || r.Header.Get("X-Goog-FieldMask") != fieldMask {
			t.Errorf("missing headers: %v", r.Header)
		}
		if r.URL.Query().Get("key") != "" {
			t.Error("API key must not be in the URL")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if (body["textQuery"] != "tile manufacturer in Morbi" && body["textQuery"] != "a") || body["regionCode"] != "IN" {
			t.Errorf("body = %v", body)
		}
		_, _ = w.Write([]byte(`{"places":[{"id":"abc","displayName":{"text":"X Tiles"},"formattedAddress":"Morbi","googleMapsUri":"https://maps.google.com/?cid=1"}]}`))
	}))
	defer srv.Close()
	g := NewGooglePlaces("k", 2)
	g.BaseURL = srv.URL
	res, err := g.SearchBusinesses(context.Background(), "tile manufacturer", "Morbi")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ProviderPlaceID != "abc" || res[0].DisplayName != "X Tiles" {
		t.Fatalf("res = %+v", res)
	}
	_, _ = g.SearchBusinesses(context.Background(), "a", "")
	if _, err := g.SearchBusinesses(context.Background(), "a", ""); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected daily cap to apply, got %v", err)
	}
}

func TestGooglePlacesDisabledWithoutKey(t *testing.T) {
	g := NewGooglePlaces("", 10)
	if _, err := g.SearchBusinesses(context.Background(), "x", ""); !errors.Is(err, ErrDisabled) {
		t.Fatalf("expected ErrDisabled, got %v", err)
	}
	if p := g.RetentionPolicy(); len(p.StorableFields) != 1 || p.StorableFields[0] != "provider_place_id" || p.ContentCache != 0 {
		t.Fatalf("retention policy must allow storing only place IDs: %+v", p)
	}
}
