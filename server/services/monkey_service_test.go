package services

import (
	"bytes"
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
