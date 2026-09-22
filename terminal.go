package main

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const wordmark = `█▄▄▄▄ ▄▄▄▄▄▄▄▄▄▄▄ ▄▄▄▄▄▄▄█▄▄▄ █▄▄▄▄ ▄▄▄▄▄
█   █ █     █▄▄▄█ ▄▄▄▄█  █    █   █ █▄▄▄█
█▄▄▄█▄█     █▄▄▄▄▄█▄▄▄█  █▄▄▄▄█   █▄█▄▄▄▄`

const (
	ansiHideCursor   = "\x1b[?25l"
	ansiShowCursor   = "\x1b[?25h"
	ansiReset        = "\x1b[0m"
	ansiClearLine    = "\x1b[2K"
	ansiClearDisplay = "\x1b[2J"
	ansiHome         = "\x1b[H"
	splashTime       = time.Second
	renderInterval   = 50 * time.Millisecond
	resizeDebounce   = 200 * time.Millisecond
)

type terminalLifecycle struct {
	writer     io.Writer
	restoreRaw func() error
	restored   bool
}

func (l *terminalLifecycle) hideCursor() error {
	_, err := io.WriteString(l.writer, ansiHideCursor)
	return err
}

func (l *terminalLifecycle) restore() error {
	if l.restored {
		return nil
	}
	l.restored = true
	_, writeErr := io.WriteString(l.writer, ansiReset+ansiShowCursor+"\r\n")
	var rawErr error
	if l.restoreRaw != nil {
		rawErr = l.restoreRaw()
	}
	return errors.Join(writeErr, rawErr)
}

type frameWriter struct {
	writer      io.Writer
	initialized bool
	anchorValid bool
	lineCount   int
}

func (f *frameWriter) write(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	var frame strings.Builder
	if f.initialized && !f.anchorValid {
		if err := f.resetViewport(); err != nil {
			return err
		}
	}
	if f.initialized {
		frame.WriteByte('\r')
		if f.lineCount > 1 {
			frame.WriteString("\x1b[")
			frame.WriteString(strconv.Itoa(f.lineCount - 1))
			frame.WriteByte('A')
		}
	}
	for i, line := range lines {
		if f.initialized {
			frame.WriteString(ansiClearLine)
		}
		frame.WriteString(line)
		if i < len(lines)-1 {
			frame.WriteString("\r\n")
		}
	}

	_, err := io.WriteString(f.writer, frame.String())
	if err == nil {
		f.initialized = true
		f.anchorValid = true
		f.lineCount = len(lines)
	}
	return err
}

func (f *frameWriter) clear() error {
	if !f.initialized {
		return nil
	}
	if !f.anchorValid {
		return f.resetViewport()
	}

	var frame strings.Builder
	frame.WriteByte('\r')
	if f.lineCount > 1 {
		frame.WriteString("\x1b[")
		frame.WriteString(strconv.Itoa(f.lineCount - 1))
		frame.WriteByte('A')
	}
	for i := 0; i < f.lineCount; i++ {
		frame.WriteString(ansiClearLine)
		if i < f.lineCount-1 {
			frame.WriteString("\r\n")
		}
	}

	_, err := io.WriteString(f.writer, frame.String())
	if err == nil {
		f.initialized = false
		f.anchorValid = true
		f.lineCount = 0
	}
	return err
}

func (f *frameWriter) invalidateAnchor() {
	if f.initialized {
		f.anchorValid = false
	}
}

func (f *frameWriter) resetViewport() error {
	_, err := io.WriteString(f.writer, ansiClearDisplay+ansiHome)
	if err == nil {
		f.initialized = false
		f.anchorValid = true
		f.lineCount = 0
	}
	return err
}

type resizeAction int

const (
	resizeRender resizeAction = iota
	resizeSuspend
	resizeRecover
)

type resizeTracker struct {
	width        int
	pendingWidth int
	pendingSince time.Time
	pending      bool
}

func newResizeTracker(width int) *resizeTracker {
	return &resizeTracker{width: effectiveFrameWidth(width)}
}

func (r *resizeTracker) observe(width int, now time.Time) resizeAction {
	effectiveWidth := effectiveFrameWidth(width)
	if !r.pending && effectiveWidth == r.width {
		return resizeRender
	}
	if !r.pending || effectiveWidth != r.pendingWidth {
		r.pending = true
		r.pendingWidth = effectiveWidth
		r.pendingSince = now
		return resizeSuspend
	}
	if now.Sub(r.pendingSince) < resizeDebounce {
		return resizeSuspend
	}
	r.width = r.pendingWidth
	r.pending = false
	return resizeRecover
}

func drawInteractiveFrame(frames *frameWriter, resize *resizeTracker, session *Session, now time.Time, terminalWidth int) error {
	switch resize.observe(terminalWidth, now) {
	case resizeSuspend:
		frames.invalidateAnchor()
		return nil
	case resizeRecover:
		if err := frames.resetViewport(); err != nil {
			return err
		}
	}
	return frames.write(renderCurrentFrame(session, now, resize.width))
}

func prepareInteractiveExit(frames *frameWriter, resize *resizeTracker, now time.Time, terminalWidth int) error {
	switch resize.observe(terminalWidth, now) {
	case resizeSuspend:
		frames.invalidateAnchor()
	case resizeRecover:
		return frames.resetViewport()
	}
	return nil
}

