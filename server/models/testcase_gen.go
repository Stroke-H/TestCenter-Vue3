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
	SourceModule   string      `json:"source_module,omitempty"`
	SourceFeature  string      `json:"source_feature,omitempty"`
	Category       string      `json:"category,omitempty"` // 常规功能测试 / 边界极限测试 / 异常容错测试 / 稳定性并发测试
	Type           string      `json:"type"`               // POSITIVE, NEGATIVE, EXCEPTION, CONCURRENCY
	Title          string      `json:"title"`
	Precondition   string      `json:"precondition"`
	Steps          []string    `json:"steps"`
	TestData       interface{} `json:"test_data"`
	ExpectedResult string      `json:"expected_result"`
	Priority       string      `json:"priority"`
	Remark         string      `json:"remark"`
}

type TestCaseReviewRole struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IdentityMD  string `json:"identity_md"`
}

type TestCaseReviewResult struct {
	RoleKey              string   `json:"role_key"`
	RoleName             string   `json:"role_name"`
	IdentityMD           string   `json:"identity_md"`
	OverallConclusion    string   `json:"overall_conclusion"`
	Highlights           []string `json:"highlights"`
	MissingCoverage      []string `json:"missing_coverage"`
	SuggestedNewCases    []string `json:"suggested_new_cases"`
	SuggestedMergeOrDrop []string `json:"suggested_merge_or_drop"`
	RiskLevel            string   `json:"risk_level"`
	ReviewFocus          []string `json:"review_focus"`
	ReviewedAt           string   `json:"reviewed_at"`
	AIProvider           string   `json:"ai_provider,omitempty"`
	AIModel              string   `json:"ai_model,omitempty"`
}

// GenerationRecord represents a historical generation session
type GenerationRecord struct {
	ID              string                          `json:"id"`
	Title           string                          `json:"title"`
	ProjectCode     string                          `json:"project_code"`
	Module          string                          `json:"module"`
	RequirementText string                          `json:"requirement_text"`
	CreatedAt       string                          `json:"created_at"` // ISO8601 string
	Points          []RequirementPoint              `json:"points"`
	Cases           []TestCase                      `json:"cases"`
	ReviewResults   map[string]TestCaseReviewResult `json:"review_results,omitempty"`
}
