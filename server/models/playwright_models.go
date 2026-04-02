package models

// PlaywrightSuite represents a collection of test cases with shared variables and context.
type PlaywrightSuite struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Variables   map[string]string `json:"variables"`   // Suite-level variables
	Setup       []TestStep        `json:"setup"`       // Global setup steps for the suite
	Teardown    []TestStep        `json:"teardown"`    // Global teardown steps for the suite
	CaseIDs     []string          `json:"case_ids"`    // List of test case IDs included in this suite
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// PlaywrightCase represents a single automated test scenario.
type PlaywrightCase struct {
	ID          string            `json:"id"`
	SuiteID     string            `json:"suite_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Variables   map[string]string `json:"variables"`   // Case-level variables
	Steps       []TestStep        `json:"steps"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// TestStep represents a single atomic action within a test case.
type TestStep struct {
	ID          string            `json:"id"`
	Keyword     string            `json:"keyword"`      // Keyword name (e.g., "Goto", "Click")
	Args        map[string]string `json:"args"`         // Arguments for the keyword
	ReturnVar   string            `json:"return_var"`   // Variable name to store the return value
	Description string            `json:"description"`  // Step description or comment
	Disabled    bool              `json:"disabled"`     // Whether to skip this step
}

// UserKeyword represents a reusable sequence of steps defined by the user.
type UserKeyword struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Args        []ArgDef   `json:"args"`          // Arguments definition for the user keyword
	Steps       []TestStep `json:"steps"`         // Sequence of steps
	CreatedAt   string     `json:"created_at"`
}

// ArgDef defines an argument for a UserKeyword.
type ArgDef struct {
	Name         string `json:"name"`
	DefaultValue string `json:"default_value"`
	Required     bool   `json:"required"`
}

// PlaywrightReport represents the execution result of a test case or suite.
type PlaywrightReport struct {
	ID           string        `json:"id"`
	CaseID       string        `json:"case_id"`
	CaseName     string        `json:"case_name"`
	Status       string        `json:"status"` // "Passed", "Failed", "Stopped"
	Duration     string        `json:"duration"`
	StepResults  []StepResult  `json:"step_results"`
	ExecutedAt   string        `json:"executed_at"`
	ExecutedBy   string        `json:"executed_by"`
}

// StepResult stores the result of an individual test step execution.
type StepResult struct {
	StepIndex    int    `json:"step_index"`
	Keyword      string `json:"keyword"`
	Status       string `json:"status"`  // "pass", "fail", "skip"
	Duration     string `json:"duration"`
	Screenshot   string `json:"screenshot"` // Base64 or local file path
	ErrorMessage string `json:"error_message"`
}
