package services

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"
)

func TestMonkeyLogClassifierFiltersSystemNoise(t *testing.T) {
	classifier := monkeyLogClassifier{targetPackage: "com.example.app"}

	_, ok, ignored := classifier.classify(
		"03-11 02:29:02.267 E/FirebaseCrashlytics(32258): javax.net.ssl.SSLHandshakeException: connection closed",
		"logcat",
	)
	if ok || !ignored {
		t.Fatalf("expected unrelated system log to be ignored, got ok=%v ignored=%v", ok, ignored)
	}
}

func TestMonkeyLogClassifierKeepsTargetAppCrash(t *testing.T) {
	classifier := monkeyLogClassifier{targetPackage: "com.example.app"}

	_, ok, ignored := classifier.classify(
		"03-11 02:29:04.317 E/com.example.app.ui.AppLog(32258): java.util.NoSuchElementException: List is empty.",
		"logcat",
	)
	if !ok || ignored {
		t.Fatalf("expected package log to seed target pid, got ok=%v ignored=%v", ok, ignored)
	}

	evidence, ok, ignored := classifier.classify(
		"03-11 02:29:07.405 E/AndroidRuntime(32258): FATAL EXCEPTION: main",
		"logcat",
	)
	if !ok || ignored || evidence.Level != "critical" {
		t.Fatalf("expected target app fatal exception, got evidence=%+v ok=%v ignored=%v", evidence, ok, ignored)
	}
}

func TestParseCurrentActivitySupportsModernAndroidOutput(t *testing.T) {
	output := "mResumedActivity: ActivityRecord{123 u0 com.example.app/.ui.MainActivity t42}"
	if activity := parseCurrentActivity(output); activity != "com.example.app/.ui.MainActivity" {
		t.Fatalf("unexpected activity: %s", activity)
	}
}

func TestForegroundPackageFromActivity(t *testing.T) {
	if packageName := foregroundPackageFromActivity("com.example.app/.ui.MainActivity"); packageName != "com.example.app" {
		t.Fatalf("unexpected foreground package: %s", packageName)
	}
	if packageName := foregroundPackageFromActivity("UnknownActivity"); packageName != "" {
		t.Fatalf("expected unknown activity to be ignored, got %s", packageName)
	}
}

func TestShouldForceStopForegroundPackageProtectsSystemApps(t *testing.T) {
	for _, packageName := range []string{"android", "com.android.systemui", "com.google.android.permissioncontroller"} {
		if shouldForceStopForegroundPackage(packageName, "com.example.app") {
			t.Fatalf("expected system package %s to be protected", packageName)
		}
	}
	if !shouldForceStopForegroundPackage("com.example.other", "com.example.app") {
		t.Fatal("expected unrelated third-party package to be force stopped")
	}
}

func TestValidateMonkeyLaunchOutputRejectsAbortedLaunch(t *testing.T) {
	if err := validateMonkeyLaunchOutput("Events injected: 1"); err != nil {
		t.Fatalf("expected successful launcher output, got %v", err)
	}
	if err := validateMonkeyLaunchOutput("** No activities found to run, monkey aborted."); err == nil {
		t.Fatal("expected aborted launcher output to fail")
	}
}

func TestStopADBMonkeyProcessUsesAvailableCommand(t *testing.T) {
	if err := stopADBMonkeyProcess(context.Background(), "/usr/bin/true", "device-1"); err != nil {
		t.Fatalf("expected successful stop command, got %v", err)
	}
}

func TestFinalCaptureContextSurvivesCanceledRun(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if finalCtx := finalCaptureContext(runCtx); finalCtx.Err() != nil {
		t.Fatalf("expected final capture context to remain usable, got %v", finalCtx.Err())
	}
}

func TestMonkeyExecutionReportStatusPreservesStoppedRuns(t *testing.T) {
	if status := monkeyExecutionReportStatus("stopped"); status != "Stopped" {
		t.Fatalf("expected stopped report status, got %s", status)
	}
	if status := normalizeMonkeyExecutionReportStatus(ExecutionReport{
		Type: "Monkey 测试", Status: "Failed", AnalysisResult: "结束原因: stopped_or_duration_reached",
	}); status != "Stopped" {
		t.Fatalf("expected historical stopped report normalization, got %s", status)
	}
	if status := normalizeMonkeyExecutionReportStatus(ExecutionReport{
		Type: "Monkey 测试", Status: "Failed", AnalysisResult: "结束原因: target_app_crash",
	}); status != "Failed" {
		t.Fatalf("expected crash report to stay failed, got %s", status)
	}
}

func TestBuildMonkeyArgsAddsIgnoreCrashesOnlyWhenEnabled(t *testing.T) {
	args := buildMonkeyArgs(MonkeyRunRequest{
		DeviceID:           "device-1",
		PackageName:        "com.example.app",
		ThrottleMs:         300,
		Seed:               "123",
		EventTotal:         100,
		ContinueAfterCrash: true,
	})
	if !slices.Contains(args, "--ignore-crashes") {
		t.Fatal("expected --ignore-crashes when continueAfterCrash is enabled")
	}

	args = buildMonkeyArgs(MonkeyRunRequest{
		DeviceID:    "device-1",
		PackageName: "com.example.app",
		ThrottleMs:  300,
		Seed:        "123",
		EventTotal:  100,
	})
	if slices.Contains(args, "--ignore-crashes") {
		t.Fatal("did not expect --ignore-crashes by default")
	}
}

func TestMonkeyStreamReplaysEventsAfterLastEventID(t *testing.T) {
	run := &monkeyRunState{streamSubscribers: map[chan MonkeyStreamEvent]struct{}{}}
	publishMonkeyStreamEvent(run, "summary", MonkeyRunSummary{RunID: "MONKEY-1", Status: "running"})
	publishMonkeyStreamEvent(run, "graph-event", MonkeyGraphEvent{ID: "screen-0001"})

	subscriber, replay := subscribeMonkeyStream(run, 1)
	defer unsubscribeMonkeyStream(run, subscriber)
	if len(replay) != 1 || replay[0].Type != "graph-event" {
		t.Fatalf("expected graph event replay after id 1, got %+v", replay)
	}
}

func TestWriteMonkeySSEFormatsStandardFrame(t *testing.T) {
	var buffer bytes.Buffer
	writeMonkeySSE(&buffer, MonkeyStreamEvent{
		ID: 7, Type: "summary", Data: MonkeyRunSummary{RunID: "MONKEY-1", Status: "running"},
	})
	frame := buffer.String()
	for _, expected := range []string{"id: 7\n", "event: summary\n", `data: {"runId":"MONKEY-1"`} {
		if !strings.Contains(frame, expected) {
			t.Fatalf("expected SSE frame to contain %q, got %q", expected, frame)
		}
	}
}
