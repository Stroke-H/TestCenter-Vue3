package services

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

func UpdateDefectTestingVersionHandler(c *gin.Context) {
	user, ok := defectUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请登录"})
		return
	}
	project := c.Param("project_code")
	ctx := c.Request.Context()
	allowed, err := defectPermissionAllowed(user, defectPermissionCreate)
	if err != nil || !allowed || !defectProjectAllows(ctx, user, project, "create") {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有该项目提交缺陷的权限"})
		return
	}
	var req struct {
		Testing         string  `json:"testing"`
		ExpectedTesting *string `json:"expected_testing"`
		ExpectedOnline  *string `json:"expected_online"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ExpectedTesting == nil || req.ExpectedOnline == nil {
		c.JSON(400, gin.H{"error": "缺少原版本信息，请刷新"})
		return
	}
	v, e := normalizeDefectVersions([]string{req.Testing})
	if e != nil {
		c.JSON(400, gin.H{"error": e.Error()})
		return
	}
	db, _, e := DatabaseManager.DB(ctx)
	if e != nil {
		c.JSON(500, gin.H{"error": e.Error()})
		return
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		c.JSON(500, gin.H{"error": e.Error()})
		return
	}
	defer tx.Rollback()
	var raw string
	e = tx.QueryRowContext(ctx, "SELECT versions_json FROM defect_project_versions WHERE project_code=? FOR UPDATE", project).Scan(&raw)
	if e != nil {
		c.JSON(409, gin.H{"error": "版本配置不存在或读取失败，请刷新后重试"})
		return
	}
	cfg, e := decodeDefectVersionConfig(raw)
	if e != nil {
		c.JSON(500, gin.H{"error": e.Error()})
		return
	}
	if cfg.Testing != *req.ExpectedTesting || cfg.Online != *req.ExpectedOnline {
		c.JSON(409, gin.H{"error": "版本已被更新，请刷新后重试"})
		return
	}
	values := map[string]any{}
	if !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		if e = json.Unmarshal([]byte(raw), &values); e != nil {
			c.JSON(500, gin.H{"error": e.Error()})
			return
		}
	}
	cfg.Testing = v[0]
	cfg.Versions = []string{}
	if cfg.Online != "" {
		cfg.Versions = append(cfg.Versions, cfg.Online)
	}
	if cfg.Testing != cfg.Online {
		cfg.Versions = append(cfg.Versions, cfg.Testing)
	}
	values["online"] = cfg.Online
	values["testing"] = cfg.Testing
	values["versions"] = cfg.Versions
	data, _ := json.Marshal(values)
	_, e = tx.ExecContext(ctx, "UPDATE defect_project_versions SET versions_json=?,updated_by=?,updated_at=? WHERE project_code=?", string(data), user.ID, time.Now().Format(time.RFC3339Nano), project)
	if e != nil {
		c.JSON(500, gin.H{"error": e.Error()})
		return
	}
	if e = tx.Commit(); e != nil {
		c.JSON(500, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, cfg)
}
