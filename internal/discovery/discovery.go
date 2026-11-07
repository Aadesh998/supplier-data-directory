// Package discovery defines the boundary for third-party business discovery providers.
//
// Policy (see docs/data-source-policy.md): results are shown transiently to an admin for
// manual outreach only. They are never written to the companies tables. The only value that may
// be persisted is the provider's place identifier, which Google's Places policies permit storing
// indefinitely ("You can therefore store place ID values indefinitely").
package discovery

import (
	"context"
	"errors"
	"time"
)

// Result is one transient discovery hit.
type Result struct {
	ProviderPlaceID  string `json:"provider_place_id"`
	DisplayName      string `json:"display_name"`
	FormattedAddress string `json:"formatted_address"`
	GoogleMapsURI    string `json:"google_maps_uri,omitempty"`
}

// RetentionPolicy describes what may be stored from this provider.
type RetentionPolicy struct {
	StorableFields []string      `json:"storable_fields"`
	ContentCache   time.Duration `json:"content_cache"` // 0 = do not store content at all
	Attribution    string        `json:"attribution"`
	Notice         string        `json:"notice"`
}

// Provider is a discovery source.
type Provider interface {
	Name() string
	SearchBusinesses(ctx context.Context, query, location string) ([]Result, error)
	GetPermittedMetadata(ctx context.Context, id string) (Result, error)
	RetentionPolicy() RetentionPolicy
}

// ErrDisabled is returned when discovery is not configured.
var ErrDisabled = errors.New("discovery provider is disabled")

// ErrQuotaExceeded is returned when the local daily cap is reached.
var ErrQuotaExceeded = errors.New("daily discovery quota reached")
