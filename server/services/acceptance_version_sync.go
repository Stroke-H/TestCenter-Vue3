package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var releaseVersionPattern = regexp.MustCompile(`^([vV]?)([0-9]+)\.([0-9]+)\.([0-9]+)(?:\s*[（(]build[^）)]*[）)])?$`)
var acceptanceVersionSyncMu sync.Mutex

type releaseVersion struct {
	prefix  string
	numbers [3]int
	widths  [3]int
}

func parseReleaseVersion(value string) (releaseVersion, error) {
	m := releaseVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	v := releaseVersion{}
	if m == nil {
		return v, fmt.Errorf("无法识别版本号 %q，请手动配置提测版本", value)
	}
	v.prefix = m[1]
	for i := 0; i < 3; i++ {
		n, e := strconv.Atoi(m[i+2])
		if e != nil || n > 999999 {
			return v, fmt.Errorf("版本号数值超出范围")
		}
		v.numbers[i] = n
		v.widths[i] = len(m[i+2])
	}
	return v, nil
}
func reportVersionTime(value string) time.Time {
	loc := acceptanceTodoLocation()
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, e := time.ParseInLocation(layout, value, loc); e == nil {
			return t
		}
	}
	return time.Time{}
}

// Infer the usual changed segment; use the most common recent increments on it.
func NextAcceptanceVersion(project, current string, reports []AcceptanceReport) (string, string, error) {
	base, err := parseReleaseVersion(current)
	if err != nil {
		return "", "", err
	}
	history := []AcceptanceReport{}
	for _, r := range reports {
		if strings.TrimPrefix(r.ProjectCode, "A") == strings.TrimPrefix(project, "A") {
			history = append(history, r)
		}
	}
	sort.SliceStable(history, func(i, j int) bool {
		return reportVersionTime(history[i].CreatedAt).Before(reportVersionTime(history[j].CreatedAt))
	})
	counts := [3]int{}
	steps := [3][]int{}
	var prev *releaseVersion
	lastAxis := 2
	for _, r := range history {
		v, e := parseReleaseVersion(r.Version)
		if e != nil {
			continue
		}
		if prev != nil {
			for i := 0; i < 3; i++ {
				if v.numbers[i] != prev.numbers[i] {
					if v.numbers[i] > prev.numbers[i] {
						counts[i]++
						steps[i] = append(steps[i], v.numbers[i]-prev.numbers[i])
						lastAxis = i
					}
					break
				}
			}
		}
		copy := v
		prev = &copy
	}
	axis := lastAxis
	for i := 0; i < 3; i++ {
		if counts[i] > counts[axis] {
			axis = i
		}
	}
	for _, r := range history {
		if isTTminsProjectText(project, r.ProjectName) && counts[2] == counts[axis] {
			axis = 2
			break
		}
	}
	step := 1
	rule := "默认规则（样本不足）：末段 +1"
	if counts[axis] > 0 {
		step = steps[axis][0]
		for _, n := range steps[axis] {
			a, b := step, n
			for b != 0 {
				a, b = b, a%b
			}
			step = a
		}
		rule = fmt.Sprintf("历史推导：%s +%d", []string{"首段", "中段", "末段"}[axis], step)
	}
	base.numbers[axis] += step
	for i := axis + 1; i < 3; i++ {
		base.numbers[i] = 0
	}
	next := fmt.Sprintf("%s%0*d.%0*d.%0*d", base.prefix, base.widths[0], base.numbers[0], base.widths[1], base.numbers[1], base.widths[2], base.numbers[2])
	return next, rule, nil
}

// Always sync the latest submitted report, so editing an older report cannot roll back a project.
func SyncAcceptanceProjectVersion(project string) error {
	acceptanceVersionSyncMu.Lock()
	defer acceptanceVersionSyncMu.Unlock()
	reports, err := GetAcceptanceReports()
	if err != nil {
		return err
	}
	projects, err := ConfigServiceInstance.GetAllProjects()
	if err != nil {
		return err
	}
	canonical := ""
	for _, p := range projects {
		if p.ProjectCode == project || p.ID == project {
			canonical = p.ProjectCode
			break
		}
	}
	if canonical == "" {
		return fmt.Errorf("未找到项目 %s", project)
	}
	var latest *AcceptanceReport
	for _, r := range reports {
		if strings.TrimPrefix(r.ProjectCode, "A") != strings.TrimPrefix(canonical, "A") {
			continue
		}
		if latest == nil || reportVersionTime(r.CreatedAt).After(reportVersionTime(latest.CreatedAt)) {
			copy := r
			latest = &copy
		}
	}
	if latest == nil {
		return nil
	}
	next, rule, err := NextAcceptanceVersion(canonical, latest.Version, reports)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = ensureDefectSchema(ctx); err != nil {
		return err
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	err = tx.QueryRowContext(ctx, "SELECT versions_json FROM defect_project_versions WHERE project_code=? FOR UPDATE", canonical).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	values := map[string]any{}
	if raw != "" && !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		if err = json.Unmarshal([]byte(raw), &values); err != nil {
			return err
		}
	}
	if values["source_report_id"] == latest.ID && values["online"] == strings.TrimSpace(latest.Version) {
		return nil
	}
	if online, ok := values["online"].(string); ok {
		old, e := parseReleaseVersion(online)
		newV, _ := parseReleaseVersion(latest.Version)
		if e == nil {
			for i := 0; i < 3; i++ {
				if old.numbers[i] > newV.numbers[i] {
					return fmt.Errorf("验收版本低于当前线上版本，未自动回退")
				}
				if old.numbers[i] < newV.numbers[i] {
					break
				}
			}
		}
	}
	values["online"] = strings.TrimSpace(latest.Version)
	values["testing"] = next
	values["versions"] = []string{strings.TrimSpace(latest.Version), next}
	values["rule"] = rule
	values["source_report_id"] = latest.ID
	data, _ := json.Marshal(values)
	_, err = tx.ExecContext(ctx, `INSERT INTO defect_project_versions(project_code,versions_json,updated_by,updated_at) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE versions_json=VALUES(versions_json),updated_by=VALUES(updated_by),updated_at=VALUES(updated_at)`, canonical, string(data), "acceptance-version-sync", time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
