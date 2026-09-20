package services

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefectFeishuPriorityUsesSeverity(t *testing.T) {
	tests := map[int]string{1: "option_1", 2: "option_2", 3: "option_3", 4: "option_3"}
	for severity, expected := range tests {
		if actual := defectFeishuPriority(severity); actual != expected {
			t.Fatalf("severity S%d: expected %s, got %s", severity, expected, actual)
		}
	}
}

func TestDefectFeishuPlatformMapping(t *testing.T) {
	tests := map[string]string{
		"Android": "bhbxi3k4r", "iOS": "v_j6_91mp", "TTmins": "trckpxtp2",
		"TikTok Minis": "trckpxtp2", "Web/H5": "ww3m1nibc", "运营后台": "sm8n44had",
	}
	for input, expected := range tests {
		if actual := defectFeishuPlatform(input); actual != expected {
			t.Fatalf("platform %s: expected %s, got %s", input, expected, actual)
		}
	}
}

func TestDefectFeishuBusinessMapping(t *testing.T) {
	tests := []struct {
		name     string
		defect   Defect
		expected string
	}{
		{
			name:     "TTmins takes precedence over ShortsWave in the project name",
			defect:   Defect{ProjectCode: "A1138", ProjectName: "ShortsWave - TTmins"},
			expected: defectFeishuOverseasConfig.BusinessTT,
		},
		{
			name:     "TTmins can be identified from the selected platform",
			defect:   Defect{ProjectCode: "A1160", ProjectName: "MeloShorts", Environment: DefectEnvironment{Platform: "TTmins"}},
			expected: defectFeishuOverseasConfig.BusinessTT,
		},
		{
			name:     "ShortsWave keeps its own business line",
			defect:   Defect{ProjectCode: "A1106", ProjectName: "ShortsWave - iOS"},
			expected: defectFeishuOverseasConfig.BusinessShortsWaveOther,
		},
		{
			name:     "unrelated products are not misclassified",
			defect:   Defect{ProjectCode: "A1096", ProjectName: "MeloShorts - Android"},
			expected: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := defectFeishuBusiness(test.defect, defectFeishuOverseasConfig); actual != test.expected {
				t.Fatalf("expected leaf option %q, got %q", test.expected, actual)
			}
		})
	}
}

func TestDefectFeishuBusinessUsesLeafOptionID(t *testing.T) {
	shortsWave := defectFeishuBusiness(Defect{ProjectName: "ShortsWave - Android"}, defectFeishuOverseasConfig)
	if shortsWave != defectFeishuBusinessSWOther {
		t.Fatalf("ShortsWave must use its generic leaf option, got %s", shortsWave)
	}
	ttmins := defectFeishuBusiness(Defect{ProjectName: "ShortsWave - TTmins"}, defectFeishuOverseasConfig)
	if ttmins != defectFeishuBusinessTT {
		t.Fatalf("TTMins must use its leaf option ID, got %s", ttmins)
	}
}

