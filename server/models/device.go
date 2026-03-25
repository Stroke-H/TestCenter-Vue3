package models

type Device struct {
	ID         string `json:"id"`
	DeviceName string `json:"device_name"`
	OS         string `json:"os"`
	Model      string `json:"model"`
	AllowedApp string `json:"allowed_app"` // Comma-separated project codes
	CreatedAt  string `json:"created_at"`
}
