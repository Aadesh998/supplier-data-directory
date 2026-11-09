package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bharatmfg/directory/internal/httpx"
	"github.com/bharatmfg/directory/internal/importer"
	"github.com/bharatmfg/directory/internal/store"
)

func (s *Server) importFields(w http.ResponseWriter, r *http.Request) error {
	httpx.JSON(w, 200, map[string]any{"data": importer.Fields})
	return nil
}

func (s *Server) listImports(w http.ResponseWriter, r *http.Request) error {
	p := httpx.ParsePagination(r, 25)
	data, total, err := store.ListImportJobs(r.Context(), s.Store.Pool, p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(data, p, total))
	return nil
}

func (s *Server) createImport(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, s.Cfg.MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return httpx.E(http.StatusRequestEntityTooLarge, "payload_too_large", "file is larger than the upload limit")
		}
		return httpx.E(http.StatusBadRequest, "invalid_upload", "expected multipart/form-data with a file field")
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck
	file, hdr, err := r.FormFile("file")
	if err != nil {
		return httpx.FieldErrors(map[string]string{"file": "is required"})
	}
	defer file.Close()
	name := filepath.Base(hdr.Filename)
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".csv" && ext != ".txt" {
		return httpx.FieldErrors(map[string]string{"file": "must be a .csv file (export spreadsheets as CSV UTF-8)"})
	}
	data, err := io.ReadAll(io.LimitReader(file, s.Cfg.MaxUploadBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > s.Cfg.MaxUploadBytes {
		return httpx.E(http.StatusRequestEntityTooLarge, "payload_too_large", "file is larger than the upload limit")
	}
	if ct := http.DetectContentType(data); !strings.HasPrefix(ct, "text/") && ct != "application/octet-stream" {
		return httpx.FieldErrors(map[string]string{"file": "content does not look like a text CSV file (" + ct + ")"})
	}
	if mt, _, _ := mime.ParseMediaType(hdr.Header.Get("Content-Type")); strings.Contains(mt, "zip") || strings.Contains(mt, "excel") && ext != ".csv" {
		return httpx.FieldErrors(map[string]string{"file": "XLSX is not supported yet; export as CSV UTF-8"})
	}
	in := store.CreateImportInput{Filename: name, Data: data, SourceID: r.FormValue("source_id")}
	if in.SourceID != "" && !validUUID(in.SourceID) {
		return httpx.FieldErrors(map[string]string{"source_id": "must be a UUID"})
	}
	if raw := r.FormValue("source"); raw != "" {
		var src store.SourceInput
		if err := json.Unmarshal([]byte(raw), &src); err != nil {
			return httpx.FieldErrors(map[string]string{"source": "must be a JSON data-source declaration"})
		}
		if err := httpx.Validate(&src); err != nil {
			return err
		}
		if err := s.checkSourcePermissionRole(r, src); err != nil {
			return err
		}
		in.NewSource = &src
	}
	if in.SourceID == "" && in.NewSource == nil {
		return httpx.FieldErrors(map[string]string{"source": "declare the data origin and reuse rights before importing"})
	}
	if raw := r.FormValue("column_mapping"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &in.Mapping); err != nil {
			return httpx.FieldErrors(map[string]string{"column_mapping": "must be a JSON object"})
		}
	}
	in.Defer = isTrue(r.FormValue("async"))
	job, err := s.Store.CreateImport(r.Context(), s.actor(r), in)
	if err != nil {
		return translate(err)
	}
	if job.Task != nil {
		// Large files: rows are normalised and checked for duplicates in the background.
		go s.Store.RunImportTask(s.actor(r), job.ID, "process", false)
		httpx.JSON(w, http.StatusAccepted, job)
		return nil
	}
	httpx.JSON(w, http.StatusCreated, job)
	return nil
}

func (s *Server) getImport(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	j, err := store.GetImportJob(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, j)
	return nil
}

func (s *Server) listImportRows(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	p := httpx.ParsePagination(r, 50)
	q := r.URL.Query()
	rows, total, err := store.ListImportRows(r.Context(), s.Store.Pool, id, q.Get("disposition"), q.Get("filter"), p.Limit, p.Offset())
	if err != nil {
		return err
	}
	httpx.JSON(w, 200, httpx.NewPaged(rows, p, total))
	return nil
}

type rowUpdate struct {
	Disposition      string  `json:"disposition" validate:"required,oneof=pending accepted rejected merged"`
	MatchedCompanyID *string `json:"matched_company_id" validate:"omitempty,uuid"`
}

func (s *Server) updateImportRow(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	rowID, err := pathID(r, "rowId")
	if err != nil {
		return err
	}
	var in rowUpdate
	if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
		return err
	}
	row, err := s.Store.UpdateImportRow(r.Context(), s.actor(r), id, rowID, in.Disposition, in.MatchedCompanyID)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, row)
	return nil
}

type mappingBody struct {
	ColumnMapping map[string]string `json:"column_mapping" validate:"required"`
	Async         bool              `json:"async"`
}

func isTrue(v string) bool { return v == "1" || strings.EqualFold(v, "true") }

func (s *Server) remapImport(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in mappingBody
	if err := httpx.Decode(w, r, &in, 32<<10); err != nil {
		return err
	}
	j, err := s.Store.RemapImport(r.Context(), s.actor(r), id, in.ColumnMapping, in.Async)
	if err != nil {
		return translate(err)
	}
	if in.Async {
		go s.Store.RunImportTask(s.actor(r), id, "process", false)
		httpx.JSON(w, http.StatusAccepted, j)
		return nil
	}
	httpx.JSON(w, 200, j)
	return nil
}

type commitBody struct {
	DryRun              *bool `json:"dry_run"`
	IncludeCleanPending bool  `json:"include_clean_pending"`
	// Async runs the dry run / commit in the background; poll GET /imports/{id} for task_result.
	Async bool `json:"async"`
}

func (s *Server) commitImport(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	var in commitBody
	if r.ContentLength != 0 {
		if err := httpx.Decode(w, r, &in, 8<<10); err != nil {
			return err
		}
	}
	dry := in.DryRun == nil || *in.DryRun // dry run unless explicitly false
	if in.Async {
		task := map[bool]string{true: "dry_run", false: "commit"}[dry]
		if err := store.ClaimImportTask(r.Context(), s.Store.Pool, id, task); err != nil {
			return translate(err)
		}
		go s.Store.RunImportTask(s.actor(r), id, task, in.IncludeCleanPending)
		j, err := store.GetImportJob(r.Context(), s.Store.Pool, id)
		if err != nil {
			return err
		}
		httpx.JSON(w, http.StatusAccepted, j)
		return nil
	}
	res, err := s.Store.CommitImport(r.Context(), s.actor(r), id, dry, in.IncludeCleanPending)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, res)
	return nil
}

func (s *Server) rollbackImport(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	j, err := s.Store.RollbackImport(r.Context(), s.actor(r), id)
	if err != nil {
		return translate(err)
	}
	httpx.JSON(w, 200, j)
	return nil
}

func (s *Server) importReport(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r, "id")
	if err != nil {
		return err
	}
	if _, err := store.GetImportJob(r.Context(), s.Store.Pool, id); err != nil {
		return err
	}
	b, err := store.ImportReportCSV(r.Context(), s.Store.Pool, id)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="import-`+id+`-report.csv"`)
	_, _ = w.Write(b)
	return nil
}
