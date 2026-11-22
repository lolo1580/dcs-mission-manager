package app

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"dcsmanager/internal/aerodrome"
	"dcsmanager/internal/api"
	"dcsmanager/internal/auth"
	"dcsmanager/internal/category"
	"dcsmanager/internal/charts"
	"dcsmanager/internal/config"
	"dcsmanager/internal/db"
	"dcsmanager/internal/live"
	"dcsmanager/internal/state"
	"dcsmanager/internal/stats"
)

// RunServer starts the multi-user web application. It never reads DCS, never
// opens the telemetry ports and never installs anything: its only inputs are the
// database and the uploads sent by desktops. The desktop binary remains fully
// usable without it.
func RunServer() error {
	cfg := config.Load()
	cfg.Mode = "server"
	// A container is reached from another machine by definition. Keep an
	// explicit DCSMANAGER_HTTP_ADDR authoritative, but do not fall back to the
	// desktop loopback default.
	if os.Getenv("DCSMANAGER_HTTP_ADDR") == "" {
		cfg.HTTPAddr = "0.0.0.0:8080"
	}

	log.Printf("dcsmanager server %s", Version)

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database %s: %w", cfg.DBPath, err)
	}
	defer database.Close()

	if err := bootstrapAdmin(database, cfg); err != nil {
		return err
	}

	store := state.New(cfg.UnitTTL, cfg.MaxUnits)
	liveStore := live.New(1000, 500)
	classifier := category.New(cfg.CategoriesFile)
	statsService := stats.New(database, classifier)

	airfields, err := aerodrome.Load()
	if err != nil {
		log.Printf("aerodrome: embedded dataset unavailable: %v", err)
	} else {
		log.Printf("aerodrome: %d airfields from the embedded dataset", airfields.Count())
	}
	chartCatalog, err := charts.Load(cfg.ChartsDir)
	if err != nil {
		log.Printf("charts: %v", err)
		chartCatalog, _ = charts.Load("")
	}

	server := api.New(cfg, store, liveStore, database, statsService, airfields, chartCatalog, nil)
	server.SetVersion(Version)
	server.SetMode("server")
	server.SetAuth(database)
	server.SetLocalOnly(false)

	// Expired sessions are harmless but accumulate; prune them periodically.
	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := database.PruneSessions(time.Now().UnixMilli()); err != nil {
				log.Printf("sessions: prune: %v", err)
			}
			if err := database.PrunePasswordResets(time.Now().UnixMilli()); err != nil {
				log.Printf("password resets: prune: %v", err)
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Printf("server: web app on http://%s", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// bootstrapAdmin creates the first administrator on an empty database. There is
// no default password: an operator must provide both variables, so a forgotten
// deployment can never expose a well-known account.
func bootstrapAdmin(database *db.DB, cfg config.Config) error {
	count, err := database.CountUsers()
	if err != nil {
		return fmt.Errorf("count accounts: %w", err)
	}
	if count > 0 {
		return nil
	}
	if cfg.AdminUser == "" || cfg.AdminPassword == "" {
		return errors.New("no account exists: set DCSMANAGER_ADMIN_USER and DCSMANAGER_ADMIN_PASSWORD for the first start")
	}
	if !auth.ValidUsername(cfg.AdminUser) {
		return errors.New("DCSMANAGER_ADMIN_USER must be 3-32 characters: letters, digits, dot, dash or underscore")
	}
	hash, err := auth.HashPassword(cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("DCSMANAGER_ADMIN_PASSWORD: %w", err)
	}
	admin, err := database.CreateUser(auth.User{
		Username:     cfg.AdminUser,
		DisplayName:  cfg.AdminUser,
		Role:         auth.RoleAdmin,
		Active:       true,
		DCSNames:     []string{},
		PasswordHash: hash,
	})
	if err != nil {
		return fmt.Errorf("create administrator: %w", err)
	}
	log.Printf("server: administrator %q created (#%d)", admin.Username, admin.ID)
	return nil
}
