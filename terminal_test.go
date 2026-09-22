package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLineOutputContainsNoTerminalControls(t *testing.T) {
	practice, _ := findPractice("coherence")
	var output bytes.Buffer
	lineOutput := LineOutput{Writer: &output}
	lineOutput.Started(practice)
	lineOutput.PhaseStarted(PhaseStart{Phase: practice.Phases[0]})
	lineOutput.Completed(practice, 1)

	for _, forbidden := range []string{"\x1b[", ansiEnterAlternate, ansiLeaveAlternate, ansiHideCursor, "\a"} {
		if strings.Contains(output.String(), forbidden) {
			t.Errorf("non-interactive output contains terminal control %q: %q", forbidden, output.String())
		}
	}
}

func TestAlternateLifecycleAndSummaryOrderingAreIdempotent(t *testing.T) {
	var output bytes.Buffer
	restoreCalls := 0
	lifecycle := &terminalLifecycle{
		writer: &output,
		restoreRaw: func() error {
			restoreCalls++
			output.WriteString("<raw-restored>")
			return nil
		},
	}

	if err := lifecycle.start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.finish("summary"); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.finish("summary"); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for control, want := range map[string]int{
		ansiEnterAlternate: 1,
		ansiLeaveAlternate: 1,
		ansiHideCursor:     1,
		ansiShowCursor:     1,
		ansiReset:          1,
		"summary\r\n":      1,
	} {
		if count := strings.Count(got, control); count != want {
			t.Errorf("%q count = %d, want %d; output = %q", control, count, want, got)
		}
	}
	if restoreCalls != 1 {
		t.Errorf("raw restore calls = %d, want 1", restoreCalls)
	}
	leave := strings.Index(got, ansiLeaveAlternate)
	raw := strings.Index(got, "<raw-restored>")
	summary := strings.Index(got, "summary")
	if !(leave >= 0 && leave < raw && raw < summary) {
		t.Errorf("cleanup/summary order is wrong: %q", got)
	}
	if strings.Contains(got, "\x1b[3J") {
		t.Errorf("lifecycle erased scrollback: %q", got)
	}
}

func TestLifecycleCleanupRunsDuringPanicUnwind(t *testing.T) {
	var output bytes.Buffer
	func() {
		defer func() { _ = recover() }()
		lifecycle := &terminalLifecycle{writer: &output}
		if err := lifecycle.start(); err != nil {
			t.Fatal(err)
		}
		defer lifecycle.finish("")
		panic("test panic")
	}()
	if strings.Count(output.String(), ansiLeaveAlternate) != 1 {
		t.Errorf("panic cleanup output = %q, want one alternate-screen leave", output.String())
	}
	if strings.Count(output.String(), ansiShowCursor) != 1 {
		t.Errorf("panic cleanup output = %q, want cursor restored", output.String())
	}
}

func TestLifecycleJoinsRestoreErrorAndStillLeavesAlternateScreen(t *testing.T) {
	var output bytes.Buffer
	wantErr := errors.New("restore failed")
	lifecycle := &terminalLifecycle{writer: &output, restoreRaw: func() error { return wantErr }}
	if err := lifecycle.start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.finish(""); !errors.Is(err, wantErr) {
		t.Fatalf("finish error = %v, want %v", err, wantErr)
	}
	if strings.Count(output.String(), ansiLeaveAlternate) != 1 {
		t.Errorf("error cleanup output = %q, want one alternate-screen leave", output.String())
	}
}

func TestQuitKeysIncludeQAndControlC(t *testing.T) {
	for _, key := range []byte{'q', 'Q', 0x03} {
		if !isQuitKey(key) {
			t.Errorf("isQuitKey(%#x) = false, want true", key)
		}
	}
	for _, key := range []byte{' ', 'x', '\r'} {
		if isQuitKey(key) {
			t.Errorf("isQuitKey(%#x) = true, want false", key)
		}
	}
}

func TestWordmarkRequiresCompleteBrandedCompositionSpace(t *testing.T) {
	lines := strings.Split(wordmark, "\n")
	if len(lines) != 3 {
		t.Fatalf("wordmark has %d lines, want 3", len(lines))
	}
	width := wordmarkWidth()
	if !wordmarkFits(width, brandedMinHeight) {
		t.Error("wordmark does not fit at its minimum composition dimensions")
	}
	if wordmarkFits(width-1, brandedMinHeight) {
		t.Error("wordmark fits in a terminal that is too narrow")
	}
	if wordmarkFits(width, brandedMinHeight-1) {
		t.Error("wordmark fits in a terminal that is too short")
	}
}

