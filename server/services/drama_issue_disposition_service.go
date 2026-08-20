package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	dramaIssueActionDataIssue          = "data_issue"
	dramaIssueActionTemporarilyIgnored = "temporarily_ignored"
)

type dramaIssueDescriptor struct {
	ClientKey string   `json:"clientKey,omitempty"`
	DramaID   string   `json:"dramaId"`
	Errors    []string `json:"errors"`
}

type dramaIssueDisposition struct {
	Signature   string   `json:"signature"`
	DramaID     string   `json:"dramaId"`
	Errors      []string `json:"errors"`
	Action      string   `json:"action"`
	SourceRunID string   `json:"sourceRunId"`
	UpdatedBy   string   `json:"updatedBy,omitempty"`
	UpdatedAt   string   `json:"updatedAt"`
}

type dramaIssueDispositionStore struct {
	Version int                              `json:"version"`
	Items   map[string]dramaIssueDisposition `json:"items"`
}

type resolveDramaIssueDispositionsRequest struct {
	Issues []dramaIssueDescriptor `json:"issues"`
}

type saveDramaIssueDispositionRequest struct {
	DramaID string   `json:"dramaId"`
	Errors  []string `json:"errors"`
	Action  string   `json:"action"`
}

var dramaIssueDispositionMu sync.Mutex

func ResolveDramaIssueDispositionsHandler(c *gin.Context) {
	var request resolveDramaIssueDispositionsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue disposition request"})
		return
	}
	if len(request.Issues) > 5000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "too many issues"})
		return
	}

	store, err := loadDramaIssueDispositionStore()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load issue dispositions"})
		return
	}

	items := make([]map[string]any, 0, len(request.Issues))
	for _, issue := range request.Issues {
		signature := dramaIssueSignature(issue.DramaID, issue.Errors)
		if signature == "" {
			continue
		}
		disposition := store.Items[signature]
		items = append(items, map[string]any{
			"clientKey": issue.ClientKey,
			"signature": signature,
			"action":    disposition.Action,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func SaveDramaIssueDispositionHandler(c *gin.Context) {
	runID := sanitizeReportSuffix(c.Param("runId"))
	if strings.TrimSpace(runID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "runId is required"})
		return
	}

	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: Invalid token"})
		return
	}

	var request saveDramaIssueDispositionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue disposition request"})
		return
	}
	if !isValidDramaIssueAction(request.Action) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported issue disposition"})
		return
	}

	signature := dramaIssueSignature(request.DramaID, request.Errors)
	if signature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dramaId and error details are required"})
		return
	}
	if !archivedDramaRunContainsIssue(runID, signature) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "issue does not match this archived report"})
		return
	}

	disposition := dramaIssueDisposition{
		Signature:   signature,
		DramaID:     normalizeDramaIssueDramaID(request.DramaID),
		Errors:      normalizeDramaIssueErrors(request.Errors),
		Action:      request.Action,
		SourceRunID: runID,
		UpdatedBy:   firstNonEmpty(user.Nickname, user.Username),
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	if err := saveDramaIssueDisposition(disposition); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save issue disposition"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"signature": signature,
		"action":    disposition.Action,
		"updatedBy": disposition.UpdatedBy,
		"updatedAt": disposition.UpdatedAt,
	})
}

func isValidDramaIssueAction(action string) bool {
	return action == dramaIssueActionDataIssue || action == dramaIssueActionTemporarilyIgnored
}

func archivedDramaRunContainsIssue(runID string, signature string) bool {
	runDir := filepath.Join(testRunStorageRoot(projectRootDir()), runID)
	metadataContent, err := os.ReadFile(filepath.Join(runDir, "metadata.json"))
	if err != nil {
		return false
	}
	var archive TestRunArchive
	if json.Unmarshal(metadataContent, &archive) != nil || archive.TestType != "drama" {
		return false
	}
	reportHTML := ""
	if content, readErr := os.ReadFile(filepath.Join(runDir, "artifacts", "report.html")); readErr == nil {
		reportHTML = string(content)
	}
	for _, failure := range loadArchivedDramaFailures(runDir, reportHTML) {
		if dramaIssueSignature(failure.DramaID, failure.Errors) == signature {
			return true
		}
	}
	return false
}

