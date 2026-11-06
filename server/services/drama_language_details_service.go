package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

func splitDramaLanguageFailures(failures []dramaFailureSummary) (languages, remaining []dramaFailureSummary) {
	for _, failure := range failures {
		language, other := failure, failure
		language.Errors, other.Errors = nil, nil
		for _, line := range failure.Errors {
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "•")), "[简介语种]") {
				language.Errors = append(language.Errors, line)
			} else {
				other.Errors = append(other.Errors, line)
			}
		}
		if len(language.Errors) > 0 {
			languages = append(languages, language)
		}
		if len(other.Errors) > 0 {
			remaining = append(remaining, other)
		}
	}
	return
}

func compactDramaLanguageFailure(failure dramaFailureSummary) dramaFailureSummary {
	result := failure
	result.Errors = make([]string, 0, len(failure.Errors))
	for _, line := range failure.Errors {
		if index := strings.Index(line, "。description="); index >= 0 {
			line = line[:index] + "。"
		}
		result.Errors = append(result.Errors, line)
	}
	return result
}

func dramaLanguageDetailsURL(reportURL string) string {
	parsed, err := url.Parse(reportURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 5 || parts[0] != "api" || parts[1] != "test-runs" || parts[3] != "artifacts" || parts[4] != "report" {
		return ""
	}
	parsed.Path = "/api/test-runs/" + parts[2] + "/language-errors"
	parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", ""
	return parsed.String()
}

type dramaLanguageDetail struct {
	DramaID     string
	IntID       string
	Title       string
	CNTitle     string
	Expected    string
	Detected    string
	Description string
	Conclusion  string
	DramaURL    string
}

var languageExpectedPattern = regexp.MustCompile(`lang=([a-zA-Z][a-zA-Z_-]*)`)
var languageDetectedPattern = regexp.MustCompile(`识别语种=([a-zA-Z][a-zA-Z_-]*)`)
var languageRunIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,199}$`)

func languageDetailRows(failures []dramaFailureSummary) []dramaLanguageDetail {
	languages, _ := splitDramaLanguageFailures(failures)
	rows := make([]dramaLanguageDetail, 0, len(languages))
	for _, failure := range languages {
		for _, line := range failure.Errors {
			row := dramaLanguageDetail{DramaID: failure.DramaID, IntID: failure.IntID, Title: failure.Title, CNTitle: failure.CNTitle, DramaURL: buildDramaManagementURL(failure.DramaID)}
			if match := languageExpectedPattern.FindStringSubmatch(line); len(match) > 1 {
				row.Expected = match[1]
			}
			if match := languageDetectedPattern.FindStringSubmatch(line); len(match) > 1 {
				row.Detected = match[1]
			}
			compact := compactDramaLanguageFailure(dramaFailureSummary{Errors: []string{line}})
			row.Conclusion = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(compact.Errors[0]), "•"))
			if index := strings.Index(line, "。description="); index >= 0 {
				row.Description = line[index+len("。description="):]
				if end := strings.LastIndex(row.Description, "；内容指纹="); end >= 0 {
					row.Description = row.Description[:end]
				}
			}
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].IntID != rows[j].IntID {
			return rows[i].IntID < rows[j].IntID
		}
		if rows[i].DramaID != rows[j].DramaID {
			return rows[i].DramaID < rows[j].DramaID
		}
		return rows[i].Conclusion < rows[j].Conclusion
	})
	return rows
}

// Explicitly public, read-only and scoped to one immutable report archive.
// No account, OAuth credentials or live drama_info.json are exposed here.
func GetDramaLanguageDetailsHandler(c *gin.Context) {
	serveDramaLanguageDetails(c, projectRootDir())
}

func serveDramaLanguageDetails(c *gin.Context, root string) {
	if c.Query("view") != "original" {
		serveDramaReportPage(c, root, "language", "语种异常明细")
		return
	}
	runID := c.Param("runId")
	if !languageRunIDPattern.MatchString(runID) {
		c.String(http.StatusBadRequest, "报告编号无效")
		return
	}
	runDir := filepath.Join(testRunStorageRoot(root), runID)
	content, err := os.ReadFile(filepath.Join(runDir, "metadata.json"))
	if err != nil {
		c.String(http.StatusNotFound, "报告不存在或已删除")
		return
	}
	var archive TestRunArchive
	if json.Unmarshal(content, &archive) != nil || archive.TestType != "drama" {
		c.String(http.StatusNotFound, "无效的剧集报告")
		return
	}
	rows := languageDetailRows(loadArchivedDramaFailures(runDir, ""))
	view := struct {
		RunID, StartedAt, Duration string
		Count                      int
		Rows                       []dramaLanguageDetail
	}{runID, archive.StartedAt, archive.Duration, len(rows), rows}
	var rendered bytes.Buffer
	if err := dramaLanguageDetailsTemplate.Execute(&rendered, view); err != nil {
		c.String(http.StatusInternalServerError, "生成明细失败")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'self'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", rendered.Bytes())
}

var dramaLanguageDetailsTemplate = template.Must(template.New("language-details").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>语种异常明细 · TestCenter</title>
<style>*{box-sizing:border-box}body{margin:0;background:#f4f7fb;color:#243247;font:14px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}main{max-width:1100px;margin:40px auto;padding:0 22px}header{margin-bottom:24px}.eyebrow{font-size:11px;letter-spacing:2px;color:#5276bf;font-weight:700}h1{font-size:28px;margin:6px 0}header p{color:#748197;margin:4px 0}.toolbar{display:flex;align-items:center;justify-content:space-between;gap:16px;margin:20px 0}.total{color:#355c9e;font-weight:600}input{width:min(430px,65%);border:1px solid #dce4f0;border-radius:10px;padding:11px 14px;font:inherit;background:#fff;outline:none}input:focus{border-color:#6696eb}.card{background:#fff;border:1px solid #e0e7f1;border-radius:14px;padding:22px;margin:14px 0;box-shadow:0 3px 14px #23375305}.row{display:flex;align-items:flex-start;justify-content:space-between;gap:16px;flex-wrap:wrap}h2{font-size:17px;margin:0;overflow-wrap:anywhere}.meta{color:#7d889a;font-size:12px;overflow-wrap:anywhere}.cn{margin:5px 0 12px;color:#64748b}.badge{display:inline-block;border-radius:7px;padding:4px 10px;font-size:12px;font-weight:600;background:#f1f5f9;color:#475569}.badge.actual{background:#fff1ed;color:#c35336}.conclusion{padding:10px 12px;background:#fff7ed;border-radius:9px;color:#9a571c;overflow-wrap:anywhere}.description{white-space:pre-wrap;overflow-wrap:anywhere;padding:16px;border:1px solid #e6ebf2;background:#fafbfd;border-radius:10px;margin:8px 0 0;line-height:1.9}.label{margin-top:16px;font-size:12px;color:#7d889a;font-weight:600}a{color:#356bca;text-decoration:none;font-size:12px}a:hover{text-decoration:underline}.empty{text-align:center;padding:55px 12px;color:#8290a3}.foot{font-size:12px;color:#8a95a5;margin:22px 0}.hidden{display:none!important}@media(max-width:600px){main{margin:22px auto;padding:0 14px}.card{padding:16px}h1{font-size:23px}.toolbar{align-items:flex-start;flex-direction:column}input{width:100%}}</style></head>
<body><main><header><div class="eyebrow">TESTCENTER / LANGUAGE CHECK</div><h1>语种异常明细</h1><p>{{.StartedAt}} · 测试耗时 {{.Duration}}</p><p class="meta">报告：{{.RunID}}</p></header><div class="toolbar"><span class="total">共 {{.Count}} 条语种异常</span><input id="search" type="search" placeholder="搜索剧名、编号、语种或简介" aria-label="搜索异常"></div>
{{range .Rows}}<article class="card"><div class="row"><div><h2>{{.IntID}} · {{.Title}}</h2><div class="meta">{{.DramaID}}</div></div>{{if .DramaURL}}<a href="{{.DramaURL}}" target="_blank" rel="noopener noreferrer">查看剧集信息 ↗</a>{{end}}</div><p class="cn">{{.CNTitle}}</p><div class="row"><span><span class="badge">配置 lang：{{if .Expected}}{{.Expected}}{{else}}空或未知{{end}}</span> <span class="badge actual">识别语种：{{if .Detected}}{{.Detected}}{{else}}未确定{{end}}</span></span></div><p class="conclusion">{{.Conclusion}}</p><div class="label">完整简介 description</div><div class="description">{{if .Description}}{{.Description}}{{else}}该异常未记录简介内容{{end}}</div></article>{{end}}
<div id="empty" class="empty{{if .Count}} hidden{{end}}">没有匹配的语种异常</div><p class="foot">本页为对应检测报告的只读快照。中文按语种比较，不区分简繁体。</p></main>
<script>const input=document.getElementById('search'),cards=[...document.querySelectorAll('.card')];input.addEventListener('input',()=>{const q=input.value.trim().toLowerCase();let count=0;cards.forEach(card=>{const show=card.textContent.toLowerCase().includes(q);card.classList.toggle('hidden',!show);if(show)count++});document.getElementById('empty').classList.toggle('hidden',count>0)});</script></body></html>`))