func TestSplashDurationIsAboutOneSecond(t *testing.T) {
	if splashTime < 900*time.Millisecond || splashTime > 1100*time.Millisecond {
		t.Errorf("splashTime = %s, want approximately one second", splashTime)
	}
}

func TestLogoOnlySplashPrecedesPersistentPacer(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	renderer := newAlternateRenderer(&output, 100, 30)
	if err := renderer.drawLogoOnly(); err != nil {
		t.Fatal(err)
	}
	logoOutput := output.String()
	if !strings.Contains(logoOutput, strings.Split(wordmark, "\n")[0]) {
		t.Errorf("splash did not contain wordmark: %q", logoOutput)
	}
	if strings.Contains(logoOutput, "BOX BREATH") {
		t.Errorf("logo-only splash already contains pacer: %q", logoOutput)
	}

	if err := renderer.drawPacer(renderCurrentFrame(session, testStart, renderer.layout.frameWidth)); err != nil {
		t.Fatal(err)
	}
	if err := renderer.drawPacer(renderCurrentFrame(session, testStart.Add(time.Second), renderer.layout.frameWidth)); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), strings.Split(wordmark, "\n")[0]); got != 1 {
		t.Errorf("ordinary pacer updates drew logo %d times, want 1", got)
	}
	if !strings.Contains(output.String(), "BOX BREATH") {
		t.Errorf("pacer was not rendered below splash: %q", output.String())
	}
}

func TestCompositionPositioningAndSmallTerminalFallback(t *testing.T) {
	large := calculateComposition(100, 30)
	if !large.showLogo || large.compositionH != 7 {
		t.Fatalf("large layout = %+v, want seven-row branded composition", large)
	}
	if large.logoRow < 1 || large.frameRow+2 > large.size.height {
		t.Errorf("large layout has invalid rows: %+v", large)
	}
	if large.logoColumn < 1 || large.frameColumn < 1 {
		t.Errorf("large layout has invalid columns: %+v", large)
	}
	compositionLeft := minInt(large.logoColumn, large.frameColumn)
	leftMargin := compositionLeft - 1
	rightMargin := large.size.width - (compositionLeft + large.compositionW - 1)
	if difference(leftMargin, rightMargin) > 1 {
		t.Errorf("composition is not centered: left %d, right %d", leftMargin, rightMargin)
	}
	if large.logoRow > large.size.height/3 {
		t.Errorf("composition begins too low for upper-third placement: %+v", large)
	}

	for _, dimensions := range []screenSize{{width: 40, height: 20}, {width: 80, height: 6}, {width: 8, height: 2}, {width: 1, height: 1}} {
		layout := calculateComposition(dimensions.width, dimensions.height)
		if layout.showLogo {
			t.Errorf("small layout %+v unexpectedly shows logo", layout)
		}
		if layout.frameRow < 1 || layout.frameRow > layout.size.height || layout.frameColumn < 1 {
			t.Errorf("small layout has invalid coordinates: %+v", layout)
		}
		practice, _ := findPractice("coherence")
		session, err := NewSession(practice, testStart)
		if err != nil {
			t.Fatal(err)
		}
		lines := renderCurrentFrame(session, testStart, layout.frameWidth)
		for _, line := range lines {
			if textWidth(line) > layout.size.width {
				t.Errorf("line %q exceeds terminal width %d", line, layout.size.width)
			}
		}
	}
}

