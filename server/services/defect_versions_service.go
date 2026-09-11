package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

func normalizeDefectVersions(values []string) ([]string, error) {
	if len(values) > 500 {
		return nil, errors.New("每个项目最多配置 500 个版本")
	}
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || utf8.RuneCountInString(value) > 128 || strings.ContainsAny(value, "\r\n\t") {
			return nil, errors.New("版本号不能为空、包含换行或超过 128 个字符")
		}
		if !seen[value] {
			result = append(result, value)
			seen[value] = true
		}
	}
	return result, nil
}

type DefectVersionConfig struct {
	Rule     string   `json:"rule,omitempty"`
	Online   string   `json:"online"`
	Testing  string   `json:"testing"`
	Versions []string `json:"versions"`
}

func decodeDefectVersionConfig(raw string) (DefectVersionConfig, error) {
	config := DefectVersionConfig{Versions: []string{}}
	// Older configurations contain an unlabelled array; do not guess its stages.
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		err := json.Unmarshal([]byte(raw), &config.Versions)
		return config, err
	}
	err := json.Unmarshal([]byte(raw), &config)
	return config, err
}

func listDefectVersions(ctx context.Context) (map[string]DefectVersionConfig, error) {
	if err := ensureDefectSchema(ctx); err != nil {
		return nil, err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, "SELECT project_code, versions_json FROM defect_project_versions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]DefectVersionConfig{}
	for rows.Next() {
		var project, raw string
		if err := rows.Scan(&project, &raw); err != nil {
			return nil, err
		}
		values, err := decodeDefectVersionConfig(raw)
		if err != nil {
			return nil, err
		}
		result[project] = values
	}
	return result, rows.Err()
}

func defectVersionAllowed(value, previous string, versions []string, configured bool) bool {
	if value == "" || value == previous {
		return true
	}
	if !configured {
		return utf8.RuneCountInString(value) <= 128
	}
	for _, version := range versions {
		if value == version {
			return true
		}
	}
	return false
}

func validateDefectVersion(ctx context.Context, project, value, previous string) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	var raw string
	err = db.QueryRowContext(ctx, "SELECT versions_json FROM defect_project_versions WHERE project_code=?", project).Scan(&raw)
	configured := !errors.Is(err, sql.ErrNoRows)
	if err != nil && configured {
		return err
	}
	versions := []string{}
	if configured {
		config, err := decodeDefectVersionConfig(raw)
		if err != nil {
			return err
		}
		versions = config.Versions
	}
	if !defectVersionAllowed(value, previous, versions, configured) {
		return fmt.Errorf("版本号不在项目 %s 的版本配置中，请刷新后选择", project)
	}
	return nil
}

func SaveDefectVersionsHandler(c *gin.Context) {
	user, ok := requirePermissionAdmin(c)
	if !ok {
		return
	}
	project := c.Param("project_code")
	if _, err := defectProject(project); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var req struct {
		Online  *string `json:"online"`
		Testing *string `json:"testing"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Online == nil || req.Testing == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供当前线上版本和提测版本"})
		return
	}
	config := DefectVersionConfig{Online: strings.TrimSpace(*req.Online), Testing: strings.TrimSpace(*req.Testing)}
	inputs := []string{}
	for _, value := range []string{config.Online, config.Testing} {
		if value != "" {
			inputs = append(inputs, value)
		}
	}
	values, err := normalizeDefectVersions(inputs)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	if err := ensureDefectSchema(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	config.Versions = values
	raw, _ := json.Marshal(config)
	_, err = db.ExecContext(ctx, `INSERT INTO defect_project_versions (project_code, versions_json, updated_by, updated_at) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE versions_json=VALUES(versions_json), updated_by=VALUES(updated_by), updated_at=VALUES(updated_at)`, project, string(raw), user.ID, time.Now().Format(time.RFC3339Nano))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, config)
}
