// Command api runs Peoplvance API and its maintenance commands.
//
//	api serve                      start the HTTP server
//	api migrate up|down|reset|status
//	api seed [--demo]              load synthetic records (--demo publishes them; refused in production)
//	api create-admin --email E --role owner|admin|reviewer   (password read from ADMIN_PASSWORD or stdin)
//	api import-csv --file F --as ADMIN_EMAIL --source-name NAME [--commit]   (stage, and optionally commit, a CSV import)
//	api set-password --email E     reset an admin password (ADMIN_PASSWORD or stdin); revokes sessions
//	api maintenance                re-verification, retention and permission-expiry jobs
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bharatmfg/directory/db"
	"github.com/bharatmfg/directory/internal/api"
	"github.com/bharatmfg/directory/internal/config"
	"github.com/bharatmfg/directory/internal/database"
	"github.com/bharatmfg/directory/internal/discovery"
	"github.com/bharatmfg/directory/internal/gmaps"
	"github.com/bharatmfg/directory/internal/notify"
	"github.com/bharatmfg/directory/internal/secure"
	"github.com/bharatmfg/directory/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)
	if err := run(log, os.Args[1:]); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger, args []string) error {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	if args[0] == "healthcheck" {
		return healthcheck()
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Connect(ctx, cfg.DatabaseURL, cfg.DatabaseSchema)
	if err != nil {
		return err
	}
	defer pool.Close()
	box, err := secure.NewBox(cfg.DataEncryptionKey)
	if err != nil {
		return err
	}
	st := store.New(pool, box)

	switch args[0] {
	case "serve":
		if cfg.MigrateOnStart {
			if err := database.MigrateUp(ctx, pool); err != nil {
				return fmt.Errorf("migrate: %w", err)
			}
		}
		return serve(ctx, log, cfg, st)
	case "migrate":
		sub := "up"
		if len(args) > 1 {
			sub = args[1]
		}
		switch sub {
		case "up":
			return database.MigrateUp(ctx, pool)
		case "down":
			return database.MigrateDown(ctx, pool)
		case "reset":
			if cfg.IsProduction() {
				return errors.New("migrate reset is refused in production")
			}
			if cfg.DatabaseSchema != "" {
				return errors.New("migrate reset is refused when DATABASE_SCHEMA is set (shared database); drop the schema manually if you really mean it")
			}
			if err := database.MigrateDownAll(ctx, pool); err != nil {
				return err
			}
			return database.MigrateUp(ctx, pool)
		case "status":
			lines, err := database.MigrationStatus(ctx, pool)
			for _, l := range lines {
				fmt.Println(l)
			}
			return err
		}
		return fmt.Errorf("unknown migrate subcommand %q", sub)
	case "seed":
		fsFlags := flag.NewFlagSet("seed", flag.ContinueOnError)
		demo := fsFlags.Bool("demo", false, "publish synthetic records for local previews")
		if err := fsFlags.Parse(args[1:]); err != nil {
			return err
		}
		if *demo && cfg.IsProduction() {
			return errors.New("demo seed is refused in production")
		}
		return seed(ctx, st, *demo)
	case "create-admin":
		fsFlags := flag.NewFlagSet("create-admin", flag.ContinueOnError)
		email := fsFlags.String("email", "", "admin email")
		role := fsFlags.String("role", "owner", "owner|admin|reviewer")
		name := fsFlags.String("name", "", "display name")
		if err := fsFlags.Parse(args[1:]); err != nil {
			return err
		}
		password := os.Getenv("ADMIN_PASSWORD")
		if password == "" {
			fmt.Fprint(os.Stderr, "Password (min 12 chars): ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			password = strings.TrimSpace(line)
		}
		u, err := store.CreateAdminUser(ctx, pool, store.SystemActor(), *email, *name, password, *role)
		if err != nil {
			return err
		}
		fmt.Printf("created %s admin %s (%s)\n", u.Role, u.Email, u.ID)
		return nil
	case "set-password":
		fsFlags := flag.NewFlagSet("set-password", flag.ContinueOnError)
		email := fsFlags.String("email", "", "admin email")
		if err := fsFlags.Parse(args[1:]); err != nil {
			return err
		}
		password := os.Getenv("ADMIN_PASSWORD")
		if password == "" {
			fmt.Fprint(os.Stderr, "New password (min 12 chars): ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			password = strings.TrimSpace(line)
		}
		if err := store.SetPassword(ctx, pool, store.SystemActor(), *email, password); err != nil {
			if store.IsNotFound(err) {
				return fmt.Errorf("no admin with email %q", *email)
			}
			return err
		}
		fmt.Printf("password updated for %s; existing sessions revoked\n", *email)
		return nil
	case "import-csv":
		return importCSV(ctx, st, args[1:])
	case "gmaps-worker":
		// Long-running: runs gmaps_scraper discovery jobs one at a time, pauses when none are due.
		return gmaps.NewWorker(gmaps.ConfigFromEnv(), st, log).Run(ctx)
	case "maintenance":
		rep, err := st.RunMaintenance(ctx)
		if err != nil {
			return err
		}
		log.Info("maintenance complete", "marked_stale", rep.MarkedStale, "unpublished", len(rep.Unpublished),
			"sessions_purged", rep.SessionsPurged, "submissions_purged", rep.SubmissionsPurged)
		return nil
	}
	return fmt.Errorf("unknown command %q", args[0])
}

// healthcheck probes the local /readyz endpoint (used by the container HEALTHCHECK).
func healthcheck() error {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" || strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
		if addr == "127.0.0.1" {
			addr += ":8080"
		}
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/readyz")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz returned %d", resp.StatusCode)
	}
	return nil
}

func seed(ctx context.Context, st *store.Store, demo bool) error {
	files, err := fs.Glob(db.Seeds, "seeds/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		if strings.HasSuffix(f, "002_demo_publish.sql") && !demo {
			continue
		}
		b, err := fs.ReadFile(db.Seeds, f)
		if err != nil {
			return err
		}
		if _, err := st.Pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		slog.Info("applied seed", "file", f)
	}
	return nil
}

