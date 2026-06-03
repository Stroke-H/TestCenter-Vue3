package services

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
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

func TestNormalizeMonkeyWirelessAddressAcceptsPrivateLANAddress(t *testing.T) {
	address, err := normalizeMonkeyWirelessAddress("192.168.1.86:39147")
	if err != nil || address != "192.168.1.86:39147" {
		t.Fatalf("expected private wireless address, got address=%s err=%v", address, err)
	}
}

func TestNormalizeMonkeyWirelessAddressRejectsPublicAddress(t *testing.T) {
	if _, err := normalizeMonkeyWirelessAddress("8.8.8.8:5555"); err == nil {
		t.Fatal("expected public wireless address to be rejected")
	}
}

func TestMonkeyADBDeviceSourceDetectsWirelessAddress(t *testing.T) {
	if source := monkeyADBDeviceSource("192.168.1.86:39147"); source != "wifi" {
		t.Fatalf("expected wifi device source, got %s", source)
	}
	if source := monkeyADBDeviceSource("28191FDH200BE2"); source != "usb" {
		t.Fatalf("expected usb device source, got %s", source)
	}
}

func TestParseMonkeyRecoveryActivityUsesArchivedOriginalComponent(t *testing.T) {
	output := "archiveActivityInfo=ArchiveActivityInfo { title = RapidTV, originalComponentName = ComponentInfo{com.rapid.short.tv/com.drama.rapid.ui.activity.FirstActivity} }"
	activity := parseMonkeyRecoveryActivity(output, "com.rapid.short.tv")
	if activity != "com.rapid.short.tv/com.drama.rapid.ui.activity.FirstActivity" {
		t.Fatalf("expected archived recovery activity, got %s", activity)
	}
}

