package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/bharatmfg/directory/internal/importer"
)

// ImportJob mirrors import_jobs with its source.
type ImportJob struct {
	ID            string            `json:"id"`
	Filename      string            `json:"filename"`
	Status        string            `json:"status"`
	Headers       []string          `json:"headers"`
	ColumnMapping map[string]string `json:"column_mapping"`
	TotalRows     int               `json:"total_rows"`
	ValidRows     int               `json:"valid_rows"`
	ErrorRows     int               `json:"error_rows"`
	DuplicateRows int               `json:"duplicate_rows"`
	CommittedRows int               `json:"committed_rows"`
	Errors        []any             `json:"errors"`
	Source        DataSource        `json:"source"`
	// Background task state (see RunImportTask): task is non-null while work is running.
	Task           *string         `json:"task"`
	TaskStartedAt  *time.Time      `json:"task_started_at"`
	TaskFinishedAt *time.Time      `json:"task_finished_at"`
	TaskResult     json.RawMessage `json:"task_result"`
	TaskError      *string         `json:"task_error"`
	CreatedAt      time.Time       `json:"created_at"`
	CommittedAt    *time.Time      `json:"committed_at"`
}

// ImportRow mirrors import_rows.
type ImportRow struct {
	ID                  string            `json:"id"`
	RowNumber           int               `json:"row_number"`
	RawData             map[string]string `json:"raw_data"`
	NormalizedData      json.RawMessage   `json:"normalized_data"`
	TransformationLog   []string          `json:"transformation_log"`
	ValidationErrors    []importer.Issue  `json:"validation_errors"`
	ValidationWarnings  []importer.Issue  `json:"validation_warnings"`
	DuplicateCandidates []Candidate       `json:"duplicate_candidates"`
	Disposition         string            `json:"disposition"`
	MatchedCompanyID    *string           `json:"matched_company_id"`
	CreatedCompanyID    *string           `json:"created_company_id"`
	CommittedAt         *time.Time        `json:"committed_at"`
}

// GetImportJob loads a job and its declared source.
func GetImportJob(ctx context.Context, q DBTX, id string) (ImportJob, error) {
	var j ImportJob
	var sourceID string
	err := q.QueryRow(ctx, `SELECT id, filename, status, headers, column_mapping, total_rows, valid_rows, error_rows,
		duplicate_rows, committed_rows, errors, declared_source_id, created_at, committed_at,
		task, task_started_at, task_finished_at, task_result, task_error FROM import_jobs WHERE id = $1`, id).
		Scan(&j.ID, &j.Filename, &j.Status, &j.Headers, &j.ColumnMapping, &j.TotalRows, &j.ValidRows, &j.ErrorRows,
			&j.DuplicateRows, &j.CommittedRows, &j.Errors, &sourceID, &j.CreatedAt, &j.CommittedAt,
			&j.Task, &j.TaskStartedAt, &j.TaskFinishedAt, &j.TaskResult, &j.TaskError)
	if err != nil {
		return j, err
	}
	j.Source, err = GetSource(ctx, q, sourceID)
	return j, err
}

