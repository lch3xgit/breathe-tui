package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/term"
)

const wordmark = `█▄▄▄▄ ▄▄▄▄▄▄▄▄▄▄▄ ▄▄▄▄▄▄▄█▄▄▄ █▄▄▄▄ ▄▄▄▄▄
█   █ █     █▄▄▄█ ▄▄▄▄█  █    █   █ █▄▄▄█
█▄▄▄█▄█     █▄▄▄▄▄█▄▄▄█  █▄▄▄▄█   █▄█▄▄▄▄`

const (
	ansiEnterAlternate = "\x1b[?1049h"
	ansiLeaveAlternate = "\x1b[?1049l"
	ansiHideCursor     = "\x1b[?25l"
	ansiShowCursor     = "\x1b[?25h"
	ansiReset          = "\x1b[0m"
	ansiClearLine      = "\x1b[2K"
	ansiClearDisplay   = "\x1b[2J"
	ansiHome           = "\x1b[H"
	splashTime         = time.Second
	renderInterval     = 50 * time.Millisecond
	resizeDebounce     = 200 * time.Millisecond
	brandedMinHeight   = 9
)

type terminalLifecycle struct {
	writer       io.Writer
	restoreRaw   func() error
	started      bool
	alternate    bool
	restored     bool
	summaryWrote bool
}

func (l *terminalLifecycle) start() error {
	if l.started {
		return nil
	}
	l.started = true
	l.alternate = true
	_, err := io.WriteString(l.writer, ansiEnterAlternate+ansiClearDisplay+ansiHome+ansiHideCursor)
	return err
}

func (l *terminalLifecycle) restore() error {
	if l.restored {
		return nil
	}
	l.restored = true

	var writeErr error
	if l.started {
		sequence := ansiReset + ansiShowCursor
		if l.alternate {
			sequence += ansiLeaveAlternate
			l.alternate = false
		}
		_, writeErr = io.WriteString(l.writer, sequence)
	}
	var rawErr error
	if l.restoreRaw != nil {
		rawErr = l.restoreRaw()
	}
	return errors.Join(writeErr, rawErr)
}

func (l *terminalLifecycle) finish(summary string) error {
	restoreErr := l.restore()
	if restoreErr != nil || summary == "" || l.summaryWrote {
		return restoreErr
	}
	l.summaryWrote = true
	_, summaryErr := io.WriteString(l.writer, summary+"\r\n")
	return errors.Join(restoreErr, summaryErr)
}

type screenSize struct {
	width  int
	height int
}

func normalizedScreenSize(width, height int) screenSize {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	return screenSize{width: width, height: height}
}

type compositionLayout struct {
	size         screenSize
	frameWidth   int
	frameRow     int
	frameColumn  int
	frameRows    int
	showLogo     bool
	logoRow      int
	logoColumn   int
	compositionW int
	compositionH int
}

func calculateComposition(width, height int) compositionLayout {
	size := normalizedScreenSize(width, height)
	frameWidth := effectiveFrameWidth(size.width)
	frameRows := minInt(3, size.height)
	logoWidth := wordmarkWidth()
	showLogo := wordmarkFits(size.width, size.height)

	compositionWidth := frameWidth
	compositionHeight := frameRows
	if showLogo {
		compositionWidth = maxInt(compositionWidth, logoWidth)
		compositionHeight = 7
	}

	top := upperThirdTop(size.height, compositionHeight)
	left := centeredStart(size.width, compositionWidth)
	layout := compositionLayout{
		size:         size,
		frameWidth:   frameWidth,
		frameRows:    frameRows,
		showLogo:     showLogo,
		compositionW: compositionWidth,
		compositionH: compositionHeight,
		frameRow:     top,
		frameColumn:  left + (compositionWidth-frameWidth)/2,
	}
	if showLogo {
		layout.logoRow = top
		layout.logoColumn = left + (compositionWidth-logoWidth)/2
		layout.frameRow = top + 4
	}
	return layout
}

func upperThirdTop(height, compositionHeight int) int {
	if compositionHeight > height {
		compositionHeight = height
	}
	center := maxInt(1, height/3)
	top := center - (compositionHeight-1)/2
	if top < 1 {
		top = 1
	}
	lastTop := height - compositionHeight + 1
	if top > lastTop {
		top = lastTop
	}
	return top
}

func centeredStart(available, content int) int {
	if content >= available {
		return 1
	}
	return (available-content)/2 + 1
}

func cursorPosition(row, column int) string {
	return fmt.Sprintf("\x1b[%d;%dH", maxInt(1, row), maxInt(1, column))
}

type alternateRenderer struct {
	writer io.Writer
	layout compositionLayout
}

func newAlternateRenderer(writer io.Writer, width, height int) *alternateRenderer {
	return &alternateRenderer{writer: writer, layout: calculateComposition(width, height)}
}

func (r *alternateRenderer) resize(width, height int) {
	r.layout = calculateComposition(width, height)
}

func (r *alternateRenderer) drawLogoOnly() error {
	if !r.layout.showLogo {
		return nil
	}
	_, err := io.WriteString(r.writer, renderPositionedLines(strings.Split(wordmark, "\n"), r.layout.logoRow, r.layout.logoColumn, r.layout.size, false))
	return err
}

func (r *alternateRenderer) drawPacer(lines []string) error {
	_, err := io.WriteString(r.writer, renderPositionedLines(lines, r.layout.frameRow, r.layout.frameColumn, r.layout.size, true))
	return err
}

