package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"testcenter-server/services"
)

func main() {
	projectCode := "1106"
	if len(os.Args) > 1 {
		projectCode = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	count, err := services.AnalyzeProjectReportsForConfig(ctx, projectCode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	record, err := services.GetProjectConfigRecord(projectCode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output, _ := json.MarshalIndent(map[string]any{
		"project_code":   record.ProjectCode,
		"analyzed_count": count,
		"items":          record.Items,
	}, "", "  ")
	fmt.Println(string(output))
}
