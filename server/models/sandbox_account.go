package models

type SandboxAccount struct {
	ID          string `json:"id"`
	AccountType string `json:"account_type"`
	Account     string `json:"account"`
	Password    string `json:"password"`
	ProjectCode string `json:"project_code"`
	CreatedAt   string `json:"created_at"`
}
