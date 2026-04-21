package model

import (
	"context"
	"encoding/json"
	"sync"
	"testcenter-server/database"
	"time"
)

type TestPhone struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	OS         string `json:"os"`
	Model      string `json:"model"`
	AllowedApp string `json:"allowed_app"`
	CreatedAt  string `json:"created_at"`
}

var (
	testPhones     []TestPhone
	testPhonesLock sync.RWMutex
	testPhonesFile = "data/test_phones.jsonl"
)

func GetTestPhones() []TestPhone {
	testPhonesLock.RLock()
	defer testPhonesLock.RUnlock()
	return testPhones
}

func AddTestPhone(phone TestPhone) error {
	testPhonesLock.Lock()
	defer testPhonesLock.Unlock()

	if phone.ID == "" {
		phone.ID = time.Now().Format("20060102150405")
	}
	if phone.CreatedAt == "" {
		phone.CreatedAt = time.Now().Format(time.RFC3339)
	}

	testPhones = append(testPhones, phone)
	return upsertTestPhone(phone)
}

func saveTestPhones() error {
	for _, p := range testPhones {
		if err := upsertTestPhone(p); err != nil {
			return err
		}
	}
	return nil
}

func LoadTestPhones() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := database.NewManager().DB(ctx)
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "SELECT `raw_json` FROM `test_phones` ORDER BY `migrated_at` ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	var loaded []TestPhone
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		var p TestPhone
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		loaded = append(loaded, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	testPhonesLock.Lock()
	testPhones = loaded
	testPhonesLock.Unlock()
	return nil
}

func upsertTestPhone(phone TestPhone) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	db, _, err := database.NewManager().DB(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(phone)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO test_phones (id, device_name, os, model, allowed_app, created_at, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			device_name = VALUES(device_name),
			os = VALUES(os),
			model = VALUES(model),
			allowed_app = VALUES(allowed_app),
			created_at = VALUES(created_at),
			raw_json = VALUES(raw_json),
			migrated_at = CURRENT_TIMESTAMP
	`, phone.ID, phone.DeviceName, phone.OS, phone.Model, phone.AllowedApp, phone.CreatedAt, string(raw))
	return err
}
