// Package config defines the urapt-server configuration and its loading from
// defaults, a TOML file, environment variables, and command-line flags, with
// later sources overriding earlier ones.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Defaults applied before any other source.
var Defaults = Config{
	Bind:             "0.0.0.0:8080",
	BaseURL:          "http://localhost:8080",
	StoreDir:         "./store",
	LogLevel:         "info",
	SigningKeyType:   "rsa",
	SigningKeyBits:   4096,
	MaxPackageSize:   1024 * 1024 * 1024,
	OpenRegistration: true,
	ConfigPath:       "./urapt-server.toml",
}

// Config is the urapt-server runtime configuration.
type Config struct {
	Bind             string `toml:"bind"`
	BaseURL          string `toml:"base_url"`
	StoreDir         string `toml:"store_dir"`
	DBPath           string `toml:"db_path"`
	PackagesDir      string `toml:"packages_dir"`
	LogLevel         string `toml:"log_level"`
	SigningKeyType   string `toml:"signing_key_type"`
	SigningKeyBits   int    `toml:"signing_key_bits"`
	SigningKeyUserID string `toml:"signing_key_user_id"`
	MaxPackageSize   int64  `toml:"max_package_size"`
	OpenRegistration bool   `toml:"open_registration"`
	TLSEnabled       bool   `toml:"tls_enabled"`
	TLSCert          string `toml:"tls_cert"`
	TLSKey           string `toml:"tls_key"`

	ConfigPath string `toml:"-"`
}

// Load builds the effective Config from Defaults -> file -> env -> flags.
// Flags is a map of flag name to string value (already parsed by the caller).
func Load(configPath string, flags map[string]string) (Config, error) {
	c := Defaults
	c.ConfigPath = configPath

	if err := applyFile(&c, configPath); err != nil {
		return Config{}, err
	}
	applyEnv(&c)
	if err := applyFlags(&c, flags); err != nil {
		return Config{}, err
	}
	c.finalize()
	return c, nil
}

func applyFile(c *Config, path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := toml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}

func applyEnv(c *Config) {
	set(c, "URAPT_BIND", &c.Bind)
	set(c, "URAPT_BASE_URL", &c.BaseURL)
	set(c, "URAPT_STORE_DIR", &c.StoreDir)
	set(c, "URAPT_DB_PATH", &c.DBPath)
	set(c, "URAPT_PACKAGES_DIR", &c.PackagesDir)
	set(c, "URAPT_LOG_LEVEL", &c.LogLevel)
	set(c, "URAPT_SIGNING_KEY_TYPE", &c.SigningKeyType)
	set(c, "URAPT_SIGNING_KEY_USER_ID", &c.SigningKeyUserID)
	set(c, "URAPT_TLS_CERT", &c.TLSCert)
	set(c, "URAPT_TLS_KEY", &c.TLSKey)
	setInt(c, "URAPT_SIGNING_KEY_BITS", &c.SigningKeyBits)
	setInt64(c, "URAPT_MAX_PACKAGE_SIZE", &c.MaxPackageSize)
	setBool(c, "URAPT_OPEN_REGISTRATION", &c.OpenRegistration)
	setBool(c, "URAPT_TLS_ENABLED", &c.TLSEnabled)
}

func applyFlags(c *Config, flags map[string]string) error {
	for k, v := range flags {
		switch k {
		case "bind":
			c.Bind = v
		case "base-url":
			c.BaseURL = v
		case "store-dir":
			c.StoreDir = v
		case "db-path":
			c.DBPath = v
		case "packages-dir":
			c.PackagesDir = v
		case "log-level":
			c.LogLevel = v
		case "signing-key-type":
			c.SigningKeyType = v
		case "signing-key-bits":
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("invalid --signing-key-bits %q: %w", v, err)
			}
			c.SigningKeyBits = n
		case "signing-key-user-id":
			c.SigningKeyUserID = v
		case "max-package-size":
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid --max-package-size %q: %w", v, err)
			}
			c.MaxPackageSize = n
		case "open-registration":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid --open-registration %q: %w", v, err)
			}
			c.OpenRegistration = b
		case "tls-enabled":
			b, err := strconv.ParseBool(v)
			if err != nil {
				return fmt.Errorf("invalid --tls-enabled %q: %w", v, err)
			}
			c.TLSEnabled = b
		case "tls-cert":
			c.TLSCert = v
		case "tls-key":
			c.TLSKey = v
		case "config":
			// already handled
		}
	}
	return nil
}

// finalize fills derived defaults: DBPath and PackagesDir default under
// StoreDir, and SigningKeyUserID gets a hostname-based default.
func (c *Config) finalize() {
	if c.DBPath == "" {
		c.DBPath = filepath.Join(c.StoreDir, "database", "sqlite.db")
	}
	if c.PackagesDir == "" {
		c.PackagesDir = filepath.Join(c.StoreDir, "packages")
	}
	if c.SigningKeyUserID == "" {
		host, err := os.Hostname()
		if err != nil || host == "" {
			host = "localhost"
		}
		c.SigningKeyUserID = "urapt-server <" + host + ">"
	}
	if c.MaxPackageSize <= 0 {
		c.MaxPackageSize = Defaults.MaxPackageSize
	}
}

// Validate checks the config for obvious errors before startup.
func (c *Config) Validate() error {
	if c.Bind == "" {
		return fmt.Errorf("bind address is required")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base_url is required")
	}
	c.BaseURL = strings.TrimRight(c.BaseURL, "/")
	if c.SigningKeyBits <= 0 {
		return fmt.Errorf("signing_key_bits must be positive")
	}
	if c.TLSEnabled && (c.TLSCert == "" || c.TLSKey == "") {
		return fmt.Errorf("tls_enabled requires tls_cert and tls_key")
	}
	return nil
}

func set(c *Config, env string, dst *string) {
	if v, ok := os.LookupEnv(env); ok && v != "" {
		*dst = v
	}
}

func setInt(c *Config, env string, dst *int) {
	if v, ok := os.LookupEnv(env); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

func setInt64(c *Config, env string, dst *int64) {
	if v, ok := os.LookupEnv(env); ok && v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*dst = n
		}
	}
}

func setBool(c *Config, env string, dst *bool) {
	if v, ok := os.LookupEnv(env); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			*dst = b
		}
	}
}