// ListImportJobs lists jobs newest first.
func ListImportJobs(ctx context.Context, q DBTX, limit, offset int) ([]ImportJob, int, error) {
	var total int
	if err := q.QueryRow(ctx, `SELECT count(*) FROM import_jobs`).Scan(&total); err != nil {
		return nil, 0, err
	}
	ids, err := collect(ctx, q, `SELECT id FROM import_jobs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, func(r pgx.Rows) (string, error) {
		var x string
		return x, r.Scan(&x)
	}, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ImportJob, 0, len(ids))
	for _, id := range ids {
		j, err := GetImportJob(ctx, q, id)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, j)
	}
	return out, total, nil
}

func loadLookups(ctx context.Context, q DBTX) (importer.Lookups, error) {
	lk := importer.Lookups{Roles: map[string]bool{}, Categories: map[string]string{}}
	roles, err := ListRoles(ctx, q)
	if err != nil {
		return lk, err
	}
	for _, r := range roles {
		lk.Roles[r.Code] = true
	}
	cats, err := ListCategories(ctx, q, false)
	if err != nil {
		return lk, err
	}
	for _, c := range cats {
		lk.Categories[strings.ToLower(c.Name)] = c.Slug
		lk.Categories[c.Slug] = c.Slug
	}
	return lk, nil
}

// CreateImportInput describes an upload.
type CreateImportInput struct {
	Filename  string
	Data      []byte
	SourceID  string       // existing source, or
	NewSource *SourceInput // a new source declaration
	Mapping   map[string]string
	// Defer leaves row processing to a background task (status "uploaded", task "process").
	// The caller then runs RunImportTask(jobID, "process", ...).
	Defer bool
}

// CreateImport parses and stages a CSV. Nothing reaches the companies table until commit.
func (s *Store) CreateImport(ctx context.Context, a Actor, in CreateImportInput) (ImportJob, error) {
	parsed, err := importer.Parse(in.Data)
	if err != nil {
		return ImportJob{}, ErrInvalid{Msg: err.Error(), Fields: map[string]string{"file": err.Error()}}
	}
	mapping := in.Mapping
	if len(mapping) == 0 {
		mapping = importer.AutoMap(parsed.Headers)
	}
	sum := sha256.Sum256(in.Data)
	hash := hex.EncodeToString(sum[:])
	var jobID string
	err = s.InTx(ctx, func(tx pgx.Tx) error {
		sourceID := in.SourceID
		if in.NewSource != nil {
			src, err := CreateSource(ctx, tx, a, *in.NewSource)
			if err != nil {
				return err
			}
			sourceID = src.ID
		}
		if sourceID == "" {
			return ErrInvalid{Msg: "a data-origin declaration is required", Fields: map[string]string{"source": "declare where this data came from and what reuse rights you hold"}}
		}
		if _, err := GetSource(ctx, tx, sourceID); err != nil {
			if IsNotFound(err) {
				return ErrInvalid{Msg: "unknown source", Fields: map[string]string{"source_id": "does not exist"}}
			}
			return err
		}
		headers, _ := json.Marshal(parsed.Headers)
		mjson, _ := json.Marshal(mapping)
		err := tx.QueryRow(ctx, `INSERT INTO import_jobs (filename, file_hash, file_size, declared_source_id, headers, column_mapping, initiated_by, total_rows)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, in.Filename, hash, len(in.Data), sourceID, headers, mjson, a.ID, len(parsed.Rows)).Scan(&jobID)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "import_jobs_active_file_key" {
			return ErrConflict{Msg: "this exact file has already been imported for this source; roll back or cancel the earlier import first"}
		}
		if err != nil {
			return err
		}
		batch := &pgx.Batch{}
		for i, row := range parsed.Rows {
			raw, _ := json.Marshal(row)
			batch.Queue(`INSERT INTO import_rows (import_job_id, row_number, raw_data) VALUES ($1, $2, $3)`, jobID, i+1, raw)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
		if err := Audit(ctx, tx, a, "import.upload", "import_job", &jobID, nil,
			map[string]any{"filename": in.Filename, "rows": len(parsed.Rows), "source_id": sourceID}); err != nil {
			return err
		}
		if err := importer.ValidateMapping(mapping, parsed.Headers); err != nil {
			_, err2 := tx.Exec(ctx, `UPDATE import_jobs SET status = 'uploaded', errors = $2 WHERE id = $1`, jobID, mustJSON([]string{err.Error()}))
			return err2
		}
		if in.Defer {
			_, err := tx.Exec(ctx, `UPDATE import_jobs SET status = 'uploaded', task = 'process', task_started_at = now() WHERE id = $1`, jobID)
			return err
		}
		return processRows(ctx, tx, jobID, mapping)
	})
	if err != nil {
		return ImportJob{}, err
	}
	return GetImportJob(ctx, s.Pool, jobID)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// processRows (re)normalises every row of a job, finds duplicate candidates and updates counters.
