package model

import (
	"encoding/json"
	"os"
	"sync"
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
	return saveTestPhones()
}

func saveTestPhones() error {
	file, err := os.Create(testPhonesFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, p := range testPhones {
		if err := encoder.Encode(p); err != nil {
			return err
		}
	}
	return nil
}

func LoadTestPhones() error {
	file, err := os.Open(testPhonesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	var loaded []TestPhone
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var p TestPhone
		if err := decoder.Decode(&p); err != nil {
			return err
		}
		loaded = append(loaded, p)
	}

	testPhonesLock.Lock()
	testPhones = loaded
	testPhonesLock.Unlock()
	return nil
}
