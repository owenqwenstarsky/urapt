// Package app wires the urapt-server dependencies and runs the HTTP server.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"

	"urapt/server/aptrepo"
	"urapt/server/auth"
	"urapt/server/cache"
	"urapt/server/restapi"
	"urapt/server/store"
	"urapt/shared/config"
	"urapt/shared/db"
	"urapt/shared/gpg"
	"urapt/shared/log"
)

// App holds the server configuration and runtime dependencies.
type App struct {
	args    []string
	version string
}

// New constructs an App from the given command-line args and version string.
func New(args []string, version string) *App {
	return &App{args: args, version: version}
}

// signerProvider implements restapi.SignerProvider backed by a *gpg.Key.
type signerProvider struct {
	key *gpg.Key
}

func (s *signerProvider) PublicKeyArmored() (string, error) {
	if s.key == nil {
		return "", fmt.Errorf("no key")
	}
	return s.key.ArmoredPublic()
}

func (s *signerProvider) Fingerprint() string {
	if s.key == nil {
		return ""
	}
	return s.key.Fingerprint
}

// Key returns the signing key for the APT endpoint (Phase 6).
func (s *signerProvider) Key() *gpg.Key { return s.key }

// Run loads config, opens the database, ensures a signing key, and serves
// HTTP until the context is cancelled.
func (a *App) Run(ctx context.Context) error {
	cfg, err := a.loadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		return err
	}
	logger := log.New(cfg.LogLevel)
	logger.Info("starting urapt-server", "version", a.version, "bind", cfg.Bind, "base_url", cfg.BaseURL)

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid config", "err", err)
		return err
	}

	if err := ensureDirs(cfg); err != nil {
		logger.Error("setup store dirs", "err", err)
		return err
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		logger.Error("open database", "err", err)
		return err
	}
	defer database.Close()

	st := store.New(database)
	authSvc := auth.NewService(st)

	signer, err := ensureSigningKey(ctx, st, cfg)
	if err != nil {
		logger.Error("ensure signing key", "err", err)
		return err
	}
	sp := &signerProvider{key: signer}
	logger.Info("signing key ready", "fingerprint", sp.Fingerprint())

	idxCache := cache.New()

	root := chi.NewRouter()
	root.Mount("/api/v1", restapi.New(st, authSvc, sp, &cfg, idxCache))
	root.Mount("/apt", aptrepo.New(st, authSvc, sp.Key(), idxCache))

	srv := &http.Server{
		Addr:    cfg.Bind,
		Handler: root,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Bind)
		if cfg.TLSEnabled {
			errCh <- srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			errCh <- srv.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
			return err
		}
	}
	return nil
}

// loadConfig parses flags and builds the effective Config.
func (a *App) loadConfig() (config.Config, error) {
	fs := flag.NewFlagSet("urapt-server", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.Defaults.ConfigPath, "path to config file")
	fs.String("bind", "", "bind address")
	fs.String("base-url", "", "external base URL")
	fs.String("store-dir", "", "store directory")
	fs.String("db-path", "", "sqlite db path")
	fs.String("packages-dir", "", "packages directory")
	fs.String("log-level", "", "log level")
	fs.String("signing-key-type", "", "signing key type")
	fs.Int("signing-key-bits", 0, "signing key bits")
	fs.String("signing-key-user-id", "", "signing key user id")
	fs.Int64("max-package-size", 0, "max package size in bytes")
	fs.Bool("open-registration", true, "allow new account registration")
	fs.Bool("tls-enabled", false, "enable TLS")
	fs.String("tls-cert", "", "TLS cert path")
	fs.String("tls-key", "", "TLS key path")
	if err := fs.Parse(a.args); err != nil {
		return config.Config{}, err
	}

	flags := map[string]string{}
	fs.Visit(func(f *flag.Flag) {
		flags[f.Name] = f.Value.String()
	})
	return config.Load(*configPath, flags)
}

// ensureDirs creates the store, database, and packages directories.
func ensureDirs(cfg config.Config) error {
	for _, dir := range []string{cfg.StoreDir, filepath.Dir(cfg.DBPath), cfg.PackagesDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return nil
}

// ensureSigningKey loads the default key from the DB or generates one.
func ensureSigningKey(ctx context.Context, st *store.Store, cfg config.Config) (*gpg.Key, error) {
	existing, err := st.GetDefaultGPGKey(ctx)
	if err == nil {
		key, err := gpg.ParseArmoredPrivate(existing.PrivateKeyArmored)
		if err != nil {
			return nil, fmt.Errorf("parse stored key: %w", err)
		}
		return key, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("load key: %w", err)
	}

	key, err := gpg.GenerateKey(cfg.SigningKeyUserID, cfg.SigningKeyBits)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	pub, err := key.ArmoredPublic()
	if err != nil {
		return nil, err
	}
	priv, err := key.ArmoredPrivate()
	if err != nil {
		return nil, err
	}
	if _, err := st.SaveGPGKey(ctx, key.Fingerprint, key.UserID, pub, priv, true); err != nil {
		return nil, fmt.Errorf("save key: %w", err)
	}
	return key, nil
}

// shutdownTimeout for graceful HTTP shutdown.
const shutdownTimeout = 30 * time.Second