func TestLogoHidesAndReappearsOnFullRedraw(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	renderer := newAlternateRenderer(&output, 100, 30)
	if err := renderer.redrawComposition(renderCurrentFrame(session, testStart, renderer.layout.frameWidth)); err != nil {
		t.Fatal(err)
	}
	logoLine := strings.Split(wordmark, "\n")[0]
	if !strings.Contains(output.String(), logoLine) {
		t.Fatal("initial full composition does not contain logo")
	}
	renderer.resize(40, 8)
	if err := renderer.redrawComposition(renderCurrentFrame(session, testStart, renderer.layout.frameWidth)); err != nil {
		t.Fatal(err)
	}
	lastClear := strings.LastIndex(output.String(), ansiClearDisplay)
	if strings.Contains(output.String()[lastClear:], logoLine) {
		t.Error("small resized composition still contains logo")
	}
	if !strings.Contains(output.String()[lastClear:], "BOX BREATH") {
		t.Error("small resized composition lost pacer")
	}
	renderer.resize(100, 30)
	if err := renderer.redrawComposition(renderCurrentFrame(session, testStart, renderer.layout.frameWidth)); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), logoLine); got != 2 {
		t.Errorf("logo appearances = %d, want 2 after resize restoration", got)
	}
}

func TestResizeTrackerDebouncesWidthAndHeightChanges(t *testing.T) {
	resize := newResizeTracker(80, 24)
	if got := resize.observe(80, 24, testStart); got != resizeRender {
		t.Errorf("unchanged dimensions action = %v, want render", got)
	}
	if got := resize.observe(70, 24, testStart); got != resizeSuspend {
		t.Errorf("width change action = %v, want suspend", got)
	}
	if got := resize.observe(70, 20, testStart.Add(100*time.Millisecond)); got != resizeSuspend {
		t.Errorf("height change action = %v, want suspend", got)
	}
	if got := resize.observe(70, 20, testStart.Add(299*time.Millisecond)); got != resizeSuspend {
		t.Errorf("unstable dimensions action = %v, want suspend", got)
	}
	if got := resize.observe(70, 20, testStart.Add(300*time.Millisecond)); got != resizeRecover {
		t.Errorf("stabilized dimensions action = %v, want recover", got)
	}
	if resize.current != (screenSize{width: 70, height: 20}) {
		t.Errorf("current size = %+v, want 70x20", resize.current)
	}
}

func TestResizeRecoveryClearsOnceAndOrdinaryTicksStayLocal(t *testing.T) {
	practice, _ := findPractice("box")
	session, err := NewSession(practice, testStart)
	if err != nil {
		t.Fatal(err)
	}
	startedAt := session.StartedAt
	deadline := session.PhaseDeadline
	var output bytes.Buffer
	renderer := newAlternateRenderer(&output, 80, 24)
	resize := newResizeTracker(80, 24)

	if err := drawInteractiveFrame(renderer, resize, session, testStart, 80, 24); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), ansiClearDisplay) {
		t.Errorf("ordinary tick cleared display: %q", output.String())
	}
	if err := drawInteractiveFrame(renderer, resize, session, testStart.Add(time.Millisecond), 60, 18); err != nil {
		t.Fatal(err)
	}
	if err := drawInteractiveFrame(renderer, resize, session, testStart.Add(resizeDebounce+time.Millisecond), 60, 18); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), ansiClearDisplay); got != 1 {
		t.Errorf("resize clears = %d, want 1; output = %q", got, output.String())
	}
	if !strings.Contains(output.String(), ansiClearDisplay+ansiHome) {
		t.Errorf("resize recovery did not clear and home: %q", output.String())
	}
	if !strings.Contains(output.String(), strings.Split(wordmark, "\n")[0]) || !strings.Contains(output.String(), "BOX BREATH") {
		t.Errorf("resize recovery did not redraw the complete composition: %q", output.String())
	}
	if strings.Contains(output.String(), "\x1b[3J") || strings.Contains(output.String(), "\x1b[2A") {
		t.Errorf("resize emitted scrollback erase or relative cursor-up: %q", output.String())
	}
	if session.StartedAt != startedAt || session.PhaseDeadline != deadline {
		t.Errorf("resize changed session timing: start %s, deadline %s", session.StartedAt, session.PhaseDeadline)
	}
	if strings.Contains(output.String(), "\a") {
		t.Errorf("resize emitted a bell: %q", output.String())
	}

	before := strings.Count(output.String(), ansiClearDisplay)
	if err := drawInteractiveFrame(renderer, resize, session, testStart.Add(resizeDebounce+2*time.Millisecond), 60, 18); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(output.String(), ansiClearDisplay); got != before {
		t.Errorf("ordinary post-resize tick changed clear count from %d to %d", before, got)
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

func difference(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
