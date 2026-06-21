package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // embed timezone database so Docker containers don't need system tzdata

	"github.com/conorfennell/knolhash/internal/grpc"
	"github.com/conorfennell/knolhash/internal/storage"
	"github.com/conorfennell/knolhash/internal/sync"
	"github.com/conorfennell/knolhash/internal/web"
	"github.com/conorfennell/knolhash/internal/worldcup"

	"github.com/go-playground/validator/v10"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag" // Using pflag for better flag parsing with koanf
)

var (
	commit    = "dev"
	buildDate = "unknown"
)

// Config holds the application's configuration.
type Config struct {
	DBPath              string        `koanf:"db_path" validate:"required"`
	Serve               bool          `koanf:"serve"`
	ListenAddr          string        `koanf:"listen_addr" validate:"required_if=Serve true"`
	GRPCListenAddr      string        `koanf:"grpc_listen_addr" validate:"required_if=Serve true"`
	SyncInterval        time.Duration `koanf:"sync_interval" validate:"required_if=Serve true,gt=0"`
	FootballDataAPIKey  string        `koanf:"football_data_api_key"`
}

var k = koanf.New(".") // Initialize koanf with a dot delimiter

func main() {
	// 1. Configure Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("KnolHash starting up", "commit", commit, "build_date", buildDate)

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
			strings.TrimPrefix(s, "KNOLHASH_")),
			"_", ".")
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
		runWebServer(db, cfg.ListenAddr, cfg.GRPCListenAddr, cfg.SyncInterval, cfg.FootballDataAPIKey)
		return
	}

	// Default action is to sync
	sync.RunSync(db)
}

// runWebServer starts the HTTP and gRPC servers and a background sync ticker.
func runWebServer(db *storage.DB, addr string, grpcAddr string, syncInterval time.Duration, footballDataAPIKey string) {
	startBackgroundSync(db, syncInterval)

	wc := worldcup.NewCache()
	wc.StartBackgroundRefresh()

	lc := worldcup.NewLiveCache(footballDataAPIKey)
	lc.Start()

	go func() {
		if err := grpc.StartServer(grpcAddr); err != nil {
			slog.Error("Failed to start gRPC server", "error", err)
			os.Exit(1)
		}
	}()

	server := web.NewServer(db, wc, lc, commit, buildDate)
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
