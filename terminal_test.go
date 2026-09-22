package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLineOutputContainsNoCursorControls(t *testing.T) {
	practice, _ := findPractice("coherence")
	var output bytes.Buffer
	lineOutput := LineOutput{Writer: &output}
	lineOutput.Started(practice)
	lineOutput.PhaseStarted(PhaseStart{Phase: practice.Phases[0]})
	lineOutput.Completed(practice, 1)

	if strings.Contains(output.String(), "\x1b[") {
		t.Errorf("non-interactive output contains ANSI cursor controls: %q", output.String())
	}
	if strings.Contains(output.String(), "\a") {
		t.Errorf("non-interactive output contains a bell byte: %q", output.String())
	}
}

func TestTerminalCleanupIsIdempotent(t *testing.T) {
	var output bytes.Buffer
	restoreCalls := 0
	lifecycle := &terminalLifecycle{
		writer: &output,
		restoreRaw: func() error {
			restoreCalls++
			return nil
		},
	}

	if err := lifecycle.hideCursor(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.restore(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.restore(); err != nil {
		t.Fatal(err)
	}

	if restoreCalls != 1 {
		t.Errorf("raw restore calls = %d, want 1", restoreCalls)
	}
	if got := strings.Count(output.String(), ansiShowCursor); got != 1 {
		t.Errorf("show-cursor writes = %d, want 1; output = %q", got, output.String())
	}
	if got := strings.Count(output.String(), ansiReset); got != 1 {
		t.Errorf("style resets = %d, want 1; output = %q", got, output.String())
	}
	if !strings.HasSuffix(output.String(), "\r\n") {
		t.Errorf("cleanup output %q does not end with a clean new line", output.String())
	}
}

func TestWordmarkRequiresEnoughTerminalSpace(t *testing.T) {
	lines := strings.Split(wordmark, "\n")
	if len(lines) != 3 {
		t.Fatalf("wordmark has %d lines, want 3", len(lines))
	}
	width := 0
	for _, line := range lines {
		if lineWidth := textWidth(line); lineWidth > width {
			width = lineWidth
		}
	}
	if !wordmarkFits(width, 3) {
		t.Error("wordmark does not fit at its exact dimensions")
	}
	if wordmarkFits(width-1, 3) {
		t.Error("wordmark fits in a terminal that is too narrow")
	}
	if wordmarkFits(width, 2) {
		t.Error("wordmark fits in a terminal that is too short")
	}
}

func TestSplashDurationIsAboutOneSecond(t *testing.T) {
	if splashTime < 900*time.Millisecond || splashTime > 1100*time.Millisecond {
		t.Errorf("splashTime = %s, want approximately one second", splashTime)
	}
}

func TestPhaseBellOnlyRingsForLiveTransitions(t *testing.T) {
	starts := []PhaseStart{{Phase: Phase{Name: "Exhale"}}}
	delayed := []PhaseStart{{Phase: Phase{Name: "Exhale"}}, {Phase: Phase{Name: "Inhale"}}}
	tests := []struct {
		name    string
		enabled bool
		starts  []PhaseStart
		want    string
	}{
		{name: "startup", enabled: true, starts: nil, want: ""},
		{name: "sound disabled", enabled: false, starts: starts, want: ""},
		{name: "one transition", enabled: true, starts: starts, want: "\a"},
		{name: "delayed catchup", enabled: true, starts: delayed, want: "\a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := emitPhaseBell(&output, test.enabled, test.starts); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != test.want {
				t.Errorf("bell output = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPausedAndResumedSessionsDoNotCreateBellTransitions(t *testing.T) {
	practice, _ := findPractice("calm")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	session.Pause(testStart.Add(time.Second))
	if starts := session.Advance(testStart.Add(time.Hour)); starts != nil {
		t.Errorf("paused Advance() = %v, want nil", starts)
	}
	session.Resume(testStart.Add(2 * time.Hour))
	if starts := session.Advance(session.PhaseStartedAt); starts != nil {
		t.Errorf("resume produced starts = %v, want nil", starts)
	}
}

func TestFrameClearPreservesPriorHistory(t *testing.T) {
	var output bytes.Buffer
	output.WriteString("earlier history\r\n")
	frames := &frameWriter{writer: &output}
	if err := frames.write([]string{"┌─ frame", "│ frame", "└─ frame"}); err != nil {
		t.Fatal(err)
	}
	if err := frames.clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := output.WriteString("breathe · COHERENCE · ended after 00:42"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output.String(), "earlier history\r\n") {
		t.Errorf("clear erased earlier history: %q", output.String())
	}
	if strings.Contains(output.String(), "\x1b[2J") || strings.Contains(output.String(), "\x1b[?1049") {
		t.Errorf("clear used full-screen controls: %q", output.String())
	}
}

func TestSplashExitLeavesZeroDurationSummary(t *testing.T) {
	practice, _ := findPractice("box")
	var output bytes.Buffer
	frames := &frameWriter{writer: &output}
	if err := frames.write(strings.Split(wordmark, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := finishSplashExit(frames, &output, practice); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(output.String(), "breathe · BOX BREATH · ended after 00:00") {
		t.Errorf("splash exit output = %q", output.String())
	}
}

func TestResizeTrackerDebouncesAndIgnoresWidePhysicalChanges(t *testing.T) {
	start := testStart
	resize := newResizeTracker(80)
	if got := resize.observe(120, start); got != resizeRender {
		t.Errorf("wide width action = %v, want ordinary render", got)
	}
	if got := resize.observe(48, start); got != resizeSuspend {
		t.Errorf("first narrow width action = %v, want suspend", got)
	}
	if got := resize.observe(44, start.Add(100*time.Millisecond)); got != resizeSuspend {
		t.Errorf("resize burst action = %v, want suspend", got)
	}
	if got := resize.observe(44, start.Add(299*time.Millisecond)); got != resizeSuspend {
		t.Errorf("unstable width action = %v, want suspend", got)
	}
	if got := resize.observe(44, start.Add(300*time.Millisecond)); got != resizeRecover {
		t.Errorf("stabilized width action = %v, want recovery", got)
	}
	if resize.width != 44 {
		t.Errorf("effective width = %d, want 44", resize.width)
	}
	if got := resize.observe(44, start.Add(350*time.Millisecond)); got != resizeRender {
		t.Errorf("post-recovery action = %v, want ordinary render", got)
	}
}

func TestResizeRecoveryResetsViewportOnceAndResumesRendering(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frames := &frameWriter{writer: &output}
	resize := newResizeTracker(80)
	if err := drawInteractiveFrame(frames, resize, session, testStart, 80); err != nil {
		t.Fatal(err)
	}
	if err := drawInteractiveFrame(frames, resize, session, testStart.Add(time.Millisecond), 40); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), ansiClearDisplay); got != 0 {
		t.Errorf("resize suspension cleared viewport %d times, want 0", got)
	}
	if err := drawInteractiveFrame(frames, resize, session, testStart.Add(resizeDebounce+time.Millisecond), 40); err != nil {
		t.Fatal(err)
	}
	if err := drawInteractiveFrame(frames, resize, session, testStart.Add(resizeDebounce+2*time.Millisecond), 40); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), ansiClearDisplay); got != 1 {
		t.Errorf("viewport resets = %d, want 1", got)
	}
	if !strings.Contains(output.String(), ansiClearDisplay+ansiHome) {
		t.Errorf("recovery did not clear and home the visible display: %q", output.String())
	}
	if strings.Contains(output.String(), "\x1b[3J") || strings.Contains(output.String(), "\x1b[?1049") {
		t.Errorf("recovery used a scrollback or alternate-screen control: %q", output.String())
	}
	if !frames.initialized || !frames.anchorValid || frames.lineCount != 3 {
		t.Errorf("frame state after recovery = initialized %t, anchor valid %t, lines %d", frames.initialized, frames.anchorValid, frames.lineCount)
	}
	if !strings.Contains(output.String(), framedMiddle(session, testStart.Add(resizeDebounce+2*time.Millisecond), 40)) {
		t.Errorf("rendering did not resume after recovery: %q", output.String())
	}
}

func TestOrdinaryFrameTicksDoNotClearViewport(t *testing.T) {
	practice, _ := findPractice("coherence")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frames := &frameWriter{writer: &output}
	resize := newResizeTracker(80)
	if err := drawInteractiveFrame(frames, resize, session, testStart, 80); err != nil {
		t.Fatal(err)
	}
	if err := drawInteractiveFrame(frames, resize, session, testStart.Add(renderInterval), 80); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), ansiClearDisplay) {
		t.Errorf("ordinary frame ticks cleared the viewport: %q", output.String())
	}
}

func TestExitSummaryAfterResizeUsesViewportRecovery(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	frames := &frameWriter{writer: &output}
	if err := frames.write(renderCurrentFrame(session, testStart, 80)); err != nil {
		t.Fatal(err)
	}
	frames.invalidateAnchor()
	if err := finishInteractiveSession(frames, &output, session, testStart.Add(42*time.Second), false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), ansiClearDisplay+ansiHome) {
		t.Errorf("resize-aware exit did not reset the viewport: %q", output.String())
	}
	if !strings.HasSuffix(output.String(), "breathe · BOX BREATH · ended after 00:42") {
		t.Errorf("resize exit summary = %q", output.String())
	}
}