func serve(ctx context.Context, log *slog.Logger, cfg config.Config, st *store.Store) error {
	var mailer notify.Mailer = notify.LogMailer{Log: log}
	if cfg.SMTPHost != "" {
		mailer = notify.SMTPMailer{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.MailFrom}
	}
	var disc discovery.Provider
	if cfg.GooglePlacesEnabled {
		if cfg.GooglePlacesAPIKey == "" {
			return errors.New("GOOGLE_PLACES_ENABLED=true requires GOOGLE_PLACES_API_KEY")
		}
		disc = discovery.NewGooglePlaces(cfg.GooglePlacesAPIKey, cfg.GooglePlacesDailyCap)
		log.Warn("Google Places discovery adapter ENABLED: results are transient; only place IDs may be stored")
	}
	// Background import tasks that were running when the process last stopped were rolled back by
	// PostgreSQL; mark them interrupted so they can be started again. (Assumes one API instance.)
	if n, err := store.ClearInterruptedImportTasks(ctx, st.Pool); err != nil {
		return err
	} else if n > 0 {
		log.Warn("marked interrupted import tasks", "count", n)
	}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(cfg, st, log, mailer, disc).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	// Daily maintenance in-process for single-instance deployments.
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := st.RunMaintenance(ctx); err != nil {
					log.Error("maintenance failed", "err", err)
				}
			}
		}
	}()
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.HTTPAddr, "env", cfg.Env)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// importCSV stages a CSV through the same importer as Admin → Imports (normalisation, encryption of
// personal contacts, duplicate detection, provenance) and optionally commits the clean rows. Rows
// with errors or duplicate candidates stay pending for review in the admin console.
func importCSV(ctx context.Context, st *store.Store, args []string) error {
	fsFlags := flag.NewFlagSet("import-csv", flag.ContinueOnError)
	file := fsFlags.String("file", "", "CSV file to import (UTF-8)")
	as := fsFlags.String("as", "", "email of the owner/admin the import is attributed to")
	sourceName := fsFlags.String("source-name", "", "data source (provider) name; created internal-only if it does not exist")
	notes := fsFlags.String("source-notes", "", "notes for a newly created source (where the data came from, basis for use)")
	commit := fsFlags.Bool("commit", false, "commit clean rows (no errors, no duplicate candidates)")
	if err := fsFlags.Parse(args); err != nil {
		return err
	}
	if *file == "" || *as == "" || *sourceName == "" {
		return errors.New("--file, --as and --source-name are required")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	var actor store.Actor
	err = st.Pool.QueryRow(ctx, `SELECT id, email::text, role FROM admin_users WHERE email = lower($1) AND active AND role IN ('owner','admin')`, *as).
		Scan(&actor.ID, &actor.Email, &actor.Role)
	if err != nil {
		return fmt.Errorf("no active owner/admin with email %q", *as)
	}
	actor.Type = "admin"

	in := store.CreateImportInput{Filename: filepath.Base(*file), Data: data}
	var sourceID string
	err = st.Pool.QueryRow(ctx, `SELECT id FROM data_sources WHERE provider = $1 ORDER BY created_at LIMIT 1`, *sourceName).Scan(&sourceID)
	switch {
	case err == nil:
		in.SourceID = sourceID
		fmt.Printf("using existing source %q (%s)\n", *sourceName, sourceID)
	case store.IsNotFound(err):
		yes, no := true, false
		src := store.SourceInput{SourceType: "authorized_import", Provider: *sourceName,
			AllowedInternalUse: &yes, AllowedPublicDisplay: &no, AllowedCommercialAPI: &no}
		if *notes != "" {
			src.Notes = notes
		}
		in.NewSource = &src
		fmt.Printf("creating internal-only source %q (public display and commercial use OFF)\n", *sourceName)
	default:
		return err
	}
	job, err := st.CreateImport(ctx, actor, in)
	if err != nil {
		return err
	}
	fmt.Printf("staged import %s: %d rows, %d valid, %d with errors, %d with duplicate candidates\n",
		job.ID, job.TotalRows, job.ValidRows, job.ErrorRows, job.DuplicateRows)
	if !*commit {
		fmt.Println("not committed: review it in Admin → Imports, or re-run with --commit")
		return nil
	}
	res, err := st.CommitImport(ctx, actor, job.ID, false, true)
	if err != nil {
		return err
	}
	fmt.Printf("committed: %d created, %d merged, %d skipped, %d rejected, %d errors\n",
		res.Created, res.Merged, res.Skipped, res.Rejected, len(res.Errors))
	for i, e := range res.Errors {
		if i == 10 {
			fmt.Printf("  … %d more (see the import report)\n", len(res.Errors)-10)
			break
		}
		fmt.Printf("  %+v\n", e)
	}
	fmt.Println("rows with duplicate candidates or errors are still pending: review them in Admin → Imports")
	return nil
}
