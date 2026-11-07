// Package openapi embeds the API contract so non-production builds can serve it.
package openapi

import _ "embed"

// Spec is the OpenAPI 3.1 document.
//
//go:embed openapi.yaml
var Spec []byte