func TestParseMonkeyRecoveryActivityRejectsOtherPackage(t *testing.T) {
	output := "com.example.other/com.example.other.MainActivity"
	if activity := parseMonkeyRecoveryActivity(output, "com.rapid.short.tv"); activity != "" {
		t.Fatalf("expected unrelated recovery activity to be rejected, got %s", activity)
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

func TestDetectMonkeySystemOverlayFromFocusedNotificationShade(t *testing.T) {
	output := "mCurrentFocus=Window{123 u0 com.android.systemui/com.android.systemui.shade.NotificationShade}"
	if overlay := detectMonkeySystemOverlay(output); overlay != "通知栏" {
		t.Fatalf("expected notification shade overlay, got %s", overlay)
	}
}

func TestStatusBarProtectionCommandsUseAvailableADBCommand(t *testing.T) {
	if err := setMonkeyStatusBarExpansionProtected(context.Background(), "/usr/bin/true", "device-1", true); err != nil {
		t.Fatalf("expected status bar protection command to succeed, got %v", err)
	}
	if err := collapseMonkeyStatusBar(context.Background(), "/usr/bin/true", "device-1"); err != nil {
		t.Fatalf("expected status bar collapse command to succeed, got %v", err)
	}
}

func TestParseMonkeyAdInspectionFindsCloseButton(t *testing.T) {
	output := `UI hierarchy dumped to: /dev/tty
<hierarchy rotation="0">
  <node text="Advertisement" resource-id="com.example:id/interstitial_container" class="android.view.View" content-desc="" bounds="[0,0][1080,2400]" clickable="false">
    <node text="" resource-id="com.example:id/ad_close" class="android.widget.ImageButton" content-desc="关闭广告" bounds="[960,80][1040,160]" clickable="true"></node>
  </node>
</hierarchy>`
	inspection, err := parseMonkeyAdInspection(output)
	if err != nil {
		t.Fatalf("parse ad hierarchy: %v", err)
	}
	if !inspection.Detected || !inspection.HasCloseTarget || inspection.CloseTargetX != 1000 || inspection.CloseTargetY != 120 {
		t.Fatalf("unexpected ad inspection: %+v", inspection)
	}
}

func TestParseMonkeyAdInspectionRejectsNormalCloseButton(t *testing.T) {
	output := `<hierarchy rotation="0"><node text="关闭" resource-id="com.example:id/close" class="android.widget.Button" content-desc="" bounds="[10,20][110,120]" clickable="true"></node></hierarchy>`
	inspection, err := parseMonkeyAdInspection(output)
	if err != nil {
		t.Fatalf("parse normal hierarchy: %v", err)
	}
	if inspection.Detected {
		t.Fatalf("expected normal close button to be ignored, got %+v", inspection)
	}
}

func TestIsMonkeyAdActivity(t *testing.T) {
	if !isMonkeyAdActivity("com.google.android.gms.ads.AdActivity/.MainActivity") {
		t.Fatal("expected ad activity to be detected")
	}
	if isMonkeyAdActivity("com.example.app/.ui.SettingsActivity") {
		t.Fatal("did not expect normal app activity to be detected as ad")
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

func TestSummarizeMonkeyEvidenceMessageExplainsSystemStorageFailure(t *testing.T) {
	summary := summarizeMonkeyEvidenceMessage(
		"03-11 03:51:48.042 W/AconfigPackage(22925): failed to map some package from com.android.btservices.package.map: android.os.flagging.AconfigStorageReadException: ERROR_CANNOT_READ_STORAGE_FILE: Fail to mmap storage",
	)
	if !strings.Contains(summary, "Android 系统配置映射读取失败") {
		t.Fatalf("expected readable system storage explanation, got %s", summary)
	}
}

func TestBuildEvidenceSummaryKeepsRawLogOutOfReadableSummary(t *testing.T) {
	summary := buildEvidenceSummary("warning", []MonkeyRiskEvidence{{
		Level: "warning", Source: "logcat", Message: "03-11 E/Unknown: exception while doing something",
	}})
	if strings.Contains(summary, "03-11") {
		t.Fatalf("expected readable summary instead of raw log, got %s", summary)
	}
}

func TestLoadMonkeyEventsRebuildsReadableHistoricalSummary(t *testing.T) {
	runID := "MONKEY-TEST-READABLE-SUMMARY"
	path := filepath.Join(monkeyRunDir(runID), "events.json")
	t.Cleanup(func() { _ = os.RemoveAll(monkeyRunDir(runID)) })
	if err := writeJSON(path, []MonkeyGraphEvent{{
		ID: "screen-0001", Risk: "warning", Summary: "03-11 raw legacy log",
		Evidence: []MonkeyRiskEvidence{{
			Level: "warning", Source: "logcat",
			Message: "03-11 W/AconfigPackage: android.os.flagging.AconfigStorageReadException: ERROR_CANNOT_READ_STORAGE_FILE: Fail to mmap storage",
		}},
	}}); err != nil {
		t.Fatalf("write historical events: %v", err)
	}
	events, err := loadMonkeyEvents(runID)
	if err != nil {
		t.Fatalf("load historical events: %v", err)
	}
	if len(events) != 1 || strings.Contains(events[0].Summary, "03-11") {
		t.Fatalf("expected readable historical summary, got %+v", events)
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
	appSwitchIndex := slices.Index(args, "--pct-appswitch")
	if appSwitchIndex < 0 || appSwitchIndex+1 >= len(args) || args[appSwitchIndex+1] != "0" {
		t.Fatalf("expected app switching to stay disabled during target app testing, got %v", args)
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

func TestShouldContinueMonkeyBatchOnlyForEarlyCleanExit(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	if !shouldContinueMonkeyBatch(nil, nil, time.Now(), deadline, true) {
		t.Fatal("expected clean early exit to continue with another batch")
	}
	if shouldContinueMonkeyBatch(context.Canceled, nil, time.Now(), deadline, true) {
		t.Fatal("did not expect failed batch to continue")
	}
	if shouldContinueMonkeyBatch(nil, context.Canceled, time.Now(), deadline, true) {
		t.Fatal("did not expect canceled run to continue")
	}
	if shouldContinueMonkeyBatch(nil, nil, deadline, deadline, true) {
		t.Fatal("did not expect run at deadline to continue")
	}
	if shouldContinueMonkeyBatch(nil, nil, time.Now(), deadline, false) {
		t.Fatal("did not expect critical evidence to continue when crash continuation is disabled")
	}
}

func TestMonkeyBatchRequestChangesSeedForFollowUpBatch(t *testing.T) {
	req := MonkeyRunRequest{Seed: "20260526"}
	if seed := monkeyBatchRequest(req, 1).Seed; seed != "20260526" {
		t.Fatalf("expected first batch seed to remain stable, got %s", seed)
	}
	if seed := monkeyBatchRequest(req, 3).Seed; seed != "20260528" {
		t.Fatalf("expected follow-up seed offset, got %s", seed)
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
