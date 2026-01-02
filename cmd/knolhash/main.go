package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/sync"
	"github.com/conorfennell/knolhash/internal/web"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag" // Using pflag for better flag parsing with koanf
)

// Config holds the application's configuration.
type Config struct {
	DBPath       string        `koanf:"db_path" validate:"required"`
	Serve        bool          `koanf:"serve"`
	ListenAddr   string        `koanf:"listen_addr" validate:"required_if=Serve true"`
	SyncInterval time.Duration `koanf:"sync_interval" validate:"required_if=Serve true,gt=0"`
}

var k = koanf.New(".") // Initialize koanf with a dot delimiter

func main() {
	// 1. Configure Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// 2. Set up pflag
	pflags := pflag.NewFlagSet("knolhash", pflag.ExitOnError)
	pflags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		pflags.PrintDefaults()
	}

	// Load from config.yaml (lowest precedence)
	// Check for a config file path flag first
	cfgFile, _ := pflags.GetString("config") // Assume a --config flag might exist for a path
	if cfgFile == "" {
		cfgFile = "config.yaml" // Default config file name
	}

	if err := k.Load(file.Provider(cfgFile), yaml.Parser()); err != nil {
		slog.Info("No config.yaml found or error reading it", "file", cfgFile, "error", err)
	}

	// Load from environment variables (higher precedence than file)
	// KNOLHASH_DB_PATH, KNOLHASH_LISTEN_ADDR, etc.
	k.Load(env.Provider("KNOLHASH_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "KNOLHASH_")), "_", ".")
	}), nil)

	// Load from command-line flags (highest precedence)
	k.Load(posflag.Provider(pflags, ".", k), nil)

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		slog.Error("Failed to unmarshal configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}

	// 3. Open DB
	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close() // 4. Dispatch based on flags (now using config values)
	if cfg.Serve {
		runWebServer(db, cfg.ListenAddr, cfg.SyncInterval)
		return
	}

	// Default action is to sync
	sync.RunSync(db)
}

// addNewSource adds a new source to the database, determining its type.
func addNewSource(db *storage.DB, path string) error {
	// This logic could be moved to a shared package if it gets more complex
	sourceType := "local"
	if strings.HasSuffix(path, ".git") || strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "https://") {
		sourceType = "git"
	}

	existing, err := db.FindSourceByPath(path)
	if err != nil {
		return fmt.Errorf("error checking for existing source: %w", err)
	}
	if existing != nil {
		slog.Info("Source with path already exists", "path", path)
		return nil
	}

	_, err = db.InsertSource(path, sourceType)
	if err != nil {
		return fmt.Errorf("could not insert new source: %w", err)
	}
	slog.Info("Successfully added new source", "path", path, "type", sourceType)
	return nil
}

// runWebServer starts the HTTP server and a background sync ticker.
func runWebServer(db *storage.DB, addr string, syncInterval time.Duration) {
	startBackgroundSync(db, syncInterval)

	server := web.NewServer(db)
	slog.Info("Starting web server", "addr", addr)
	if err := http.ListenAndServe(addr, server); err != nil {
		slog.Error("Failed to start web server", "error", err)
		os.Exit(1)
	}
}

// startBackgroundSync starts a goroutine that periodically calls sync.RunSync.
func startBackgroundSync(db *storage.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			slog.Info("Background sync triggered", "interval", interval)
			sync.RunSync(db)
		}
	}()
	slog.Info("Background sync started", "interval", interval)
}
