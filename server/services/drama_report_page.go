package services

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:embed assets/drama-report.html
var dramaReportPageSource string

var dramaReportPageTemplate = template.Must(template.New("drama-report").Parse(dramaReportPageSource))

func GetDramaErrorsPageHandler(c *gin.Context) {
	category := c.DefaultQuery("category", "rules")
	serveDramaReportPage(c, projectRootDir(), category, "检测异常明细")
}

func serveDramaReportPage(c *gin.Context, root, category, heading string) {
	runID := c.Param("runId")
	if !languageRunIDPattern.MatchString(runID) {
		c.String(http.StatusBadRequest, "报告编号无效")
		return
	}
	if !validDramaReportCategory(category) {
		c.String(http.StatusBadRequest, "筛选条件无效")
		return
	}
	b, err := os.ReadFile(filepath.Join(testRunStorageRoot(root), runID, "metadata.json"))
	if err != nil {
		c.String(http.StatusNotFound, "报告不存在或已删除")
		return
	}
	var archive TestRunArchive
	if err := json.Unmarshal(b, &archive); err != nil {
		c.String(http.StatusInternalServerError, "报告元数据读取失败")
		return
	}
	if archive.TestType != "drama" {
		c.String(http.StatusNotFound, "不是剧集播放测试报告")
		return
	}
	var output bytes.Buffer
	if err := dramaReportPageTemplate.Execute(&output, struct{ RunID, Category, Heading string }{runID, category, heading}); err != nil {
		c.String(http.StatusInternalServerError, "加载报告页面失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	// Like the original report route, allow the platform's frontend (which can
	// use a different port) to embed this backend page in its report iframe.
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; form-action 'none'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", output.Bytes())
}