func processRows(ctx context.Context, tx pgx.Tx, jobID string, mapping map[string]string) error {
	lk, err := loadLookups(ctx, tx)
	if err != nil {
		return err
	}
	type rawRow struct {
		id  string
		raw map[string]string
	}
	rows, err := collect(ctx, tx, `SELECT id, raw_data FROM import_rows WHERE import_job_id = $1 ORDER BY row_number`, func(r pgx.Rows) (rawRow, error) {
		var x rawRow
		return x, r.Scan(&x.id, &x.raw)
	}, jobID)
	if err != nil {
		return err
	}
	type seenKey struct{ kind, val string }
	seen := map[seenKey]int{}
	valid, errorsN, dups := 0, 0, 0
	batch := &pgx.Batch{}
	for i, row := range rows {
		res := importer.NormalizeRow(row.raw, mapping, lk)
		n := res.Normalized
		var cands []Candidate
		if len(res.Errors) == 0 {
			hashes := []string{}
			cands, err = FindCandidates(ctx, tx, CandidateKey{NormalizedName: n.NormalizedName, Domain: n.WebsiteDomain, City: n.Facility.City, GSTIN: n.GSTIN, ContactHashes: hashes})
			if err != nil {
				return err
			}
			// Duplicates within the same file.
			for _, k := range []seenKey{{"name", n.NormalizedName + "|" + strings.ToLower(n.Facility.City)}, {"domain", n.WebsiteDomain}, {"GSTIN", n.GSTIN}} {
				if k.val == "" || k.val == "|" {
					continue
				}
				if prev, ok := seen[k]; ok {
					res.Warnings = append(res.Warnings, importer.Issue{Field: k.kind,
						Message: fmt.Sprintf("same %s as row %d in this file", k.kind, prev)})
				} else {
					seen[k] = i + 1
				}
			}
		}
		if len(res.Errors) > 0 {
			errorsN++
		} else {
			valid++
		}
		if len(cands) > 0 {
			dups++
		}
		if cands == nil {
			cands = []Candidate{}
		}
		batch.Queue(`UPDATE import_rows SET normalized_data = $2, transformation_log = $3, validation_errors = $4,
			validation_warnings = $5, duplicate_candidates = $6, disposition = 'pending', matched_company_id = NULL
			WHERE id = $1 AND committed_at IS NULL`, row.id, mustJSON(n), mustJSON(orEmpty(res.Log)), mustJSON(orEmptyIssues(res.Errors)),
			mustJSON(orEmptyIssues(res.Warnings)), mustJSON(cands))
	}
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE import_jobs SET status = 'staged', column_mapping = $2, valid_rows = $3, error_rows = $4,
		duplicate_rows = $5, errors = '[]' WHERE id = $1`, jobID, mustJSON(mapping), valid, errorsN, dups)
	return err
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func orEmptyIssues(s []importer.Issue) []importer.Issue {
	if s == nil {
		return []importer.Issue{}
	}
	return s
}

// RemapImport re-runs normalisation with a new mapping (uncommitted jobs only).
// With deferred=true the mapping is saved and a background "process" task is claimed; the caller
// then runs RunImportTask.
func (s *Store) RemapImport(ctx context.Context, a Actor, id string, mapping map[string]string, deferred ...bool) (ImportJob, error) {
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status string
		var headers []string
		var task *string
		if err := tx.QueryRow(ctx, `SELECT status, headers, task FROM import_jobs WHERE id = $1 FOR UPDATE`, id).Scan(&status, &headers, &task); err != nil {
			return err
		}
		if task != nil {
			return ErrConflict{Msg: "another operation is still running on this import; wait for it to finish"}
		}
		if status != "uploaded" && status != "staged" {
			return ErrConflict{Msg: "mapping can only be changed before commit (status is " + status + ")"}
		}
		if err := importer.ValidateMapping(mapping, headers); err != nil {
			return ErrInvalid{Msg: err.Error(), Fields: map[string]string{"column_mapping": err.Error()}}
		}
		if len(deferred) > 0 && deferred[0] {
			if _, err := tx.Exec(ctx, `UPDATE import_jobs SET column_mapping = $2, task = 'process', task_started_at = now(),
				task_finished_at = NULL, task_result = NULL, task_error = NULL WHERE id = $1`, id, mustJSON(mapping)); err != nil {
				return err
			}
		} else if err := processRows(ctx, tx, id, mapping); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "import.remap", "import_job", &id, nil, map[string]any{"column_mapping": mapping})
	})
	if err != nil {
		return ImportJob{}, err
	}
	return GetImportJob(ctx, s.Pool, id)
}

const importRowCols = `id, row_number, raw_data, normalized_data, transformation_log, validation_errors, validation_warnings,
	duplicate_candidates, disposition, matched_company_id, created_company_id, committed_at`

func scanImportRow(r pgx.Row) (ImportRow, error) {
	var x ImportRow
	err := r.Scan(&x.ID, &x.RowNumber, &x.RawData, &x.NormalizedData, &x.TransformationLog, &x.ValidationErrors,
		&x.ValidationWarnings, &x.DuplicateCandidates, &x.Disposition, &x.MatchedCompanyID, &x.CreatedCompanyID, &x.CommittedAt)
	return x, err
}

// ListImportRows lists rows with optional filters: disposition, and filter in errors|duplicates|clean.
func ListImportRows(ctx context.Context, q DBTX, jobID, disposition, filter string, limit, offset int) ([]ImportRow, int, error) {
	where := "import_job_id = $1 AND ($2 = '' OR disposition = $2)"
	switch filter {
	case "errors":
		where += " AND jsonb_array_length(validation_errors) > 0"
	case "duplicates":
		where += " AND jsonb_array_length(duplicate_candidates) > 0"
	case "clean":
		where += " AND jsonb_array_length(validation_errors) = 0 AND jsonb_array_length(duplicate_candidates) = 0"
	}
	var total int
	if err := q.QueryRow(ctx, "SELECT count(*) FROM import_rows WHERE "+where, jobID, disposition).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := collect(ctx, q, "SELECT "+importRowCols+" FROM import_rows WHERE "+where+" ORDER BY row_number LIMIT $3 OFFSET $4",
		func(r pgx.Rows) (ImportRow, error) { return scanImportRow(r) }, jobID, disposition, limit, offset)
	return rows, total, err
}

// UpdateImportRow sets a row's disposition (and merge target).
func (s *Store) UpdateImportRow(ctx context.Context, a Actor, jobID, rowID, disposition string, matched *string) (ImportRow, error) {
	var row ImportRow
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM import_jobs WHERE id = $1`, jobID).Scan(&status); err != nil {
			return err
		}
		if status != "staged" && status != "committed" {
			return ErrConflict{Msg: "rows can only be reviewed on staged imports"}
		}
		if disposition == "merged" && (matched == nil || *matched == "") {
			return ErrInvalid{Msg: "merge target required", Fields: map[string]string{"matched_company_id": "is required to merge"}}
		}
		if disposition != "merged" {
			matched = nil
		}
		tag, err := tx.Exec(ctx, `UPDATE import_rows SET disposition = $3, matched_company_id = $4, reviewed_by = $5
			WHERE id = $2 AND import_job_id = $1 AND committed_at IS NULL`, jobID, rowID, disposition, matched, a.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrConflict{Msg: "row not found or already committed"}
		}
		row, err = scanImportRow(tx.QueryRow(ctx, "SELECT "+importRowCols+" FROM import_rows WHERE id = $1", rowID))
		return err
	})
	return row, err
}

