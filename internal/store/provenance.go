package store

import (
	"context"
	"time"
)

// RecordProvenance writes one provenance row per field. permission_status is "permitted"
// when the source allows at least internal use, otherwise "unknown".
func RecordProvenance(ctx context.Context, q DBTX, companyID, entityType, entityID, sourceID string, fields ...string) error {
	if sourceID == "" || len(fields) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, `INSERT INTO field_provenance (company_id, entity_type, entity_id, field_name, source_id, permission_status)
		SELECT $1, $2, $3, f, ds.id, CASE WHEN ds.allowed_internal_use THEN 'permitted' ELSE 'unknown' END
		FROM unnest($5::text[]) AS f, data_sources ds WHERE ds.id = $4
		ON CONFLICT (entity_type, entity_id, field_name, source_id)
		DO UPDATE SET observed_at = now()`, companyID, entityType, entityID, sourceID, fields)
	return err
}

// Provenance is a field provenance row with its source.
type Provenance struct {
	EntityType       string     `json:"entity_type"`
	EntityID         string     `json:"entity_id"`
	FieldName        string     `json:"field_name"`
	PermissionStatus string     `json:"permission_status"`
	ObservedAt       time.Time  `json:"observed_at"`
	Source           DataSource `json:"source"`
}

// ListProvenance returns provenance for a company.
func ListProvenance(ctx context.Context, q DBTX, companyID string) ([]Provenance, error) {
	rows, err := q.Query(ctx, `SELECT fp.entity_type, fp.entity_id, fp.field_name, fp.permission_status, fp.observed_at,
		ds.id, ds.source_type, ds.provider, ds.source_url, ds.license_reference, ds.collected_at, ds.allowed_internal_use,
		ds.allowed_public_display, ds.allowed_commercial_api, ds.retention_expires_at, ds.restrictions, ds.notes, ds.created_at
		FROM field_provenance fp JOIN data_sources ds ON ds.id = fp.source_id
		WHERE fp.company_id = $1 ORDER BY fp.entity_type, fp.field_name, fp.observed_at DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Provenance{}
	for rows.Next() {
		var p Provenance
		d := &p.Source
		if err := rows.Scan(&p.EntityType, &p.EntityID, &p.FieldName, &p.PermissionStatus, &p.ObservedAt,
			&d.ID, &d.SourceType, &d.Provider, &d.SourceURL, &d.LicenseReference, &d.CollectedAt, &d.AllowedInternalUse,
			&d.AllowedPublicDisplay, &d.AllowedCommercialAPI, &d.RetentionExpiresAt, &d.Restrictions, &d.Notes, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// permissionSet answers "may field X of entity Y be used for purpose Z".
type permissionSet map[string]bool

func permKey(companyID, entityType, entityID, field string) string {
	return companyID + "|" + entityType + "|" + entityID + "|" + field
}

func (p permissionSet) has(companyID, entityType, entityID, field string) bool {
	return p[permKey(companyID, entityType, entityID, field)]
}

// Purpose selects which permission view to consult.
type Purpose string

const (
	PurposePublic     Purpose = "public"
	PurposeCommercial Purpose = "commercial"
	PurposeInternal   Purpose = "internal"
)

func (p Purpose) view() string {
	switch p {
	case PurposeCommercial:
		return "commercial_field_permissions"
	case PurposeInternal:
		return "internal_field_permissions"
	default:
		return "public_field_permissions"
	}
}

func (p Purpose) sourceFlag() string {
	switch p {
	case PurposeCommercial:
		return "allowed_commercial_api"
	case PurposeInternal:
		return "allowed_internal_use"
	default:
		return "allowed_public_display"
	}
}

func loadPermissions(ctx context.Context, q DBTX, purpose Purpose, companyIDs []string) (permissionSet, error) {
	set := permissionSet{}
	if len(companyIDs) == 0 {
		return set, nil
	}
	rows, err := q.Query(ctx, `SELECT company_id, entity_type, entity_id, field_name FROM `+purpose.view()+
		` WHERE company_id = ANY($1::uuid[])`, companyIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c, et, eid, f string
		if err := rows.Scan(&c, &et, &eid, &f); err != nil {
			return nil, err
		}
		set[permKey(c, et, eid, f)] = true
	}
	return set, rows.Err()
}