func (r *alternateRenderer) redrawComposition(lines []string) error {
	var output strings.Builder
	output.WriteString(ansiClearDisplay)
	output.WriteString(ansiHome)
	if r.layout.showLogo {
		output.WriteString(renderPositionedLines(strings.Split(wordmark, "\n"), r.layout.logoRow, r.layout.logoColumn, r.layout.size, false))
	}
	output.WriteString(renderPositionedLines(lines, r.layout.frameRow, r.layout.frameColumn, r.layout.size, true))
	_, err := io.WriteString(r.writer, output.String())
	return err
}

func renderPositionedLines(lines []string, firstRow, column int, size screenSize, clearRows bool) string {
	var output strings.Builder
	for index, line := range lines {
		row := firstRow + index
		if row < 1 || row > size.height {
			continue
		}
		line = truncateText(line, size.width-column+1)
		if clearRows {
			output.WriteString(cursorPosition(row, 1))
			output.WriteString(ansiClearLine)
		}
		output.WriteString(cursorPosition(row, column))
		output.WriteString(line)
	}
	return output.String()
}

type resizeAction int

const (
	resizeRender resizeAction = iota
	resizeSuspend
	resizeRecover
)

type resizeTracker struct {
	current      screenSize
	pending      screenSize
	pendingSince time.Time
	resizing     bool
}

func newResizeTracker(width, height int) *resizeTracker {
	return &resizeTracker{current: normalizedScreenSize(width, height)}
}

func (r *resizeTracker) observe(width, height int, now time.Time) resizeAction {
	size := normalizedScreenSize(width, height)
	if !r.resizing && size == r.current {
		return resizeRender
	}
	if !r.resizing || size != r.pending {
		r.resizing = true
		r.pending = size
		r.pendingSince = now
		return resizeSuspend
	}
	if now.Sub(r.pendingSince) < resizeDebounce {
		return resizeSuspend
	}
	r.current = r.pending
	r.resizing = false
	return resizeRecover
}

func drawInteractiveFrame(renderer *alternateRenderer, resize *resizeTracker, session *Session, now time.Time, width, height int) error {
	switch resize.observe(width, height, now) {
	case resizeSuspend:
		return nil
	case resizeRecover:
		renderer.resize(resize.current.width, resize.current.height)
		return renderer.redrawComposition(renderCurrentFrame(session, now, renderer.layout.frameWidth))
	default:
		return renderer.drawPacer(renderCurrentFrame(session, now, renderer.layout.frameWidth))
	}
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
	summary := ""
	defer func() {
		runErr = errors.Join(runErr, lifecycle.finish(summary))
	}()
	if err := lifecycle.start(); err != nil {
		return true, err
	}

	keys := readKeys(stdin)
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	width, height := terminalSize(stdout)
	renderer := newAlternateRenderer(stdout, width, height)
	if renderer.layout.showLogo {
		if err := renderer.drawLogoOnly(); err != nil {
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
					summary = splashExitSummary(practice)
					return true, nil
				}
			case <-interrupts:
				summary = splashExitSummary(practice)
				return true, nil
			}
		}
	}

splashComplete:
	session, err := NewSession(practice, time.Now())
	if err != nil {
		return true, err
	}
	width, height = terminalSize(stdout)
	currentSize := normalizedScreenSize(width, height)
	if currentSize == renderer.layout.size {
		if err := renderer.drawPacer(renderCurrentFrame(session, session.StartedAt, renderer.layout.frameWidth)); err != nil {
			return true, err
		}
	} else {
		renderer.resize(width, height)
		if err := renderer.redrawComposition(renderCurrentFrame(session, session.StartedAt, renderer.layout.frameWidth)); err != nil {
			return true, err
		}
	}
	resize := newResizeTracker(width, height)

	ticker := time.NewTicker(renderInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			width, height = terminalSize(stdout)
			starts := session.Advance(now)
			if err := emitPhaseBell(stdout, sound, starts); err != nil {
				return true, err
			}
			if err := drawInteractiveFrame(renderer, resize, session, now, width, height); err != nil {
				return true, err
			}
			if session.State == SessionCompleted {
				summary = sessionSummary(session, now, true)
				return true, nil
			}
		case event := <-keys:
			if event.err != nil {
				return true, event.err
			}
			if isQuitKey(event.key) {
				summary = sessionSummary(session, time.Now(), false)
				return true, nil
			}
			if event.key == ' ' {
				now := time.Now()
				if session.State == SessionRunning {
					session.Pause(now)
				} else if session.State == SessionPaused {
					session.Resume(now)
				}
				width, height = terminalSize(stdout)
				if err := drawInteractiveFrame(renderer, resize, session, now, width, height); err != nil {
					return true, err
				}
				if session.State == SessionCompleted {
					summary = sessionSummary(session, now, true)
					return true, nil
				}
			}
		case <-interrupts:
			summary = sessionSummary(session, time.Now(), false)
			return true, nil
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

func splashExitSummary(practice Practice) string {
	return "breathe · " + strings.ToUpper(practice.Name) + " · ended after 00:00"
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
	if err != nil || width <= 0 || height <= 0 {
		return 80, 24
	}
	return width, height
}

func wordmarkFits(width, height int) bool {
	return width >= wordmarkWidth() && height >= brandedMinHeight
}

func wordmarkWidth() int {
	width := 0
	for _, line := range strings.Split(wordmark, "\n") {
		width = maxInt(width, textWidth(line))
	}
	return width
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
