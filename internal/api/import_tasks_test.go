package api_test

import (
	"encoding/json"
	"testing"
	"time"
)

type asyncJob struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	ValidRows     int             `json:"valid_rows"`
	CommittedRows int             `json:"committed_rows"`
	Task          *string         `json:"task"`
	TaskResult    json.RawMessage `json:"task_result"`
	TaskError     *string         `json:"task_error"`
}

func waitTask(t *testing.T, c *client, id string) asyncJob {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		var j asyncJob
		c.must("GET", "/api/v1/admin/imports/"+id, nil, 200, &j)
		if j.Task == nil {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %s still running", *j.Task)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Large imports run as background tasks so HTTP/proxy timeouts cannot cut them off.
func TestImportBackgroundTasks(t *testing.T) {
	e := newEnv(t)
	admin := e.login("admin")
	var job asyncJob
	admin.must("POST", "/api/v1/admin/imports", csvUpload("bulk.csv", importCSV, map[string]string{"async": "1",
		"source": `{"source_type":"authorized_import","provider":"bulk sheet","allowed_internal_use":true}`}), 202, &job)
	if job.Task == nil || *job.Task != "process" || job.Status != "uploaded" {
		t.Fatalf("expected a running process task, got %+v", job)
	}
	job = waitTask(t, admin, job.ID)
	if job.Status != "staged" || job.ValidRows != 2 || job.TaskError != nil {
		t.Fatalf("after processing: %+v", job)
	}

	// Dry run in the background: result recorded, nothing committed.
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"dry_run": true, "include_clean_pending": true, "async": true}, 202, nil)
	job = waitTask(t, admin, job.ID)
	var res struct {
		DryRun  bool `json:"dry_run"`
		Created int  `json:"created"`
	}
	if err := json.Unmarshal(job.TaskResult, &res); err != nil || !res.DryRun || res.Created == 0 || job.Status != "staged" {
		t.Fatalf("dry run: job=%+v res=%+v err=%v", job, res, err)
	}

	// Real commit.
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"dry_run": false, "include_clean_pending": true, "async": true}, 202, nil)
	job = waitTask(t, admin, job.ID)
	if job.Status != "committed" || job.CommittedRows == 0 || job.TaskError != nil {
		t.Fatalf("commit: %+v", job)
	}

	// A failing task records a readable error instead of hanging (rolled-back import cannot be committed).
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/rollback", nil, 200, nil)
	admin.must("POST", "/api/v1/admin/imports/"+job.ID+"/commit", map[string]any{"dry_run": false, "async": true}, 202, nil)
	job = waitTask(t, admin, job.ID)
	if job.TaskError == nil || job.Status != "rolled_back" {
		t.Fatalf("expected task error, got %+v", job)
	}
}