// Budget the actual JSON message envelope, not just visible characters.
func scheduledCardEnvelopeSize(card map[string]any) int {
	content, _ := json.Marshal(card)
	body, _ := json.Marshal(map[string]string{"receive_id": strings.Repeat("x", 128), "msg_type": "interactive", "content": string(content)})
	return len(body)
}

func splitScheduledFeishuCards(card map[string]any) []map[string]any {
	const budget = 24 * 1024
	elements, ok := card["elements"].([]map[string]any)
	if !ok {
		return []map[string]any{card}
	}
	if scheduledCardEnvelopeSize(card) <= budget && len(elements) <= 70 {
		return []map[string]any{card}
	}
	elements = splitScheduledCardTextElements(card, elements, budget-2048)
	var pages []map[string]any
	newPage := func() map[string]any {
		result := map[string]any{}
		for k, v := range card {
			result[k] = v
		}
		result["elements"] = []map[string]any{}
		return result
	}
	page := newPage()
	var sectionHeading map[string]any
	for _, element := range elements {
		isSectionHeading := false
		if text, ok := element["text"].(map[string]any); ok {
			content, _ := text["content"].(string)
			for _, prefix := range []string{"**异常剧集明细（", "**分组异常（", "**分类异常（", "**语种异常（", "**免费集数异常（", "**版权异常（", "**其他异常（"} {
				if strings.HasPrefix(content, prefix) {
					sectionHeading = element
					isSectionHeading = true
					break
				}
			}
		}
		current := page["elements"].([]map[string]any)
		page["elements"] = append(append([]map[string]any{}, current...), element)
		if len(current) > 0 && (scheduledCardEnvelopeSize(page) > budget || len(current) >= 70) {
			page["elements"] = current
			pages = append(pages, page)
			page = newPage()
			page["elements"] = []map[string]any{element}
			if sectionHeading != nil && !isSectionHeading && element["tag"] != "action" && element["tag"] != "hr" {
				page["elements"] = []map[string]any{sectionHeading, element}
			}
		}
	}
	if len(page["elements"].([]map[string]any)) > 0 {
		pages = append(pages, page)
	}
	for i, p := range pages {
		old, _ := card["header"].(map[string]any)
		header := map[string]any{}
		for k, v := range old {
			header[k] = v
		}
		baseTitle := "检测报告"
		if title, ok := old["title"].(map[string]any); ok {
			if value, ok := title["content"].(string); ok && value != "" {
				baseTitle = value
			}
		}
		header["title"] = map[string]any{"tag": "plain_text", "content": fmt.Sprintf("%s（%d/%d）", baseTitle, i+1, len(pages))}
		p["header"] = header
	}
	return pages
}
