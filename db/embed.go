// Package db embeds SQL migrations and seed files so the API binary is self-contained.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS

//go:embed seeds/*.sql
var Seeds embed.FS