// CommitResult summarises a commit or dry run.
type CommitResult struct {
	DryRun   bool          `json:"dry_run"`
	Created  int           `json:"created"`
	Merged   int           `json:"merged"`
	Skipped  int           `json:"skipped"`
	Rejected int           `json:"rejected"`
	Errors   []CommitError `json:"errors"`
}

// CommitError is a per-row commit failure.
type CommitError struct {
	RowNumber int    `json:"row_number"`
	Message   string `json:"message"`
}

var errDryRun = errors.New("dry run rollback")

// CommitImport writes accepted/merged rows (and optionally clean pending rows) to the directory.
// A dry run executes the same logic inside a transaction that is rolled back.
func (s *Store) CommitImport(ctx context.Context, a Actor, jobID string, dryRun, includeCleanPending bool) (CommitResult, error) {
	res := CommitResult{DryRun: dryRun, Errors: []CommitError{}}
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status, sourceID string
		var internal bool
		if err := tx.QueryRow(ctx, `SELECT j.status, j.declared_source_id, ds.allowed_internal_use FROM import_jobs j
			JOIN data_sources ds ON ds.id = j.declared_source_id WHERE j.id = $1 FOR UPDATE OF j`, jobID).Scan(&status, &sourceID, &internal); err != nil {
			return err
		}
		if status != "staged" && status != "committed" {
			return ErrConflict{Msg: "only staged imports can be committed (status is " + status + ")"}
		}
		if !internal {
			return ErrPolicy{Code: "source_not_permitted", Msg: "the declared source does not permit internal use; rows must stay in staging until reuse rights are documented"}
		}
		type row struct {
			id, disposition string
			number          int
			norm            importer.Normalized
			errCount        int
			candCount       int
			matched         *string
		}
		rows, err := collect(ctx, tx, `SELECT id, row_number, disposition, normalized_data, jsonb_array_length(validation_errors),
			jsonb_array_length(duplicate_candidates), matched_company_id FROM import_rows
			WHERE import_job_id = $1 AND committed_at IS NULL ORDER BY row_number`, func(r pgx.Rows) (row, error) {
			var x row
			return x, r.Scan(&x.id, &x.number, &x.disposition, &x.norm, &x.errCount, &x.candCount, &x.matched)
		}, jobID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.disposition == "rejected" {
				res.Rejected++
				continue
			}
			doCreate := r.disposition == "accepted" || (r.disposition == "pending" && includeCleanPending && r.candCount == 0)
			doMerge := r.disposition == "merged"
			if r.errCount > 0 || (!doCreate && !doMerge) {
				res.Skipped++
				continue
			}
			sp, err := tx.Begin(ctx) // savepoint per row
			if err != nil {
				return err
			}
			var companyID string
			if doCreate {
				companyID, err = s.createFromImport(ctx, sp, a, sourceID, r.norm)
			} else {
				companyID = *r.matched
				err = s.mergeFromImport(ctx, sp, sourceID, companyID, r.norm)
			}
			if err == nil {
				_, err = sp.Exec(ctx, `UPDATE import_rows SET committed_at = now(), created_company_id = $2 WHERE id = $1`,
					r.id, map[bool]*string{true: &companyID, false: nil}[doCreate])
			}
			if err != nil {
				_ = sp.Rollback(ctx)
				res.Errors = append(res.Errors, CommitError{RowNumber: r.number, Message: friendlyError(err)})
				continue
			}
			if err := sp.Commit(ctx); err != nil {
				return err
			}
			if doCreate {
				res.Created++
			} else {
				res.Merged++
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE import_jobs SET status = 'committed', committed_at = COALESCE(committed_at, now()),
			finished_at = now(), committed_rows = (SELECT count(*) FROM import_rows WHERE import_job_id = $1 AND committed_at IS NOT NULL)
			WHERE id = $1`, jobID); err != nil {
			return err
		}
		if err := Audit(ctx, tx, a, "import.commit", "import_job", &jobID, nil, res); err != nil {
			return err
		}
		if dryRun {
			return errDryRun
		}
		return nil
	})
	if errors.Is(err, errDryRun) {
		return res, nil
	}
	return res, err
}

func friendlyError(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return "database rule " + pgErr.ConstraintName + ": " + pgErr.Message
	}
	var inv ErrInvalid
	if errors.As(err, &inv) {
		parts := []string{inv.Msg}
		for k, v := range inv.Fields {
			parts = append(parts, k+" "+v)
		}
		return strings.Join(parts, "; ")
	}
	return err.Error()
}

func (s *Store) categoryIDs(ctx context.Context, q DBTX, slugs []string) ([]string, error) {
	return collect(ctx, q, `SELECT id FROM categories WHERE slug = ANY($1)`, func(r pgx.Rows) (string, error) {
		var x string
		return x, r.Scan(&x)
	}, slugs)
}

func (s *Store) createFromImport(ctx context.Context, tx pgx.Tx, a Actor, sourceID string, n importer.Normalized) (string, error) {
	catIDs, err := s.categoryIDs(ctx, tx, n.CategorySlugs)
	if err != nil {
		return "", err
	}
	in := CompanyInput{SourceID: &sourceID, DisplayName: &n.DisplayName, Roles: &n.Roles, CategoryIDs: &catIDs}
	in.LegalName, in.Description, in.WebsiteURL = strOrNil(n.LegalName), strOrNil(n.Description), strOrNil(n.WebsiteURL)
	in.GSTIN = strOrNil(n.GSTIN)
	if n.EstablishmentYear != 0 {
		in.EstablishmentYear = &n.EstablishmentYear
	}
	if len(catIDs) > 0 {
		in.PrimaryCategoryID = &catIDs[0]
	}
	id, err := createCompanyTx(ctx, tx, a, in, "approved_private")
	if err != nil {
		return "", err
	}
	return id, s.addImportChildren(ctx, tx, sourceID, id, n, catIDs)
}

func (s *Store) addImportChildren(ctx context.Context, tx pgx.Tx, sourceID, companyID string, n importer.Normalized, catIDs []string) error {
	f := n.Facility
	if f.City != "" || f.StateCode != "" || f.AddressLine1 != "" {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM facilities WHERE company_id = $1 AND deleted_at IS NULL
			AND lower(coalesce(city,'')) = lower($2) AND coalesce(state_code,'') = $3)`, companyID, f.City, f.StateCode).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			fi := FacilityInput{SourceID: sourceID, FacilityType: f.FacilityType, AddressLine1: strOrNil(f.AddressLine1),
				Locality: strOrNil(f.Locality), City: strOrNil(f.City), District: strOrNil(f.District),
				StateCode: strOrNil(f.StateCode), PostalCode: strOrNil(f.PostalCode)}
			if _, err := insertFacility(ctx, tx, companyID, fi); err != nil {
				return err
			}
		}
	}
	var primaryCat *string
	if len(catIDs) > 0 {
		primaryCat = &catIDs[0]
	}
	for _, p := range n.Products {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM products WHERE company_id = $1 AND normalized_name = lower($2) AND deleted_at IS NULL)`,
			companyID, p).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			if _, err := insertProduct(ctx, tx, companyID, ProductInput{SourceID: sourceID, Name: p, CategoryID: primaryCat}); err != nil {
				return err
			}
		}
	}
	// Imported contacts are never public: the import declaration does not establish a basis for
	// publishing contact details. Role mailboxes are stored as business email (not public);
	// personal emails and all phone numbers are stored encrypted as individual contacts.
	addContact := func(contactType, channel, value string, encrypt bool) error {
		hash := s.Box.Hash(value)
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM contacts WHERE company_id = $1 AND value_hash = $2 AND deleted_at IS NULL)`,
			companyID, hash).Scan(&exists); err != nil || exists {
			return err
		}
		var plain *string
		var enc []byte
		if encrypt {
			var err error
			if enc, err = s.Box.Encrypt(value); err != nil {
				return err
			}
		} else {
			plain = &value
		}
		_, err := tx.Exec(ctx, `INSERT INTO contacts (company_id, contact_type, channel, value, value_encrypted, value_hash, source_id,
			lawful_basis_or_permission_ref) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, companyID, contactType, channel, plain, enc, hash, sourceID,
			"import:"+sourceID)
		return err
	}
	for _, e := range n.BusinessEmails {
		if err := addContact("public_business_email", "email", e, false); err != nil {
			return err
		}
	}
	for _, e := range n.PersonalEmails {
		if err := addContact("individual", "email", e, true); err != nil {
			return err
		}
	}
	for _, p := range n.Phones {
		if err := addContact("individual", "phone", p, true); err != nil {
			return err
		}
	}
	return nil
}

// mergeFromImport adds an imported row's data to an existing company without overwriting values.
func (s *Store) mergeFromImport(ctx context.Context, tx pgx.Tx, sourceID, companyID string, n importer.Normalized) error {
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM companies WHERE id = $1 FOR UPDATE`, companyID).Scan(&deleted); err != nil {
		return err
	}
	if deleted {
		return ErrConflict{Msg: "merge target has been deleted"}
	}
	fill := map[string]any{"legal_name": strOrNil(n.LegalName), "description": strOrNil(n.Description), "website_url": strOrNil(n.WebsiteURL),
		"gstin": strOrNil(n.GSTIN)}
	if n.EstablishmentYear != 0 {
		fill["establishment_year"] = n.EstablishmentYear
	}
	for col, v := range fill {
		if v == nil || v == (*string)(nil) {
			continue
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE companies SET %s = $2 WHERE id = $1 AND %s IS NULL`, col, col), companyID, v)
		if err != nil {
			return err
		}
		if tag.RowsAffected() > 0 {
			if col == "website_url" {
				if _, err := tx.Exec(ctx, `UPDATE companies SET website_domain = $2 WHERE id = $1`, companyID, n.WebsiteDomain); err != nil {
					return err
				}
			}
			if err := RecordProvenance(ctx, tx, companyID, "company", companyID, sourceID, col); err != nil {
				return err
			}
		}
	}
	// The import also corroborates the display name.
	if err := RecordProvenance(ctx, tx, companyID, "company", companyID, sourceID, "display_name"); err != nil {
		return err
	}
	if len(n.Roles) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO company_roles (company_id, role_code, source_id) SELECT $1, unnest($2::text[]), $3 ON CONFLICT DO NOTHING`,
			companyID, n.Roles, sourceID); err != nil {
			return err
		}
	}
	catIDs, err := s.categoryIDs(ctx, tx, n.CategorySlugs)
	if err != nil {
		return err
	}
	if len(catIDs) > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO company_categories (company_id, category_id, source_id) SELECT $1, unnest($2::uuid[]), $3 ON CONFLICT DO NOTHING`,
			companyID, catIDs, sourceID); err != nil {
			return err
		}
	}
	return s.addImportChildren(ctx, tx, sourceID, companyID, n, catIDs)
}

// RollbackImport undoes a commit: companies created by the import are archived and soft-deleted,
// and child records (facilities, products, contacts, roles, categories) that the import added to
// merged companies are removed. Company-level fields filled during a merge are not reverted; the
// audit log records them.
func (s *Store) RollbackImport(ctx context.Context, a Actor, jobID string) (ImportJob, error) {
	err := s.InTx(ctx, func(tx pgx.Tx) error {
		var status, sourceID string
		var committedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT status, declared_source_id, committed_at FROM import_jobs WHERE id = $1 FOR UPDATE`, jobID).
			Scan(&status, &sourceID, &committedAt); err != nil {
			return err
		}
		switch status {
		case "rolled_back", "cancelled", "failed":
			return ErrConflict{Msg: "import is already " + status}
		}
		if committedAt != nil {
			if _, err := tx.Exec(ctx, `UPDATE companies SET publication_status = 'archived', deleted_at = now()
				WHERE id IN (SELECT created_company_id FROM import_rows WHERE import_job_id = $1 AND created_company_id IS NOT NULL)
				AND deleted_at IS NULL`, jobID); err != nil {
				return err
			}
			merged := `(SELECT matched_company_id FROM import_rows WHERE import_job_id = $1 AND disposition = 'merged' AND committed_at IS NOT NULL)`
			for _, sql := range []string{
				`UPDATE facilities SET deleted_at = now(), is_primary = false WHERE company_id IN ` + merged + ` AND source_id = $2 AND created_at >= $3`,
				`UPDATE products SET deleted_at = now() WHERE company_id IN ` + merged + ` AND source_id = $2 AND created_at >= $3`,
				`UPDATE contacts SET deleted_at = now() WHERE company_id IN ` + merged + ` AND source_id = $2 AND created_at >= $3`,
				`DELETE FROM company_roles WHERE company_id IN ` + merged + ` AND source_id = $2 AND created_at >= $3`,
				`DELETE FROM company_categories WHERE company_id IN ` + merged + ` AND source_id = $2 AND created_at >= $3`,
			} {
				if _, err := tx.Exec(ctx, sql, jobID, sourceID, *committedAt); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE import_jobs SET status = $2, rolled_back_at = now(), finished_at = now() WHERE id = $1`,
			jobID, map[bool]string{true: "rolled_back", false: "cancelled"}[committedAt != nil]); err != nil {
			return err
		}
		return Audit(ctx, tx, a, "import.rollback", "import_job", &jobID, map[string]any{"status": status}, nil)
	})
	if err != nil {
		return ImportJob{}, err
	}
	return GetImportJob(ctx, s.Pool, jobID)
}

// ImportReportCSV renders a validation report for download. Raw personal data is not included.
func ImportReportCSV(ctx context.Context, q DBTX, jobID string) ([]byte, error) {
	rows, _, err := ListImportRows(ctx, q, jobID, "", "", importer.MaxRows+1, 0)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"row_number", "display_name", "disposition", "errors", "warnings", "duplicate_candidates", "created_company_id", "committed"})
	join := func(is []importer.Issue) string {
		parts := make([]string, len(is))
		for i, x := range is {
			parts[i] = x.Field + ": " + x.Message
		}
		return strings.Join(parts, " | ")
	}
	for _, r := range rows {
		var n importer.Normalized
		_ = json.Unmarshal(r.NormalizedData, &n)
		cands := make([]string, len(r.DuplicateCandidates))
		for i, c := range r.DuplicateCandidates {
			cands[i] = fmt.Sprintf("%s (%.2f: %s)", c.DisplayName, c.Score, strings.Join(c.Reasons, ", "))
		}
		_ = w.Write([]string{strconv.Itoa(r.RowNumber), safeCSV(n.DisplayName), r.Disposition, safeCSV(join(r.ValidationErrors)),
			safeCSV(join(r.ValidationWarnings)), safeCSV(strings.Join(cands, " | ")), deref(r.CreatedCompanyID),
			strconv.FormatBool(r.CommittedAt != nil)})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// safeCSV neutralises spreadsheet formula injection.
func safeCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}
