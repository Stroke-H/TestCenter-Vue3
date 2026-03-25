package models

type Project struct {
	ID          string `json:"id"`
	ProjectCode string `json:"project_code"`
	ProjectName string `json:"project_name"`
	ShortCode   string `json:"short_code"`
	CreatedAt   string `json:"created_at"`
}