func dramaIssueSignature(dramaID string, errors []string) string {
	dramaID = normalizeDramaIssueDramaID(dramaID)
	normalizedErrors := normalizeDramaIssueErrors(errors)
	if dramaID == "" || len(normalizedErrors) == 0 {
		return ""
	}
	payload := dramaID + "\n" + strings.Join(normalizedErrors, "\n")
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func normalizeDramaIssueDramaID(dramaID string) string {
	return strings.ToLower(strings.TrimSpace(dramaID))
}

func normalizeDramaIssueErrors(errors []string) []string {
	normalized := make([]string, 0, len(errors))
	seen := make(map[string]bool, len(errors))
	for _, raw := range errors {
		line := normalizeDramaIssueError(raw)
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		normalized = append(normalized, line)
	}
	sort.Strings(normalized)
	return normalized
}

func normalizeDramaIssueError(raw string) string {
	line := strings.TrimSpace(htmlToPlainText(raw))
	line = strings.TrimSpace(strings.TrimPrefix(line, "•"))
	return strings.Join(strings.Fields(line), " ")
}

func filterTemporarilyIgnoredDramaFailures(failures []dramaFailureSummary) ([]dramaFailureSummary, int) {
	store, err := loadDramaIssueDispositionStore()
	if err != nil {
		return failures, 0
	}
	visible := make([]dramaFailureSummary, 0, len(failures))
	ignored := 0
	for _, failure := range failures {
		signature := dramaIssueSignature(failure.DramaID, failure.Errors)
		if disposition, ok := store.Items[signature]; ok && disposition.Action == dramaIssueActionTemporarilyIgnored {
			ignored++
			continue
		}
		visible = append(visible, failure)
	}
	return visible, ignored
}

func dramaIssueDispositionFilePath() string {
	if override := strings.TrimSpace(os.Getenv("DRAMA_ISSUE_DISPOSITIONS_FILE")); override != "" {
		return override
	}
	return filepath.Join(projectRootDir(), "server", "data", "drama_issue_dispositions.json")
}

func loadDramaIssueDispositionStore() (dramaIssueDispositionStore, error) {
	dramaIssueDispositionMu.Lock()
	defer dramaIssueDispositionMu.Unlock()
	return loadDramaIssueDispositionStoreUnlocked()
}

func loadDramaIssueDispositionStoreUnlocked() (dramaIssueDispositionStore, error) {
	store := dramaIssueDispositionStore{Version: 1, Items: map[string]dramaIssueDisposition{}}
	content, err := os.ReadFile(dramaIssueDispositionFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}
		return store, err
	}
	if len(strings.TrimSpace(string(content))) == 0 {
		return store, nil
	}
	if err := json.Unmarshal(content, &store); err != nil {
		return store, err
	}
	if store.Items == nil {
		store.Items = map[string]dramaIssueDisposition{}
	}
	if store.Version == 0 {
		store.Version = 1
	}
	return store, nil
}

func saveDramaIssueDisposition(disposition dramaIssueDisposition) error {
	dramaIssueDispositionMu.Lock()
	defer dramaIssueDispositionMu.Unlock()

	store, err := loadDramaIssueDispositionStoreUnlocked()
	if err != nil {
		return err
	}
	store.Items[disposition.Signature] = disposition
	return writeDramaIssueDispositionStoreUnlocked(store)
}

func writeDramaIssueDispositionStoreUnlocked(store dramaIssueDispositionStore) error {
	path := dramaIssueDispositionFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	content, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	temporaryPath := path + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	if err := os.WriteFile(temporaryPath, content, 0600); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	return nil
}
