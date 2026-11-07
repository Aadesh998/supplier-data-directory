package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// GooglePlaces implements Provider using Places API (New). It is disabled unless explicitly
// enabled with an API key (GOOGLE_PLACES_ENABLED=true). It requests the minimum field mask
// needed for an admin to recognise a business, enforces a local daily request cap on top of
// Google Cloud quotas/billing alerts, and returns results without persisting them.
type GooglePlaces struct {
	APIKey   string
	DailyCap int
	BaseURL  string // overridable in tests
	Client   *http.Client

	mu    sync.Mutex
	day   string
	count int
}

// NewGooglePlaces builds the adapter.
func NewGooglePlaces(apiKey string, dailyCap int) *GooglePlaces {
	return &GooglePlaces{APIKey: apiKey, DailyCap: dailyCap, BaseURL: "https://places.googleapis.com/v1",
		Client: &http.Client{Timeout: 10 * time.Second}}
}

// Name implements Provider.
func (g *GooglePlaces) Name() string { return "google_places" }

// RetentionPolicy implements Provider.
func (g *GooglePlaces) RetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		StorableFields: []string{"provider_place_id"},
		ContentCache:   0,
		Attribution:    "Google Maps",
		Notice: "Results come from Google Maps and are shown for manual review only. They are not saved. " +
			"Only the place ID can be stored, as a lead for personal outreach. Do not copy names, addresses or other " +
			"details into the directory; add a business only from its own submission or another permitted source.",
	}
}

func (g *GooglePlaces) take() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	if g.day != today {
		g.day, g.count = today, 0
	}
	if g.DailyCap > 0 && g.count >= g.DailyCap {
		return ErrQuotaExceeded
	}
	g.count++
	return nil
}

const fieldMask = "places.id,places.displayName,places.formattedAddress,places.googleMapsUri"

type placeJSON struct {
	ID          string `json:"id"`
	DisplayName struct {
		Text string `json:"text"`
	} `json:"displayName"`
	FormattedAddress string `json:"formattedAddress"`
	GoogleMapsURI    string `json:"googleMapsUri"`
}

func (p placeJSON) result() Result {
	return Result{ProviderPlaceID: p.ID, DisplayName: p.DisplayName.Text, FormattedAddress: p.FormattedAddress, GoogleMapsURI: p.GoogleMapsURI}
}

func (g *GooglePlaces) do(req *http.Request, out any) error {
	req.Header.Set("X-Goog-Api-Key", g.APIKey)
	resp, err := g.Client.Do(req)
	if err != nil {
		return fmt.Errorf("places request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("places API returned %d", resp.StatusCode) // body omitted: may echo request details
	}
	return json.Unmarshal(body, out)
}

// SearchBusinesses implements Provider via Text Search (New).
func (g *GooglePlaces) SearchBusinesses(ctx context.Context, query, location string) ([]Result, error) {
	if g.APIKey == "" {
		return nil, ErrDisabled
	}
	if err := g.take(); err != nil {
		return nil, err
	}
	text := strings.TrimSpace(query)
	if l := strings.TrimSpace(location); l != "" {
		text += " in " + l
	}
	payload, _ := json.Marshal(map[string]any{"textQuery": text, "regionCode": "IN", "pageSize": 20})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/places:searchText", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-FieldMask", fieldMask)
	var resp struct {
		Places []placeJSON `json:"places"`
	}
	if err := g.do(req, &resp); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(resp.Places))
	for _, p := range resp.Places {
		out = append(out, p.result())
	}
	return out, nil
}

// GetPermittedMetadata refreshes display details for a stored place ID (transient).
func (g *GooglePlaces) GetPermittedMetadata(ctx context.Context, id string) (Result, error) {
	if g.APIKey == "" {
		return Result{}, ErrDisabled
	}
	if err := g.take(); err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.BaseURL+"/places/"+url.PathEscape(id), nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("X-Goog-FieldMask", strings.ReplaceAll(fieldMask, "places.", ""))
	var p placeJSON
	if err := g.do(req, &p); err != nil {
		return Result{}, err
	}
	return p.result(), nil
}
