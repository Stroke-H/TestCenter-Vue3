package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"testcenter-server/models"
)

var (
	sandboxAccountsFile = "data/sandbox_accounts.jsonl"
	sandboxAccountsLock sync.RWMutex
)

var defaultSandboxAccounts = []models.SandboxAccount{
	{ID: "1", AccountType: "sandbox", Account: "test.en119@sandbox.com", Password: "Webeye123", ProjectCode: "A1106"},
	{ID: "2", AccountType: "sandbox", Account: "test.ja101@apple.cn", Password: "Webeye123", ProjectCode: "A1106"},
	{ID: "3", AccountType: "sandbox", Account: "testdn.en002@apple.cn", Password: "Webeye123", ProjectCode: "A1106"},
	{ID: "dataqa-1", AccountType: "dataqa", Account: "phone15-粉色 系统版本26", Password: "90976f13-1063-4aea-8a9b-82f4d34ac4fe", ProjectCode: "A1106"},
	{ID: "dataqa-2", AccountType: "dataqa", Account: "iphone13-粉色 系统版本18.6.1", Password: "61ed449a-0d86-413f-b488-605cb6e15056", ProjectCode: "A1106"},
	{ID: "dataqa-3", AccountType: "dataqa", Account: "iphone13-黑色 系统版本18.4", Password: "7c39d71e-fccc-4f9c-866b-cda8c804eb20", ProjectCode: "A1106"},
	{ID: "dataqa-4", AccountType: "dataqa", Account: "iphone8-黑色 系统版本16.7.1", Password: "ce1695c1-40cb-476f-a270-ff34cc9b2569", ProjectCode: "A1106"},
	{ID: "dataqa-5", AccountType: "dataqa", Account: "iphone15-钛钢 系统版本26", Password: "84e1a4aa-a306-4cf1-ac75-e92b21c5c5fb", ProjectCode: "A1106"},
}

func migrateDataqaProjectCodes() error {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()

	if err := ensureSandboxAccountsFile(); err != nil {
		return err
	}

	accounts, err := readSandboxAccountsUnlocked()
	if err != nil {
		return err
	}

	changed := false
	for i := range accounts {
		if accounts[i].AccountType != "dataqa" {
			continue
		}
		if accounts[i].ProjectCode != "A1106" {
			accounts[i].ProjectCode = "A1106"
			changed = true
		}
	}

	if !changed {
		return nil
	}
	return saveSandboxAccounts(accounts)
}

func ensureSandboxAccountsFile() error {
	if _, err := os.Stat(sandboxAccountsFile); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.Create(sandboxAccountsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	now := time.Now().Format(time.RFC3339)
	for _, account := range defaultSandboxAccounts {
		if account.CreatedAt == "" {
			account.CreatedAt = now
		}
		line, err := json.Marshal(account)
		if err != nil {
			continue
		}
		if _, err := writer.WriteString(string(line) + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func readSandboxAccountsUnlocked() ([]models.SandboxAccount, error) {
	file, err := os.Open(sandboxAccountsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var accounts []models.SandboxAccount
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var account models.SandboxAccount
		if err := json.Unmarshal([]byte(line), &account); err == nil {
			if account.AccountType == "" {
				account.AccountType = "sandbox"
			}
			accounts = append(accounts, account)
		}
	}
	return accounts, scanner.Err()
}

func ListSandboxAccounts() ([]models.SandboxAccount, error) {
	if err := ensureSandboxAccountsFile(); err != nil {
		return nil, err
	}

	// Ensure legacy records are consistent with the current "dataqa -> A1106" rule.
	if err := migrateDataqaProjectCodes(); err != nil {
		return nil, err
	}

	sandboxAccountsLock.RLock()
	defer sandboxAccountsLock.RUnlock()
	return readSandboxAccountsUnlocked()
}

func saveSandboxAccounts(accounts []models.SandboxAccount) error {
	file, err := os.Create(sandboxAccountsFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, account := range accounts {
		line, err := json.Marshal(account)
		if err != nil {
			continue
		}
		if _, err := writer.WriteString(string(line) + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func CreateSandboxAccount(account models.SandboxAccount) (*models.SandboxAccount, error) {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()

	if err := ensureSandboxAccountsFile(); err != nil {
		return nil, err
	}

	accounts, err := readSandboxAccountsUnlocked()
	if err != nil {
		return nil, err
	}

	account.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	if account.AccountType == "" {
		account.AccountType = "sandbox"
	}
	account.CreatedAt = time.Now().Format(time.RFC3339)
	accounts = append(accounts, account)

	if err := saveSandboxAccounts(accounts); err != nil {
		return nil, err
	}
	return &account, nil
}

func DeleteSandboxAccount(id string) error {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()

	if err := ensureSandboxAccountsFile(); err != nil {
		return err
	}

	accounts, err := readSandboxAccountsUnlocked()
	if err != nil {
		return err
	}

	filtered := make([]models.SandboxAccount, 0, len(accounts))
	for _, account := range accounts {
		if account.ID != id {
			filtered = append(filtered, account)
		}
	}

	return saveSandboxAccounts(filtered)
}

func UpdateSandboxAccount(account models.SandboxAccount) (*models.SandboxAccount, error) {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()

	if err := ensureSandboxAccountsFile(); err != nil {
		return nil, err
	}

	accounts, err := readSandboxAccountsUnlocked()
	if err != nil {
		return nil, err
	}

	targetIdx := -1
	var createdAt string
	for i := range accounts {
		if accounts[i].ID == account.ID {
			targetIdx = i
			createdAt = accounts[i].CreatedAt
			break
		}
	}
	if targetIdx == -1 {
		return nil, fmt.Errorf("sandbox account not found: %s", account.ID)
	}

	if account.AccountType == "" {
		account.AccountType = "sandbox"
	}

	// Preserve created_at to avoid breaking chronological views/logs.
	account.CreatedAt = createdAt
	accounts[targetIdx] = account

	if err := saveSandboxAccounts(accounts); err != nil {
		return nil, err
	}

	return &account, nil
}
