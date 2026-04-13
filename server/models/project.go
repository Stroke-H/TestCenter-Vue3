package models

type Project struct {
	ID          string `json:"id"`
	ProjectCode string `json:"project_code"`
	ProjectName string `json:"project_name"`
	ShortCode   string `json:"short_code"`
	WikiURL     string `json:"wiki_url"`
	Workspace   string `json:"workspace"`
	CreatedAt   string `json:"created_at"`
}