type keyEvent struct {
	key byte
	err error
}

func interactiveTerminal(stdin, stdout *os.File) bool {
	return term.IsTerminal(int(stdin.Fd())) && term.IsTerminal(int(stdout.Fd()))
}

func runInteractiveSession(practice Practice, stdin, stdout *os.File, sound bool) (handled bool, runErr error) {
	state, err := term.MakeRaw(int(stdin.Fd()))
	if err != nil {
		return false, nil
	}

	lifecycle := &terminalLifecycle{
		writer: stdout,
		restoreRaw: func() error {
			return term.Restore(int(stdin.Fd()), state)
		},
	}
	defer func() {
		runErr = errors.Join(runErr, lifecycle.restore())
	}()
	if err := lifecycle.hideCursor(); err != nil {
		return true, err
	}

	keys := readKeys(stdin)
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	frames := &frameWriter{writer: stdout}
	width, height := terminalSize(stdout)
	splashWidth := width
	showedSplash := false
	if wordmarkFits(width, height) {
		showedSplash = true
		if err := frames.write(strings.Split(wordmark, "\n")); err != nil {
			return true, err
		}
		timer := time.NewTimer(splashTime)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				goto splashComplete
			case event := <-keys:
				if event.err != nil {
					return true, event.err
				}
				if isQuitKey(event.key) {
					return true, finishSplashExit(frames, stdout, practice)
				}
			case <-interrupts:
				return true, finishSplashExit(frames, stdout, practice)
			}
		}
	}

splashComplete:
	session, err := NewSession(practice, time.Now())
	if err != nil {
		return true, err
	}
	width, _ = terminalSize(stdout)
	if showedSplash && effectiveFrameWidth(width) != effectiveFrameWidth(splashWidth) {
		if err := frames.resetViewport(); err != nil {
			return true, err
		}
	}
	resize := newResizeTracker(width)
	if err := drawInteractiveFrame(frames, resize, session, session.StartedAt, width); err != nil {
		return true, err
	}

	ticker := time.NewTicker(renderInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			width, _ = terminalSize(stdout)
			starts := session.Advance(now)
			if err := emitPhaseBell(stdout, sound, starts); err != nil {
				return true, err
			}
			if err := drawInteractiveFrame(frames, resize, session, now, width); err != nil {
				return true, err
			}
			if session.State == SessionCompleted {
				return true, finishInteractiveSession(frames, stdout, session, now, true)
			}
		case event := <-keys:
			if event.err != nil {
				return true, event.err
			}
			if isQuitKey(event.key) {
				now := time.Now()
				width, _ := terminalSize(stdout)
				if err := prepareInteractiveExit(frames, resize, now, width); err != nil {
					return true, err
				}
				return true, finishInteractiveSession(frames, stdout, session, now, false)
			}
			if event.key == ' ' {
				now := time.Now()
				if session.State == SessionRunning {
					session.Pause(now)
				} else if session.State == SessionPaused {
					session.Resume(now)
				}
				width, _ = terminalSize(stdout)
				if err := drawInteractiveFrame(frames, resize, session, now, width); err != nil {
					return true, err
				}
				if session.State == SessionCompleted {
					return true, finishInteractiveSession(frames, stdout, session, now, true)
				}
			}
		case <-interrupts:
			now := time.Now()
			width, _ := terminalSize(stdout)
			if err := prepareInteractiveExit(frames, resize, now, width); err != nil {
				return true, err
			}
			return true, finishInteractiveSession(frames, stdout, session, now, false)
		}
	}
}

func emitPhaseBell(writer io.Writer, enabled bool, starts []PhaseStart) error {
	if !enabled || len(starts) == 0 {
		return nil
	}
	_, err := io.WriteString(writer, "\a")
	return err
}

func finishInteractiveSession(frames *frameWriter, writer io.Writer, session *Session, now time.Time, completed bool) error {
	if err := frames.clear(); err != nil {
		return err
	}
	_, err := io.WriteString(writer, sessionSummary(session, now, completed))
	return err
}

func finishSplashExit(frames *frameWriter, writer io.Writer, practice Practice) error {
	if err := frames.clear(); err != nil {
		return err
	}
	_, err := io.WriteString(writer, "breathe · "+strings.ToUpper(practice.Name)+" · ended after 00:00")
	return err
}

func readKeys(reader io.Reader) <-chan keyEvent {
	events := make(chan keyEvent, 16)
	go func() {
		var buffer [1]byte
		for {
			n, err := reader.Read(buffer[:])
			if n > 0 {
				events <- keyEvent{key: buffer[0]}
			}
			if err != nil {
				events <- keyEvent{err: err}
				return
			}
		}
	}()
	return events
}

func isQuitKey(key byte) bool {
	return key == 'q' || key == 'Q' || key == 0x03
}

func terminalSize(output *os.File) (int, int) {
	width, height, err := term.GetSize(int(output.Fd()))
	if err != nil || width <= 0 {
		return 80, 24
	}
	return width, height
}

func wordmarkFits(width, height int) bool {
	if height < 3 {
		return false
	}
	for _, line := range strings.Split(wordmark, "\n") {
		if textWidth(line) > width {
			return false
		}
	}
	return true
}
