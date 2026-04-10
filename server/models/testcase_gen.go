package models

// RequirementPoint represents a structured part of a requirement
type RequirementPoint struct {
	Module      string   `json:"module"`
	Feature     string   `json:"feature"`
	Description string   `json:"description"`
	Rules       []string `json:"rules"`
}

// TestCase represents a generated test case
type TestCase struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"` // POSITIVE, NEGATIVE, EXCEPTION, CONCURRENCY
	Title          string      `json:"title"`
	Precondition   string      `json:"precondition"`
	Steps          []string    `json:"steps"`
	TestData       interface{} `json:"test_data"`
	ExpectedResult string      `json:"expected_result"`
	Priority       string      `json:"priority"`
	Remark         string      `json:"remark"`
}

// GenerationRecord represents a historical generation session
type GenerationRecord struct {
	ID              string             `json:"id"`
	Title           string             `json:"title"`
	RequirementText string             `json:"requirement_text"`
	CreatedAt       string             `json:"created_at"` // ISO8601 string
	Points          []RequirementPoint `json:"points"`
	Cases           []TestCase         `json:"cases"`
}