func TestDefectFeishuNamePrefix(t *testing.T) {
	tests := []struct {
		name     string
		defect   Defect
		expected string
	}{
		{
			name:     "TTmins uses project code and defect sequence",
			defect:   Defect{DefectNo: "BUG-20260917-0001", ProjectCode: "A1181", ProjectName: "Dramix - TTmins"},
			expected: "[A1181-20260917-0001]",
		},
		{
			name:     "ShortsWave keeps the platform defect number",
			defect:   Defect{DefectNo: "BUG-20260917-0002", ProjectCode: "A1106", ProjectName: "ShortsWave - iOS"},
			expected: "[BUG-20260917-0002]",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := defectFeishuNamePrefix(test.defect); actual != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}

func TestDefectFeishuFieldsExcludeDeferredData(t *testing.T) {
	defect := Defect{
		DefectNo: "BUG-20260917-0001", Title: "同步测试", FoundVersion: "v1.2.3",
		ProjectCode: "A1181", ProjectName: "Dramix - TTmins", Severity: 3, Priority: 1, Status: DefectStatusNew,
		Environment: DefectEnvironment{Platform: "TTmins"},
	}
	fields := defectFeishuFields(defect, defectFeishuOverseasConfig)
	keys := map[string]bool{}
	for _, field := range fields {
		keys[field["field_key"]] = true
	}
	for _, expected := range []string{"name", "template", "priority", "description", "business", "field_f7ef16"} {
		if !keys[expected] {
			t.Fatalf("missing expected field %s", expected)
		}
	}
	if strings.Contains(fields[3]["field_value"], "附件") {
		t.Fatal("description should not claim attachments were synchronized")
	}
	if fields[0]["field_value"] != "[A1181-20260917-0001] 同步测试" {
		t.Fatalf("unexpected TTmins work item name: %s", fields[0]["field_value"])
	}
}

func TestDefectFeishuSpaceByWorkspace(t *testing.T) {
	tests := []struct {
		workspace  string
		projectKey string
		templateID string
	}{
		{workspace: " 海外短剧 ", projectKey: defectFeishuProjectKey, templateID: defectFeishuTemplateID},
		{workspace: "番茄短剧", projectKey: defectFeishuTomatoProjectKey, templateID: defectFeishuTomatoTemplateID},
	}
	for _, test := range tests {
		space, ok := defectFeishuSpaceByWorkspace(test.workspace)
		if !ok {
			t.Fatalf("workspace %q should support Feishu defect sync", test.workspace)
		}
		if space.ProjectKey != test.projectKey || space.TemplateID != test.templateID {
			t.Fatalf("workspace %q routed to project=%q template=%q", test.workspace, space.ProjectKey, space.TemplateID)
		}
	}
	if _, ok := defectFeishuSpaceByWorkspace("其他空间"); ok {
		t.Fatal("unsupported workspace must not enable Feishu defect sync")
	}
}

func TestDefectFeishuTomatoFieldsUseTomatoOptions(t *testing.T) {
	ttmins := Defect{
		DefectNo: "BUG-20260918-0001", Title: "TTmins 同步测试",
		ProjectCode: "A1181", ProjectName: "Dramix - TTmins", Severity: 3,
		Environment: DefectEnvironment{Platform: "TTmins"},
	}
	fields := defectFeishuFields(ttmins, defectFeishuTomatoConfig)
	values := map[string]string{}
	for _, field := range fields {
		values[field["field_key"]] = field["field_value"]
	}
	if values["template"] != defectFeishuTomatoTemplateID {
		t.Fatalf("expected tomato template %s, got %s", defectFeishuTomatoTemplateID, values["template"])
	}
	if values["business"] != defectFeishuTomatoBusinessTT {
		t.Fatalf("expected tomato TTmins business %s, got %s", defectFeishuTomatoBusinessTT, values["business"])
	}

	other := defectFeishuBusiness(Defect{ProjectCode: "A1200", ProjectName: "Mangogo - Android"}, defectFeishuTomatoConfig)
	if other != defectFeishuTomatoBusinessOther {
		t.Fatalf("expected tomato fallback business %s, got %s", defectFeishuTomatoBusinessOther, other)
	}
}

func TestExtractDefectFeishuWorkItem(t *testing.T) {
	value := `{"work_item":{"work_item_id":7109000001,"url":"https://project.feishu.cn/shortwave/bug/detail/7109000001"}}`
	id, url := extractDefectFeishuWorkItem(value)
	if id != "7109000001" || !strings.HasSuffix(url, "/7109000001") {
		t.Fatalf("unexpected extraction: id=%s url=%s", id, url)
	}
}

func TestExactDefectFeishuUserKey(t *testing.T) {
	result := `[
		{"user_key":"7250047382782607364","username":"7250047382782607364","name_cn":"王浩","name_en":"jeft","email":"jeft.wang@webeye.cn"},
		{"user_key":"7000000000000000001","name_cn":"王浩然","name_en":"wang haoran"}
	]`
	userKey, err := exactDefectFeishuUserKey(result, " 王浩 ")
	if err != nil {
		t.Fatal(err)
	}
	if userKey != "7250047382782607364" {
		t.Fatalf("expected exact Wang Hao user key, got %s", userKey)
	}
}

func TestExactDefectFeishuUserKeyRejectsMissingOrAmbiguousUser(t *testing.T) {
	if _, err := exactDefectFeishuUserKey(`[{"user_key":"1","name_cn":"王浩然"}]`, "王浩"); err == nil {
		t.Fatal("fuzzy-only user result must be rejected")
	}
	ambiguous := `[{"user_key":"1","name_cn":"王浩"},{"user_key":"2","name_cn":"王浩"}]`
	if _, err := exactDefectFeishuUserKey(ambiguous, "王浩"); err == nil {
		t.Fatal("ambiguous exact-name users must be rejected")
	}
}

func TestFindDefectFeishuMQLField(t *testing.T) {
	var decoded any
	if err := json.Unmarshal([]byte(`{"data":{"1":[{"moql_field_list":[{"key":"work_item_id","value":{"long_value":7108349064}}]}]}}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if actual := findDefectFeishuMQLField(decoded, "work_item_id"); actual != "7108349064" {
		t.Fatalf("unexpected work item id %s", actual)
	}
}

func TestChooseDefectFeishuTransition(t *testing.T) {
	state := defectFeishuTransitionState{
		StateName: "OPEN",
		Transition: []defectFeishuTransition{
			{ID: float64(1), StateName: "IN PROGRESS"},
			{ID: float64(2), StateName: "RESOLVED"},
		},
	}
	if transition := chooseDefectFeishuTransition(state, "RESOLVED"); transition == nil || transition.ID != float64(2) {
		t.Fatal("should choose direct RESOLVED transition")
	}
	if transition := chooseDefectFeishuTransition(state, "CLOSED"); transition == nil || transition.ID != float64(2) {
		t.Fatal("should choose RESOLVED as the intermediate transition before CLOSED")
	}
}
