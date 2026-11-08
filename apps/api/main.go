// Command api runs the Bharat Manufacturing Directory API and its maintenance commands.
//
//	api serve                      start the HTTP server
//	api migrate up|down|reset|status
//	api seed [--demo]              load synthetic records (--demo publishes them; refused in production)
//	api create-admin --email E --role owner|admin|reviewer   (password read from ADMIN_PASSWORD or stdin)
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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bharatmfg/directory/db"
	"github.com/bharatmfg/directory/internal/api"
	"github.com/bharatmfg/directory/internal/config"
	"github.com/bharatmfg/directory/internal/database"
	"github.com/bharatmfg/directory/internal/discovery"
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
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
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
