package store

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

// ImportTaskTimeout bounds one background import task.
const ImportTaskTimeout = 45 * time.Minute

// ClaimImportTask marks a dry run or commit as running. Only one task may run per import; a task
// older than ImportTaskTimeout is considered dead and can be replaced.
func ClaimImportTask(ctx context.Context, q DBTX, jobID, task string) error {
	var status string
	err := q.QueryRow(ctx, `UPDATE import_jobs SET task = $2, task_started_at = now(), task_finished_at = NULL,
		task_result = NULL, task_error = NULL
		WHERE id = $1 AND (task IS NULL OR task_started_at < now() - make_interval(mins => $3)) RETURNING status`,
		jobID, task, int(ImportTaskTimeout.Minutes())).Scan(&status)
	if IsNotFound(err) {
		if _, gerr := GetImportJob(ctx, q, jobID); gerr != nil {
			return gerr
		}
		return ErrConflict{Msg: "another operation is still running on this import; wait for it to finish"}
	}
	return err
}

// RunImportTask performs a claimed task to completion, detached from any HTTP request, and records
// the result (or a readable error) on the job. task is "process", "dry_run" or "commit".
func (s *Store) RunImportTask(a Actor, jobID, task string, includeCleanPending bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ImportTaskTimeout)
	defer cancel()
	var result any
	var err error
	switch task {
	case "process":
		err = s.InTx(ctx, func(tx pgx.Tx) error {
			var mapping map[string]string
			if err := tx.QueryRow(ctx, `SELECT column_mapping FROM import_jobs WHERE id = $1`, jobID).Scan(&mapping); err != nil {
				return err
			}
			if err := processRows(ctx, tx, jobID, mapping); err != nil {
				return err
			}
			j, err := GetImportJob(ctx, tx, jobID)
			result = map[string]any{"valid_rows": j.ValidRows, "error_rows": j.ErrorRows, "duplicate_rows": j.DuplicateRows}
			return err
		})
	case "dry_run", "commit":
		result, err = s.CommitImport(ctx, a, jobID, task == "dry_run", includeCleanPending)
	default:
		err = errors.New("unknown import task " + task)
	}
	var msg *string
	if err != nil {
		slog.Error("import task failed", "job", jobID, "task", task, "err", err)
		m := taskErrorMessage(err)
		msg = &m
		result = nil
	}
	// Use a fresh context: the task context may have expired.
	fctx, fcancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer fcancel()
	_, _ = s.Pool.Exec(fctx, `UPDATE import_jobs SET task = NULL, task_finished_at = now(), task_result = $2, task_error = $3
		WHERE id = $1`, jobID, mustJSONOrNil(result), msg)
}

// ClearInterruptedImportTasks is called at startup: tasks that were running when the process
// stopped were rolled back by PostgreSQL, so mark them as interrupted.
func ClearInterruptedImportTasks(ctx context.Context, q DBTX) (int64, error) {
	tag, err := q.Exec(ctx, `UPDATE import_jobs SET task = NULL, task_finished_at = now(),
		task_error = 'interrupted by a server restart; nothing was saved, run it again' WHERE task IS NOT NULL`)
	return tag.RowsAffected(), err
}

func taskErrorMessage(err error) string {
	var conf ErrConflict
	var inv ErrInvalid
	var pol ErrPolicy
	switch {
	case errors.As(err, &conf):
		return conf.Msg
	case errors.As(err, &inv):
		return inv.Msg
	case errors.As(err, &pol):
		return pol.Msg
	case errors.Is(err, context.DeadlineExceeded):
		return "the operation took longer than 45 minutes and was cancelled; nothing was saved"
	default:
		return "the operation failed and nothing was saved; see the server log"
	}
}

func mustJSONOrNil(v any) []byte {
	if v == nil {
		return nil
	}
	return mustJSON(v)
}
