package database

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

const (
	defaultDriver          = "mysql"
	defaultHost            = "127.0.0.1"
	defaultPort            = 3306
	defaultMaxOpenConns    = 10
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 30 * time.Minute
	defaultTimeout         = 5 * time.Second
	defaultConfigFile      = "data/database_config.json"
)

// Config describes the runtime database connection settings.
// Keep credentials in environment variables instead of committing them to repo files.
type Config struct {
	Driver          string        `json:"driver"`
	Host            string        `json:"host"`
	Port            int           `json:"port"`
	Username        string        `json:"username"`
	Password        string        `json:"password"`
	Database        string        `json:"database"`
	MaxOpenConns    int           `json:"maxOpenConns"`
	MaxIdleConns    int           `json:"maxIdleConns"`
	ConnMaxLifetime time.Duration `json:"-"`
	Timeout         time.Duration `json:"-"`
	ReadTimeout     time.Duration `json:"-"`
	WriteTimeout    time.Duration `json:"-"`
	ParseTime       bool          `json:"parseTime"`
}

type fileConfig struct {
	Driver          string `json:"driver"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Database        string `json:"database"`
	MaxOpenConns    int    `json:"maxOpenConns"`
	MaxIdleConns    int    `json:"maxIdleConns"`
	ConnMaxLifetime string `json:"connMaxLifetime"`
	Timeout         string `json:"timeout"`
	ReadTimeout     string `json:"readTimeout"`
	WriteTimeout    string `json:"writeTimeout"`
	ParseTime       *bool  `json:"parseTime"`
}

type SafeConfig struct {
	Configured      bool   `json:"configured"`
	Driver          string `json:"driver"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username"`
	Database        string `json:"database"`
	MaxOpenConns    int    `json:"maxOpenConns"`
	MaxIdleConns    int    `json:"maxIdleConns"`
	ConnMaxLifetime string `json:"connMaxLifetime"`
	Timeout         string `json:"timeout"`
	ReadTimeout     string `json:"readTimeout"`
	WriteTimeout    string `json:"writeTimeout"`
	ParseTime       bool   `json:"parseTime"`
}

func LoadConfigFromEnv() Config {
	cfg := loadConfigFromFile(getenv("TESTCENTER_DB_CONFIG_FILE", defaultConfigFile))

	cfg.Driver = getenv("TESTCENTER_DB_DRIVER", cfg.Driver)
	cfg.Host = getenv("TESTCENTER_DB_HOST", cfg.Host)
	cfg.Port = getenvInt("TESTCENTER_DB_PORT", cfg.Port)
	cfg.Username = getenv("TESTCENTER_DB_USER", cfg.Username)
	cfg.Password = getenv("TESTCENTER_DB_PASSWORD", cfg.Password)
	cfg.Database = getenv("TESTCENTER_DB_NAME", cfg.Database)
	cfg.MaxOpenConns = getenvInt("TESTCENTER_DB_MAX_OPEN_CONNS", cfg.MaxOpenConns)
	cfg.MaxIdleConns = getenvInt("TESTCENTER_DB_MAX_IDLE_CONNS", cfg.MaxIdleConns)
	cfg.ConnMaxLifetime = getenvDuration("TESTCENTER_DB_CONN_MAX_LIFETIME", cfg.ConnMaxLifetime)
	cfg.Timeout = getenvDuration("TESTCENTER_DB_TIMEOUT", cfg.Timeout)
	cfg.ReadTimeout = getenvDuration("TESTCENTER_DB_READ_TIMEOUT", cfg.ReadTimeout)
	cfg.WriteTimeout = getenvDuration("TESTCENTER_DB_WRITE_TIMEOUT", cfg.WriteTimeout)
	cfg.ParseTime = getenvBool("TESTCENTER_DB_PARSE_TIME", cfg.ParseTime)

	return cfg
}

func (c Config) IsConfigured() bool {
	return c.Driver != "" && c.Host != "" && c.Port > 0 && c.Username != "" && c.Database != ""
}

func (c Config) Safe() SafeConfig {
	return SafeConfig{
		Configured:      c.IsConfigured(),
		Driver:          c.Driver,
		Host:            c.Host,
		Port:            c.Port,
		Username:        c.Username,
		Database:        c.Database,
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime.String(),
		Timeout:         c.Timeout.String(),
		ReadTimeout:     c.ReadTimeout.String(),
		WriteTimeout:    c.WriteTimeout.String(),
		ParseTime:       c.ParseTime,
	}
}

func (c Config) DSN() (string, error) {
	if c.Driver != defaultDriver {
		return "", fmt.Errorf("unsupported database driver %q", c.Driver)
	}
	if !c.IsConfigured() {
		return "", fmt.Errorf("database config is incomplete: TESTCENTER_DB_USER and TESTCENTER_DB_NAME are required")
	}

	cfg := mysql.NewConfig()
	cfg.User = c.Username
	cfg.Passwd = c.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	cfg.DBName = c.Database
	cfg.ParseTime = c.ParseTime
	cfg.Timeout = c.Timeout
	cfg.ReadTimeout = c.ReadTimeout
	cfg.WriteTimeout = c.WriteTimeout
	cfg.Params = map[string]string{
		"charset":   "utf8mb4",
		"collation": "utf8mb4_unicode_ci",
	}

	return cfg.FormatDSN(), nil
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func loadConfigFromFile(path string) Config {
	cfg := Config{
		Driver:          defaultDriver,
		Host:            defaultHost,
		Port:            defaultPort,
		MaxOpenConns:    defaultMaxOpenConns,
		MaxIdleConns:    defaultMaxIdleConns,
		ConnMaxLifetime: defaultConnMaxLifetime,
		Timeout:         defaultTimeout,
		ReadTimeout:     defaultTimeout,
		WriteTimeout:    defaultTimeout,
		ParseTime:       true,
	}

	data, err := os.ReadFile(path)
	if err != nil && !filepath.IsAbs(path) {
		data, err = os.ReadFile(filepath.Join("server", path))
	}
	if err != nil {
		return cfg
	}

	var fileCfg fileConfig
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}

	if fileCfg.Driver != "" {
		cfg.Driver = fileCfg.Driver
	}
	if fileCfg.Host != "" {
		cfg.Host = fileCfg.Host
	}
	if fileCfg.Port > 0 {
		cfg.Port = fileCfg.Port
	}
	if fileCfg.Username != "" {
		cfg.Username = fileCfg.Username
	}
	if fileCfg.Password != "" {
		cfg.Password = fileCfg.Password
	}
	if fileCfg.Database != "" {
		cfg.Database = fileCfg.Database
	}
	if fileCfg.MaxOpenConns > 0 {
		cfg.MaxOpenConns = fileCfg.MaxOpenConns
	}
	if fileCfg.MaxIdleConns > 0 {
		cfg.MaxIdleConns = fileCfg.MaxIdleConns
	}
	if value := parseDuration(fileCfg.ConnMaxLifetime); value > 0 {
		cfg.ConnMaxLifetime = value
	}
	if value := parseDuration(fileCfg.Timeout); value > 0 {
		cfg.Timeout = value
	}
	if value := parseDuration(fileCfg.ReadTimeout); value > 0 {
		cfg.ReadTimeout = value
	}
	if value := parseDuration(fileCfg.WriteTimeout); value > 0 {
		cfg.WriteTimeout = value
	}
	if fileCfg.ParseTime != nil {
		cfg.ParseTime = *fileCfg.ParseTime
	}

	return cfg
}

func parseDuration(value string) time.Duration {
	if value == "" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return parsed
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
