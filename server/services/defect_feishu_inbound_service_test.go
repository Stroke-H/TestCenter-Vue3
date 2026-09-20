package services

import (
	"testing"

	"testcenter-server/models"
)

func TestInferDefectFeishuProjectUsesExplicitProjectCode(t *testing.T) {
	item := defectFeishuRemoteItem{
		ProjectKey:  defectFeishuTomatoProjectKey,
		Name:        "飞书原生标题",
		Description: "所属项目：A1147 · Mangogo - Android",
	}
	projects := []models.Project{{ProjectCode: "A1147", ProjectName: "Mangogo - Android", Workspace: defectFeishuTomatoWorkspace}}
	if got := inferDefectFeishuProject(item, projects); got != "A1147" {
		t.Fatalf("expected A1147, got %q", got)
	}
}

func TestInferDefectFeishuProjectDoesNotGuessAmbiguousTTminsProject(t *testing.T) {
	item := defectFeishuRemoteItem{ProjectKey: defectFeishuProjectKey, Business: "TTMins", Platform: "TT Minis", Name: "没有项目编号"}
	projects := []models.Project{
		{ProjectCode: "A1181", ProjectName: "Dramix - TTmins", Workspace: defectFeishuWorkspace},
		{ProjectCode: "A1182", ProjectName: "Other - TTmins", Workspace: defectFeishuWorkspace},
	}
	if got := inferDefectFeishuProject(item, projects); got != "" {
		t.Fatalf("ambiguous native record must remain unresolved, got %q", got)
	}
}

func TestDecodeDefectFeishuMQLItems(t *testing.T) {
	payload := `{"list":[{"group_infos":[{"group_id":"1"}],"count":1}],"session_id":"session","data":{"1":[{"moql_field_list":[{"key":"work_item_id","value":{"long_value":7118287811}},{"key":"name","value":{"string_value":"原生缺陷"}},{"key":"priority","value":{"key_label_value":{"key":"option_3","label":"P2"}}},{"key":"work_item_status","value":{"key_label_value_list":[{"key":"Not started","label":"OPEN"}]}},{"key":"owner","value":{"user_value":{"email":"tester@example.com","name_cn":"测试员"}}}] }]}}`
	items, session, group, total, err := decodeDefectFeishuMQLItems(payload, defectFeishuTomatoConfig)
	if err != nil {
		t.Fatal(err)
	}
	if session != "session" || group != "1" || total != 1 || len(items) != 1 {
		t.Fatalf("unexpected page metadata: session=%q group=%q total=%d items=%d", session, group, total, len(items))
	}
	if items[0].WorkItemID != "7118287811" || items[0].Name != "原生缺陷" || items[0].OwnerName != "测试员" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}
