// Package gmaps runs discovery jobs through the Google Maps scraper service
// (see google-maps-scraper API.md) and saves the leads it finds.
package gmaps

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bharatmfg/directory/internal/store"
)

// Client talks to the scraper's /api/v1/jobs endpoints.
type Client struct {
	BaseURL string // e.g. https://leads.peoplvance.org/api/v1/jobs
	Auth    string // optional "user:password" for basic auth
	HTTP    *http.Client
}

// JobRequest is the body of POST /api/v1/jobs.
type JobRequest struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
	Lang     string   `json:"lang"`
	Depth    int      `json:"depth"`
	MaxTime  int      `json:"max_time"` // seconds
	Email    bool     `json:"email"`
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if u, p, ok := strings.Cut(c.Auth, ":"); ok {
		req.SetBasicAuth(u, p)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("scraper %s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(data[:min(len(data), 200)])))
	}
	return data, nil
}

// Create starts a scraper job and returns its id.
func (c *Client) Create(ctx context.Context, in JobRequest) (string, error) {
	data, err := c.do(ctx, http.MethodPost, "", in)
	if err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("scraper create: unexpected response %q", string(data[:min(len(data), 200)]))
	}
	return out.ID, nil
}

// Status returns pending, working, ok or failed.
func (c *Client) Status(ctx context.Context, id string) (string, error) {
	data, err := c.do(ctx, http.MethodGet, "/"+id, nil)
	if err != nil {
		return "", err
	}
	var out struct {
		Status string `json:"Status"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("scraper status: %w", err)
	}
	return out.Status, nil
}

// Download returns the results CSV of a finished job.
func (c *Client) Download(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/"+id+"/download", nil)
}

// Delete removes the job and its CSV from the scraper server.
func (c *Client) Delete(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, "/"+id, nil)
	return err
}

// ParseCSV turns the scraper's results CSV into leads, skipping rows without a name or id
// and duplicates within the file.
func ParseCSV(data []byte) ([]store.GmapsLead, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("results CSV header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.TrimPrefix(h, "\xef\xbb\xbf"))] = i
	}
	if _, ok := idx["title"]; !ok {
		return nil, errors.New("results CSV has no title column")
	}
	var leads []store.GmapsLead
	seen := map[string]bool{}
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("results CSV: %w", err)
		}
		get := func(col string) string {
			if i, ok := idx[col]; ok && i < len(rec) {
				return strings.TrimSpace(rec[i])
			}
			return ""
		}
		key := firstNonEmpty(get("place_id"), get("cid"), get("link"))
		title := get("title")
		if title == "" || key == "" || len(key) > 512 || seen[key] {
			continue
		}
		seen[key] = true
		l := store.GmapsLead{PlaceKey: key, Title: title, Category: get("category"), Address: get("address"),
			Phone: get("phone"), Website: get("website"), Emails: get("emails"), BusinessStatus: get("status"),
			MapsLink: get("link"), Latitude: parseFloat(get("latitude"), 90), Longitude: parseFloat(get("longitude"), 180),
			ReviewRating: parseFloat(get("review_rating"), 5)}
		if n, err := strconv.Atoi(get("review_count")); err == nil && n >= 0 {
			l.ReviewCount = &n
		}
		if ca := get("complete_address"); ca != "" && json.Valid([]byte(ca)) {
			l.CompleteAddress = json.RawMessage(ca)
		}
		leads = append(leads, l)
	}
	return leads, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseFloat returns nil for empty, invalid or out-of-range (|v| > limit) values.
func parseFloat(s string, limit float64) *float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < -limit || f > limit {
		return nil
	}
	return &f
}
