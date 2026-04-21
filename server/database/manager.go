package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type HealthStatus struct {
	Configured bool           `json:"configured"`
	Connected  bool           `json:"connected"`
	Config     SafeConfig     `json:"config"`
	Stats      sql.DBStats    `json:"stats,omitempty"`
	Latency    string         `json:"latency,omitempty"`
	Error      string         `json:"error,omitempty"`
	CheckedAt  time.Time      `json:"checkedAt"`
	Server     *ServerSummary `json:"server,omitempty"`
}

type ServerSummary struct {
	Version     string `json:"version"`
	Database    string `json:"database"`
	CurrentUser string `json:"currentUser"`
}

type Manager struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) DB(ctx context.Context) (*sql.DB, Config, error) {
	cfg := LoadConfigFromEnv()
	if !cfg.IsConfigured() {
		return nil, cfg, fmt.Errorf("database config is incomplete")
	}

	m.mu.RLock()
	if m.db != nil {
		db := m.db
		m.mu.RUnlock()
		return db, cfg, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db != nil {
		return m.db, cfg, nil
	}

	dsn, err := cfg.DSN()
	if err != nil {
		return nil, cfg, err
	}

	db, err := sql.Open(cfg.Driver, dsn)
	if err != nil {
		return nil, cfg, err
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, cfg, err
	}

	m.db = db
	return m.db, cfg, nil
}

func (m *Manager) Health(ctx context.Context) HealthStatus {
	start := time.Now()
	cfg := LoadConfigFromEnv()
	status := HealthStatus{
		Configured: cfg.IsConfigured(),
		Config:     cfg.Safe(),
		CheckedAt:  time.Now(),
	}
	if !cfg.IsConfigured() {
		status.Error = "database config is incomplete"
		return status
	}

	db, _, err := m.DB(ctx)
	if err != nil {
		status.Error = err.Error()
		status.Latency = time.Since(start).Round(time.Millisecond).String()
		return status
	}

	var server ServerSummary
	if err := db.QueryRowContext(ctx, "SELECT VERSION(), DATABASE(), CURRENT_USER()").Scan(&server.Version, &server.Database, &server.CurrentUser); err != nil {
		status.Error = err.Error()
		status.Latency = time.Since(start).Round(time.Millisecond).String()
		status.Stats = db.Stats()
		return status
	}

	status.Connected = true
	status.Server = &server
	status.Stats = db.Stats()
	status.Latency = time.Since(start).Round(time.Millisecond).String()
	return status
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil
	return err
}
