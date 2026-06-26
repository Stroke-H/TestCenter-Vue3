package services

import (
	"fmt"
	"sync"
	"time"

	"testcenter-server/models"
)

var (
	sandboxAccountsLock sync.RWMutex
)

var defaultSandboxAccounts = []models.SandboxAccount{}

func migrateDataqaProjectCodes() error {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()
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

func ensureSandboxAccountsSeeded() error {
	accounts, err := sqlListJSON[models.SandboxAccount]("sandbox_accounts", "`migrated_at` ASC")
	if err != nil {
		return err
	}
	if len(accounts) > 0 {
		return nil
	}
	now := time.Now().Format(time.RFC3339)
	seed := make([]models.SandboxAccount, 0, len(defaultSandboxAccounts))
	for _, account := range defaultSandboxAccounts {
		if account.CreatedAt == "" {
			account.CreatedAt = now
		}
		seed = append(seed, account)
	}
	return sqlReplaceAllJSON("sandbox_accounts", seed)
}

func readSandboxAccountsUnlocked() ([]models.SandboxAccount, error) {
	accounts, err := sqlListJSON[models.SandboxAccount]("sandbox_accounts", "`migrated_at` ASC")
	if err != nil {
		return nil, err
	}
	for i := range accounts {
		if accounts[i].AccountType == "" {
			accounts[i].AccountType = "sandbox"
		}
	}
	return accounts, nil
}

func ListSandboxAccounts() ([]models.SandboxAccount, error) {
	if err := ensureSandboxAccountsSeeded(); err != nil {
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
	return sqlReplaceAllJSON("sandbox_accounts", accounts)
}

func CreateSandboxAccount(account models.SandboxAccount) (*models.SandboxAccount, error) {
	sandboxAccountsLock.Lock()
	defer sandboxAccountsLock.Unlock()

	if err := ensureSandboxAccountsSeeded(); err != nil {
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

	if err := ensureSandboxAccountsSeeded(); err != nil {
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

	if err := ensureSandboxAccountsSeeded(); err != nil {
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
